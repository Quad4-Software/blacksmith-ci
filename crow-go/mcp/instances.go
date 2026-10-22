// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package mcp implements a Model Context Protocol server that lets coding
// agents find, read and wait on Crow pipelines across several instances.
package mcp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-yaml"

	"codefloe.com/crowci/crow/v6/crow-go/crow"
)

const (
	tokenCommandTimeout = 30 * time.Second
	// HttpTimeout bounds every API request; crow-go requests carry no context.
	httpTimeout           = 60 * time.Second
	responseHeaderTimeout = 30 * time.Second
)

// Instance is one Crow server the MCP server may talk to.
type Instance struct {
	Name         string   `yaml:"name"`
	URL          string   `yaml:"url"`
	ForgeHosts   []string `yaml:"forge_hosts"`
	TokenEnv     string   `yaml:"token_env"`
	TokenCommand []string `yaml:"token_command"`

	// token is a fixed token, used when no config file exists.
	token string
}

// NewStaticInstance returns an instance named after the server host that
// authenticates with a fixed token.
func NewStaticInstance(serverURL, token string) Instance {
	inst := Instance{URL: serverURL, token: token}
	inst.Name = inst.Host()
	return inst
}

// Host returns the host name of the instance URL.
func (i Instance) Host() string {
	u, err := url.Parse(i.URL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// Config lists the configured instances.
type Config struct {
	Instances []Instance `yaml:"instances"`
}

// LoadConfig reads the config file at path. It returns (nil, nil) if the file
// does not exist.
func LoadConfig(path string) (*Config, error) {
	buf, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read mcp config: %w", err)
	}

	cfg := new(Config)
	if err := yaml.Unmarshal(buf, cfg); err != nil {
		return nil, fmt.Errorf("parse mcp config %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid mcp config %s: %w", path, err)
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if len(c.Instances) == 0 {
		return errors.New("no instances configured")
	}
	seen := map[string]bool{}
	for i, inst := range c.Instances {
		if inst.Name == "" {
			return fmt.Errorf("instance %d has no name", i)
		}
		if seen[inst.Name] {
			return fmt.Errorf("instance %q is configured twice", inst.Name)
		}
		seen[inst.Name] = true

		u, err := url.Parse(inst.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("instance %q: url must start with http:// or https://", inst.Name)
		}
		sources := 0
		for _, set := range []bool{inst.TokenEnv != "", len(inst.TokenCommand) > 0, inst.token != ""} {
			if set {
				sources++
			}
		}
		if sources != 1 {
			return fmt.Errorf("instance %q: set exactly one of token_env or token_command", inst.Name)
		}
	}
	return nil
}

// TokenFunc returns the current token for an instance; refresh=true asks for
// a fresh one.
type TokenFunc func(ctx context.Context, refresh bool) (string, error)

// ClientFactory builds a Crow API client for a server URL.
type ClientFactory func(serverURL string, token TokenFunc) crow.Client

// Registry resolves instances and hands out memoized clients.
type Registry struct {
	instances []Instance
	factory   ClientFactory

	mu      sync.Mutex
	clients map[string]crow.Client
}

// NewRegistry validates cfg and returns a registry.
func NewRegistry(cfg *Config, factory ClientFactory) (*Registry, error) {
	if cfg == nil {
		return nil, errors.New("no mcp config")
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &Registry{
		instances: cfg.Instances,
		factory:   factory,
		clients:   map[string]crow.Client{},
	}, nil
}

// Instances returns the configured instances.
func (r *Registry) Instances() []Instance {
	return r.instances
}

// ByName returns the instance with the given name.
func (r *Registry) ByName(name string) (Instance, bool) {
	for _, inst := range r.instances {
		if inst.Name == name {
			return inst, true
		}
	}
	return Instance{}, false
}

// ByHost returns the instance whose URL has the given host.
func (r *Registry) ByHost(host string) (Instance, bool) {
	for _, inst := range r.instances {
		if strings.EqualFold(inst.Host(), host) {
			return inst, true
		}
	}
	return Instance{}, false
}

// ByForgeHost returns the instance that serves repositories on the given forge host.
func (r *Registry) ByForgeHost(host string) (Instance, bool) {
	for _, inst := range r.instances {
		for _, fh := range inst.ForgeHosts {
			if strings.EqualFold(fh, host) {
				return inst, true
			}
		}
	}
	// A single instance without forge hosts serves every remote, like the
	// CLI does with its one configured server.
	if len(r.instances) == 1 && len(r.instances[0].ForgeHosts) == 0 {
		return r.instances[0], true
	}
	return Instance{}, false
}

// ForgeHosts returns every configured forge host.
func (r *Registry) ForgeHosts() []string {
	var hosts []string
	for _, inst := range r.instances {
		hosts = append(hosts, inst.ForgeHosts...)
	}
	return hosts
}

// Client returns the API client for the named instance.
func (r *Registry) Client(name string) (crow.Client, error) {
	inst, ok := r.ByName(name)
	if !ok {
		return nil, fmt.Errorf("unknown instance %q", name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[name]; ok {
		return c, nil
	}
	c := r.factory(strings.TrimRight(inst.URL, "/"), newTokenSource(inst))
	r.clients[name] = c
	return c, nil
}

func newTokenSource(inst Instance) TokenFunc {
	if inst.token != "" {
		return func(context.Context, bool) (string, error) { return inst.token, nil }
	}
	if inst.TokenEnv != "" {
		return func(context.Context, bool) (string, error) {
			v := strings.TrimSpace(os.Getenv(inst.TokenEnv))
			if v == "" {
				return "", fmt.Errorf("instance %q: environment variable %s is empty", inst.Name, inst.TokenEnv)
			}
			return v, nil
		}
	}

	var (
		mu     sync.Mutex
		cached string
	)
	return func(ctx context.Context, refresh bool) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if cached != "" && !refresh {
			return cached, nil
		}

		ctx, cancel := context.WithTimeout(ctx, tokenCommandTimeout)
		defer cancel()
		// #nosec G204 -- the command comes from the user's own config file.
		cmd := exec.CommandContext(ctx, inst.TokenCommand[0], inst.TokenCommand[1:]...)
		out, err := cmd.Output()
		if err != nil {
			// Deliberately omit stdout/stderr: they may contain token material.
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return "", fmt.Errorf("instance %q: token_command exited with code %d", inst.Name, exitErr.ExitCode())
			}
			return "", fmt.Errorf("instance %q: token_command failed to run", inst.Name)
		}
		v := strings.TrimSpace(string(out))
		if v == "" {
			return "", fmt.Errorf("instance %q: token_command printed no token", inst.Name)
		}
		cached = v
		return v, nil
	}
}

// DefaultHTTPClientFactory builds real API clients that authenticate with a
// bearer token and refresh it once on a 401 response.
func DefaultHTTPClientFactory(skipVerify bool) ClientFactory {
	return func(serverURL string, token TokenFunc) crow.Client {
		return crow.NewClient(serverURL, newHTTPClient(skipVerify, serverURL, token))
	}
}

func newHTTPClient(skipVerify bool, serverURL string, token TokenFunc) *http.Client {
	origin, _ := url.Parse(serverURL)
	certs, err := x509.SystemCertPool()
	if err != nil {
		certs = nil
	}
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}
	base = base.Clone()
	base.ResponseHeaderTimeout = responseHeaderTimeout
	base.TLSClientConfig = &tls.Config{
		RootCAs: certs,
		// #nosec G402 -- opt-in, mirrors the CLI's --skip-verify flag.
		InsecureSkipVerify: skipVerify,
	}
	return &http.Client{
		Timeout: httpTimeout,
		Transport: &tokenTransport{
			origin: origin,
			token:  token,
			base:   base,
		},
	}
}

type tokenTransport struct {
	origin *url.URL
	token  TokenFunc
	base   http.RoundTripper
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Never hand the token to another origin, e.g. after a redirect.
	if t.origin == nil || !strings.EqualFold(req.URL.Scheme, t.origin.Scheme) || !strings.EqualFold(req.URL.Host, t.origin.Host) {
		return t.base.RoundTrip(req)
	}

	resp, err := t.do(req, false)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}
	if req.Body != nil && req.GetBody == nil {
		return resp, nil
	}
	resp.Body.Close()

	retry := req.Clone(req.Context())
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		retry.Body = body
	}
	return t.do(retry, true)
}

func (t *tokenTransport) do(req *http.Request, refresh bool) (*http.Response, error) {
	tok, err := t.token(req.Context(), refresh)
	if err != nil {
		return nil, err
	}
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+tok)
	return t.base.RoundTrip(r)
}

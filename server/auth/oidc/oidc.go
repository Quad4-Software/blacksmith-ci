// Copyright 2026 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package oidc implements sign-in through a generic OpenID Connect provider:
// discovery, the authorization code flow with PKCE, ID token verification and
// group-based access checks. It does not depend on the server package.
package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// ProviderName is stored as user_identities.provider for OIDC identities.
const ProviderName = "oidc"

const (
	defaultDisplayName   = "Single sign-on"
	defaultUsernameClaim = "preferred_username"
	defaultGroupsClaim   = "groups"
	httpTimeout          = 10 * time.Second
)

// reLogin mirrors the Crow login format enforced by model.User.Validate.
var reLogin = regexp.MustCompile(`^[a-zA-Z0-9-_.]+$`)

// Config configures the OIDC provider.
type Config struct {
	Issuer        string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        []string
	DisplayName   string
	UsernameClaim string
	GroupsClaim   string
	AllowedGroups []string
	AdminGroups   []string
	// Only disables forge OAuth sign-in; forge OAuth linking keeps working.
	Only bool
}

// Enabled reports whether OIDC sign-in is configured.
func (c Config) Enabled() bool { return c.Issuer != "" }

// maxIssuerLen is the size of the issuer column, which is part of every stored
// identity key.
const maxIssuerLen = 255

// Validate reports configuration errors that must stop the server from starting.
func (c Config) Validate() error {
	if c.Only && c.Issuer == "" {
		return errors.New("CROW_OIDC_ONLY requires CROW_OIDC_ISSUER")
	}
	if c.Issuer != "" && (c.ClientID == "" || c.ClientSecret == "") {
		return errors.New("CROW_OIDC_ISSUER requires CROW_OIDC_CLIENT_ID and CROW_OIDC_CLIENT_SECRET")
	}
	if len(c.Issuer) > maxIssuerLen {
		return errors.New("CROW_OIDC_ISSUER must not be longer than 255 characters")
	}
	return nil
}

// Claims are the verified user attributes from a sign-in.
type Claims struct {
	Subject  string
	Email    string
	Username string
	Groups   []string
}

// Provider signs users in through an OpenID Connect issuer. Discovery happens
// lazily and is retried until it succeeds, so an unreachable issuer never blocks
// server start.
type Provider struct {
	cfg        Config
	httpClient *http.Client

	mu       sync.Mutex
	provider *gooidc.Provider
}

// New returns a provider for cfg without contacting the issuer.
func New(cfg Config) *Provider {
	if !slices.Contains(cfg.Scopes, gooidc.ScopeOpenID) {
		cfg.Scopes = append([]string{gooidc.ScopeOpenID}, cfg.Scopes...)
	}
	if cfg.DisplayName == "" {
		cfg.DisplayName = defaultDisplayName
	}
	if cfg.UsernameClaim == "" {
		cfg.UsernameClaim = defaultUsernameClaim
	}
	if cfg.GroupsClaim == "" {
		cfg.GroupsClaim = defaultGroupsClaim
	}
	return &Provider{cfg: cfg, httpClient: &http.Client{Timeout: httpTimeout}}
}

// Init runs discovery eagerly so configuration problems show up in the startup log.
func (p *Provider) Init(ctx context.Context) error {
	_, err := p.discover(ctx)
	return err
}

func (p *Provider) clientContext(ctx context.Context) context.Context {
	return gooidc.ClientContext(ctx, p.httpClient)
}

func (p *Provider) discover(ctx context.Context) (*gooidc.Provider, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.provider != nil {
		return p.provider, nil
	}
	provider, err := gooidc.NewProvider(p.clientContext(ctx), p.cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC issuer %s: %w", p.cfg.Issuer, err)
	}
	p.provider = provider
	return provider, nil
}

func (p *Provider) oauth2Config(provider *gooidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.cfg.ClientID,
		ClientSecret: p.cfg.ClientSecret,
		RedirectURL:  p.cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       p.cfg.Scopes,
	}
}

// AuthCodeURL returns the issuer URL to send the user to.
func (p *Provider) AuthCodeURL(ctx context.Context, state, nonce, verifier string) (string, error) {
	provider, err := p.discover(ctx)
	if err != nil {
		return "", err
	}
	return p.oauth2Config(provider).AuthCodeURL(state, gooidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), nil
}

// Exchange redeems an authorization code and returns the verified claims.
func (p *Provider) Exchange(ctx context.Context, code, verifier, nonce string) (*Claims, error) {
	provider, err := p.discover(ctx)
	if err != nil {
		return nil, err
	}
	ctx = p.clientContext(ctx)

	tok, err := p.oauth2Config(provider).Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}
	rawIDToken, ok := tok.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New("token response has no id_token")
	}
	idToken, err := provider.Verifier(&gooidc.Config{ClientID: p.cfg.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify ID token: %w", err)
	}
	if idToken.Nonce != nonce {
		return nil, errors.New("ID token nonce does not match")
	}
	if idToken.Subject == "" {
		return nil, errors.New("ID token has no subject")
	}

	var raw map[string]any
	if err := idToken.Claims(&raw); err != nil {
		return nil, fmt.Errorf("decode ID token claims: %w", err)
	}
	claims := &Claims{
		Subject:  idToken.Subject,
		Email:    stringClaim(raw, "email"),
		Username: stringClaim(raw, p.cfg.UsernameClaim),
	}

	groups, found := groupsClaim(raw, p.cfg.GroupsClaim)
	if !found && (p.RestrictsGroups() || p.ManagesAdmin()) {
		// Many providers only put groups into the userinfo response. Groups only
		// matter for the allowed and admin group checks, so skip the extra request
		// when neither is configured.
		info, err := provider.UserInfo(ctx, oauth2.StaticTokenSource(tok))
		if err != nil {
			return nil, fmt.Errorf("fetch userinfo: %w", err)
		}
		if info.Subject != idToken.Subject {
			return nil, errors.New("userinfo subject does not match ID token subject")
		}
		var infoRaw map[string]any
		if err := info.Claims(&infoRaw); err != nil {
			return nil, fmt.Errorf("decode userinfo claims: %w", err)
		}
		groups, _ = groupsClaim(infoRaw, p.cfg.GroupsClaim)
	}
	claims.Groups = groups
	return claims, nil
}

// IsAllowed reports whether a user with these groups may sign in.
func (p *Provider) IsAllowed(groups []string) bool {
	return !p.RestrictsGroups() || intersects(p.cfg.AllowedGroups, groups)
}

// RestrictsGroups reports whether sign-in is limited to CROW_OIDC_ALLOWED_GROUPS.
func (p *Provider) RestrictsGroups() bool { return len(p.cfg.AllowedGroups) > 0 }

// ManagesAdmin reports whether admin status is derived from CROW_OIDC_ADMIN_GROUPS.
func (p *Provider) ManagesAdmin() bool { return len(p.cfg.AdminGroups) > 0 }

// IsAdmin reports whether these groups grant instance admin.
func (p *Provider) IsAdmin(groups []string) bool { return intersects(p.cfg.AdminGroups, groups) }

// Issuer is the configured issuer URL. Identities are keyed by issuer and
// subject, because a subject is only unique within its issuer.
func (p *Provider) Issuer() string { return p.cfg.Issuer }

// DisplayName is the label of the login button.
func (p *Provider) DisplayName() string { return p.cfg.DisplayName }

// ForgeLoginDisabled reports whether forge OAuth sign-in is switched off.
func (p *Provider) ForgeLoginDisabled() bool { return p.cfg.Only }

// RandomString returns a URL-safe random value for state, nonce and PKCE verifiers.
func RandomString() string { return oauth2.GenerateVerifier() }

// NormalizeUsername turns a username claim into a valid Crow login. UPN-style
// values (jane@corp.example, as sent by Microsoft Entra ID) keep their local part.
func NormalizeUsername(raw string) (string, bool) {
	login, _, _ := strings.Cut(raw, "@")
	if login == "" || len(login) > 250 || !reLogin.MatchString(login) {
		return "", false
	}
	return login, true
}

func stringClaim(raw map[string]any, name string) string {
	s, _ := raw[name].(string)
	return s
}

// groupsClaim reads a claim that is either a string or a list of strings.
// Found is false only when the claim is absent.
func groupsClaim(raw map[string]any, name string) (groups []string, found bool) {
	v, ok := raw[name]
	if !ok {
		return nil, false
	}
	switch g := v.(type) {
	case string:
		return []string{g}, true
	case []any:
		out := make([]string, 0, len(g))
		for _, e := range g {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out, true
	default:
		return nil, true
	}
}

func intersects(want, have []string) bool {
	return slices.ContainsFunc(want, func(g string) bool { return slices.Contains(have, g) })
}

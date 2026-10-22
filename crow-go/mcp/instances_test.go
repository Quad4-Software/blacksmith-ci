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

package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/crow-go/crow"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "mcp.yaml")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

func nopFactory(string, TokenFunc) crow.Client { return nil }

func TestLoadConfigMissingFileReturnsNil(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "nope.yaml"))
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigRejectsTwoTokenSources(t *testing.T) {
	p := writeConfig(t, `
instances:
  - name: codefloe
    url: https://ci.codefloe.com
    token_env: CROW_TOKEN
    token_command: ["echo", "x"]
`)
	_, err := LoadConfig(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "codefloe")
}

func TestLoadConfigRejectsURLWithoutScheme(t *testing.T) {
	p := writeConfig(t, `
instances:
  - name: codefloe
    url: ci.codefloe.com
    token_env: CROW_TOKEN
`)
	_, err := LoadConfig(p)
	require.Error(t, err)
}

func TestRegistryByHostAndForgeHost(t *testing.T) {
	reg, err := NewRegistry(&Config{Instances: []Instance{{
		Name:       "codefloe",
		URL:        "https://ci.codefloe.com",
		ForgeHosts: []string{"codefloe.com"},
		TokenEnv:   "X",
	}}}, nopFactory)
	require.NoError(t, err)

	inst, ok := reg.ByHost("ci.codefloe.com")
	assert.True(t, ok)
	assert.Equal(t, "codefloe", inst.Name)

	inst, ok = reg.ByForgeHost("codefloe.com")
	assert.True(t, ok)
	assert.Equal(t, "codefloe", inst.Name)

	_, ok = reg.ByHost("evil.example")
	assert.False(t, ok)
}

func tokenFuncFor(t *testing.T, inst Instance) func(bool) (string, error) {
	t.Helper()
	var got TokenFunc
	reg, err := NewRegistry(&Config{Instances: []Instance{inst}}, func(_ string, tok TokenFunc) crow.Client {
		got = tok
		return nil
	})
	require.NoError(t, err)
	_, err = reg.Client(inst.Name)
	require.NoError(t, err)
	require.NotNil(t, got)
	return func(refresh bool) (string, error) { return got(t.Context(), refresh) }
}

func TestTokenCommandFailureDoesNotLeakOutput(t *testing.T) {
	tok := tokenFuncFor(t, Instance{Name: "ci1", URL: "https://ci1", TokenCommand: []string{"sh", "-c", "echo secret-ish; echo more-secret >&2; exit 3"}})
	_, err := tok(false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ci1")
	assert.NotContains(t, err.Error(), "secret-ish")
	assert.NotContains(t, err.Error(), "more-secret")
}

func TestTokenCommandEmptyOutputIsError(t *testing.T) {
	tok := tokenFuncFor(t, Instance{Name: "ci1", URL: "https://ci1", TokenCommand: []string{"sh", "-c", "printf '  \\n'"}})
	_, err := tok(false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ci1")
}

func TestTokenCommandCachedUntilRefresh(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "count")
	script := "echo x >> " + counter + "; echo tok"
	tok := tokenFuncFor(t, Instance{Name: "ci1", URL: "https://ci1", TokenCommand: []string{"sh", "-c", script}})

	runs := func() int {
		b, _ := os.ReadFile(counter)
		return strings.Count(string(b), "x")
	}

	v, err := tok(false)
	require.NoError(t, err)
	assert.Equal(t, "tok", v)
	_, err = tok(false)
	require.NoError(t, err)
	assert.Equal(t, 1, runs())

	_, err = tok(true)
	require.NoError(t, err)
	assert.Equal(t, 2, runs())
}

func TestTokenEnv(t *testing.T) {
	t.Setenv("CROW_MCP_TEST_TOKEN", "envtok")
	tok := tokenFuncFor(t, Instance{Name: "ci1", URL: "https://ci1", TokenEnv: "CROW_MCP_TEST_TOKEN"})
	v, err := tok(false)
	require.NoError(t, err)
	assert.Equal(t, "envtok", v)
}

func TestTransportRetriesOnceOn401(t *testing.T) {
	var calls atomic.Int32
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := 0
	tok := func(_ context.Context, refresh bool) (string, error) {
		if refresh {
			n++
		}
		if n == 0 {
			return "old", nil
		}
		return "new", nil
	}

	resp, err := newHTTPClient(false, srv.URL, tok).Get(srv.URL)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []string{"Bearer old", "Bearer new"}, seen)
}

func TestTransportReturnsSecond401(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	resp, err := newHTTPClient(false, srv.URL, func(context.Context, bool) (string, error) { return "t", nil }).Get(srv.URL)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, int32(2), calls.Load())
}

func TestTransportDoesNotSendTokenToOtherHosts(t *testing.T) {
	var otherAuth string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer other.Close()
	own := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/x", http.StatusFound)
	}))
	defer own.Close()

	c := newHTTPClient(false, own.URL, func(context.Context, bool) (string, error) { return "secret", nil })
	resp, err := c.Get(own.URL)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Empty(t, otherAuth)
}

func TestStaticInstanceToken(t *testing.T) {
	inst := NewStaticInstance("https://ci.example.com", "tok")
	assert.Equal(t, "ci.example.com", inst.Name)
	tok := tokenFuncFor(t, inst)
	v, err := tok(false)
	require.NoError(t, err)
	assert.Equal(t, "tok", v)
}

func TestSingleInstanceWithoutForgeHostsMatchesAnyRemote(t *testing.T) {
	reg, err := NewRegistry(&Config{Instances: []Instance{NewStaticInstance("https://ci.example.com", "tok")}}, nopFactory)
	require.NoError(t, err)
	inst, ok := reg.ByForgeHost("codefloe.com")
	assert.True(t, ok)
	assert.Equal(t, "ci.example.com", inst.Name)
}

func TestForgeHostWildcardOnlyForSingleInstance(t *testing.T) {
	reg, err := NewRegistry(&Config{Instances: []Instance{
		{Name: "a", URL: "https://a", TokenEnv: "X"},
		{Name: "b", URL: "https://b", TokenEnv: "X"},
	}}, nopFactory)
	require.NoError(t, err)
	_, ok := reg.ByForgeHost("codefloe.com")
	assert.False(t, ok)
}

func TestHTTPClientHasTimeouts(t *testing.T) {
	c := newHTTPClient(false, "https://ci.example.com", func(context.Context, bool) (string, error) { return "t", nil })
	assert.Positive(t, c.Timeout)
	tr, ok := c.Transport.(*tokenTransport)
	require.True(t, ok)
	base, ok := tr.base.(*http.Transport)
	require.True(t, ok)
	assert.Positive(t, base.TLSHandshakeTimeout)
	assert.Positive(t, base.ResponseHeaderTimeout)
}

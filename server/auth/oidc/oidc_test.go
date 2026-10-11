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

package oidc_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/auth/oidc"
	"codefloe.com/crowci/crow/v6/server/auth/oidc/oidctest"
)

func newProvider(t *testing.T, idp *oidctest.Server, mutate func(*oidc.Config)) *oidc.Provider {
	t.Helper()
	cfg := oidc.Config{
		Issuer:       idp.Issuer(),
		ClientID:     idp.ClientID,
		ClientSecret: idp.ClientSecret,
		RedirectURL:  "https://crow.example/authorize/oidc/callback",
		Scopes:       []string{"profile", "email"},
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return oidc.New(cfg)
}

// signIn runs the code flow against the fake IdP and returns the verified claims.
func signIn(t *testing.T, p *oidc.Provider, idp *oidctest.Server, claims map[string]any) (*oidc.Claims, error) {
	t.Helper()
	state, nonce, verifier := oidc.RandomString(), oidc.RandomString(), oidc.RandomString()
	authURL, err := p.AuthCodeURL(t.Context(), state, nonce, verifier)
	require.NoError(t, err)
	assert.Contains(t, authURL, "code_challenge_method=S256")
	assert.Contains(t, authURL, "scope=openid+profile+email")
	code := idp.IssueCode(t, authURL, claims)
	return p.Exchange(t.Context(), code, verifier, nonce)
}

func TestConfigValidate(t *testing.T) {
	assert.NoError(t, oidc.Config{}.Validate())
	assert.EqualError(t, oidc.Config{Only: true}.Validate(), "CROW_OIDC_ONLY requires CROW_OIDC_ISSUER")
	assert.EqualError(t, oidc.Config{Issuer: "https://idp"}.Validate(), "CROW_OIDC_ISSUER requires CROW_OIDC_CLIENT_ID and CROW_OIDC_CLIENT_SECRET")
	assert.NoError(t, oidc.Config{Issuer: "https://idp", ClientID: "id", ClientSecret: "secret", Only: true}.Validate())
	assert.EqualError(t, oidc.Config{Issuer: "https://idp/" + strings.Repeat("a", 250), ClientID: "id", ClientSecret: "secret"}.Validate(), "CROW_OIDC_ISSUER must not be longer than 255 characters")
}

func TestExchange(t *testing.T) {
	idp := oidctest.New(t)

	t.Run("returns claims from the ID token", func(t *testing.T) {
		claims, err := signIn(t, newProvider(t, idp, nil), idp, map[string]any{
			"sub": "sub-1", "email": "jane@example.com", "preferred_username": "jane", "groups": []string{"crow-users"},
		})
		require.NoError(t, err)
		assert.Equal(t, &oidc.Claims{Subject: "sub-1", Email: "jane@example.com", Username: "jane", Groups: []string{"crow-users"}}, claims)
	})

	t.Run("uses configured claim names", func(t *testing.T) {
		p := newProvider(t, idp, func(c *oidc.Config) { c.UsernameClaim = "nickname"; c.GroupsClaim = "roles" })
		claims, err := signIn(t, p, idp, map[string]any{"sub": "sub-1", "nickname": "jj", "roles": []string{"ops"}})
		require.NoError(t, err)
		assert.Equal(t, "jj", claims.Username)
		assert.Equal(t, []string{"ops"}, claims.Groups)
	})

	t.Run("falls back to userinfo for groups", func(t *testing.T) {
		idp.UserInfo = map[string]any{"sub": "sub-1", "groups": []string{"from-userinfo"}}
		t.Cleanup(func() { idp.UserInfo = nil })
		claims, err := signIn(t, newProvider(t, idp, func(c *oidc.Config) { c.AllowedGroups = []string{"crow-users"} }), idp, map[string]any{"sub": "sub-1"})
		require.NoError(t, err)
		assert.Equal(t, []string{"from-userinfo"}, claims.Groups)

		admins := newProvider(t, idp, func(c *oidc.Config) { c.AdminGroups = []string{"crow-admins"} })
		claims, err = signIn(t, admins, idp, map[string]any{"sub": "sub-1"})
		require.NoError(t, err)
		assert.Equal(t, []string{"from-userinfo"}, claims.Groups)
	})

	t.Run("userinfo fallback for role mapping", func(t *testing.T) {
		idp.UserInfo = map[string]any{"sub": "sub-1", "groups": []string{"audit"}}
		t.Cleanup(func() { idp.UserInfo = nil })
		p := newProvider(t, idp, func(c *oidc.Config) { c.RoleGroups = map[string][]string{"auditor": {"audit"}} })
		claims, err := signIn(t, p, idp, map[string]any{"sub": "sub-1"})
		require.NoError(t, err)
		assert.Equal(t, []string{"audit"}, claims.Groups)
	})

	t.Run("skips userinfo when no group checks are configured", func(t *testing.T) {
		idp.UserInfoStatus = http.StatusInternalServerError
		t.Cleanup(func() { idp.UserInfoStatus = 0 })
		claims, err := signIn(t, newProvider(t, idp, nil), idp, map[string]any{"sub": "sub-1"})
		require.NoError(t, err)
		assert.Empty(t, claims.Groups)
	})

	t.Run("rejects a userinfo subject mismatch", func(t *testing.T) {
		idp.UserInfo = map[string]any{"sub": "someone-else", "groups": []string{"admins"}}
		t.Cleanup(func() { idp.UserInfo = nil })
		_, err := signIn(t, newProvider(t, idp, func(c *oidc.Config) { c.AllowedGroups = []string{"crow-users"} }), idp, map[string]any{"sub": "sub-1"})
		assert.ErrorContains(t, err, "userinfo subject")
	})

	t.Run("surfaces a failing userinfo endpoint", func(t *testing.T) {
		idp.UserInfoStatus = http.StatusInternalServerError
		t.Cleanup(func() { idp.UserInfoStatus = 0 })
		_, err := signIn(t, newProvider(t, idp, func(c *oidc.Config) { c.AllowedGroups = []string{"crow-users"} }), idp, map[string]any{"sub": "sub-1"})
		assert.ErrorContains(t, err, "fetch userinfo")
	})

	t.Run("rejects a nonce mismatch", func(t *testing.T) {
		_, err := signIn(t, newProvider(t, idp, nil), idp, map[string]any{"sub": "sub-1", "nonce": "replayed"})
		assert.ErrorContains(t, err, "nonce")
	})

	t.Run("rejects a token for another client", func(t *testing.T) {
		idp.Audience = "someone-else"
		t.Cleanup(func() { idp.Audience = "" })
		_, err := signIn(t, newProvider(t, idp, nil), idp, map[string]any{"sub": "sub-1"})
		assert.ErrorContains(t, err, "audience")
	})

	t.Run("rejects a wrong PKCE verifier", func(t *testing.T) {
		p := newProvider(t, idp, nil)
		nonce := oidc.RandomString()
		authURL, err := p.AuthCodeURL(t.Context(), oidc.RandomString(), nonce, oidc.RandomString())
		require.NoError(t, err)
		code := idp.IssueCode(t, authURL, map[string]any{"sub": "sub-1"})
		_, err = p.Exchange(t.Context(), code, oidc.RandomString(), nonce)
		assert.Error(t, err)
	})
}

func TestDiscoveryIsRetried(t *testing.T) {
	p := oidc.New(oidc.Config{Issuer: "http://127.0.0.1:1", ClientID: "id", ClientSecret: "secret"})
	assert.Error(t, p.Init(t.Context()))
	_, err := p.AuthCodeURL(t.Context(), "s", "n", "v")
	assert.Error(t, err, "an unreachable issuer keeps failing instead of caching a nil provider")
}

func TestGroupsClaim(t *testing.T) {
	idp := oidctest.New(t)
	p := newProvider(t, idp, nil)

	claims, err := signIn(t, p, idp, map[string]any{"sub": "sub-1", "groups": "single-group"})
	require.NoError(t, err)
	assert.Equal(t, []string{"single-group"}, claims.Groups)

	idp.UserInfo = map[string]any{"sub": "sub-1"}
	t.Cleanup(func() { idp.UserInfo = nil })
	claims, err = signIn(t, newProvider(t, idp, func(c *oidc.Config) { c.AllowedGroups = []string{"crow-users"} }), idp, map[string]any{"sub": "sub-1"})
	require.NoError(t, err)
	assert.Empty(t, claims.Groups)
}

func TestGroupChecks(t *testing.T) {
	p := oidc.New(oidc.Config{AllowedGroups: []string{"crow-users"}, AdminGroups: []string{"crow-admins"}})
	assert.True(t, p.RestrictsGroups())
	assert.True(t, p.ManagesAdmin())
	assert.True(t, p.IsAllowed([]string{"other", "crow-users"}))
	assert.False(t, p.IsAllowed([]string{"other"}))
	assert.False(t, p.IsAllowed(nil))
	assert.True(t, p.IsAdmin([]string{"crow-admins"}))
	assert.False(t, p.IsAdmin([]string{"crow-users"}))

	open := oidc.New(oidc.Config{})
	assert.False(t, open.RestrictsGroups())
	assert.False(t, open.ManagesAdmin())
	assert.True(t, open.IsAllowed(nil))
	assert.False(t, open.IsAdmin([]string{"crow-admins"}))
	assert.Equal(t, "Single sign-on", open.DisplayName())
	assert.Equal(t, "https://idp.example", oidc.New(oidc.Config{Issuer: "https://idp.example"}).Issuer())
}

func TestNormalizeUsername(t *testing.T) {
	for raw, want := range map[string]string{
		"jane":              "jane",
		"jane.doe":          "jane.doe",
		"jane@corp.example": "jane",
		"Jane_Doe-2":        "Jane_Doe-2",
	} {
		got, ok := oidc.NormalizeUsername(raw)
		assert.True(t, ok, raw)
		assert.Equal(t, want, got, raw)
	}
	for _, raw := range []string{"", "@corp.example", "jane doe", "jäne"} {
		_, ok := oidc.NormalizeUsername(raw)
		assert.False(t, ok, raw)
	}
}

func TestProviderRoles(t *testing.T) {
	p := oidc.New(oidc.Config{Issuer: "https://idp.example", RoleGroups: map[string][]string{
		"operator":        {"ops", "sre"},
		"auditor":         {"audit"},
		"secrets-manager": nil,
	}})
	assert.Equal(t, []string{"auditor", "operator"}, p.ManagedRoles())
	assert.Equal(t, []string{"operator"}, p.RolesFor([]string{"sre", "dev"}))
	assert.Equal(t, []string{"auditor", "operator"}, p.RolesFor([]string{"audit", "ops"}))
	assert.Empty(t, p.RolesFor(nil))
	assert.True(t, p.ManagesGroups())
	assert.False(t, oidc.New(oidc.Config{Issuer: "https://idp.example"}).ManagesGroups())
}

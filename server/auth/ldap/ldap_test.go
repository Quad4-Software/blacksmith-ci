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

package ldap_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/auth/ldap"
	"codefloe.com/crowci/crow/v6/server/auth/ldap/ldaptest"
)

const (
	janeDN   = "uid=jane,ou=people,dc=example,dc=org"
	janeUUID = "6d1b5f0e-0000-4000-8000-000000000001"
	adminsDN = "cn=crow-admins,ou=groups,dc=example,dc=org"
)

func setup(t *testing.T, mutate func(*ldap.Config)) (*ldaptest.Server, *ldap.Provider) {
	t.Helper()
	srv := ldaptest.New(t, []ldaptest.User{
		{DN: janeDN, Password: "jane-pw", Attrs: map[string][]string{
			"uid":       {"jane"},
			"mail":      {"jane@example.org"},
			"entryUUID": {janeUUID},
			"memberOf":  {adminsDN},
		}},
		{DN: "uid=janet,ou=people,dc=example,dc=org", Password: "janet-pw", Attrs: map[string][]string{
			"uid":       {"janet"},
			"entryUUID": {"6d1b5f0e-0000-4000-8000-000000000002"},
		}},
		{DN: "uid=dup,ou=people,dc=example,dc=org", Password: "dup-pw", Attrs: map[string][]string{
			"uid": {"dup"}, "entryUUID": {"6d1b5f0e-0000-4000-8000-000000000003"},
		}},
		{DN: "uid=dup,ou=other,dc=example,dc=org", Password: "dup-pw", Attrs: map[string][]string{
			"uid": {"dup"}, "entryUUID": {"6d1b5f0e-0000-4000-8000-000000000004"},
		}},
		{DN: "uid=noid,ou=people,dc=example,dc=org", Password: "noid-pw", Attrs: map[string][]string{
			"uid": {"noid"},
		}},
	}, []ldaptest.Group{
		{DN: "cn=ops,ou=groups,dc=example,dc=org", Members: []string{janeDN}},
	})
	cfg := ldap.Config{
		URL:          srv.URL,
		BindDN:       ldaptest.ServiceDN,
		BindPassword: ldaptest.ServicePassword,
		UserBaseDN:   "dc=example,dc=org",
	}
	if mutate != nil {
		mutate(&cfg)
	}
	p, err := ldap.New(cfg)
	require.NoError(t, err)
	return srv, p
}

func TestAuthenticate(t *testing.T) {
	ctx := t.Context()

	t.Run("signs in with valid credentials", func(t *testing.T) {
		_, p := setup(t, nil)
		claims, err := p.Authenticate(ctx, "jane", "jane-pw")
		require.NoError(t, err)
		assert.Equal(t, &ldap.Claims{
			Subject:  janeUUID,
			Email:    "jane@example.org",
			Username: "jane",
			Groups:   []string{adminsDN},
		}, claims)
	})

	t.Run("rejects a wrong password", func(t *testing.T) {
		_, p := setup(t, nil)
		_, err := p.Authenticate(ctx, "jane", "wrong")
		assert.ErrorIs(t, err, ldap.ErrInvalidCredentials)
	})

	t.Run("rejects an unknown user", func(t *testing.T) {
		_, p := setup(t, nil)
		_, err := p.Authenticate(ctx, "nobody", "pw")
		assert.ErrorIs(t, err, ldap.ErrInvalidCredentials)
	})

	t.Run("rejects an empty password without binding as the user", func(t *testing.T) {
		srv, p := setup(t, nil)
		_, err := p.Authenticate(ctx, "jane", "")
		assert.ErrorIs(t, err, ldap.ErrInvalidCredentials)
		assert.NotContains(t, srv.Binds(), janeDN)
		assert.Empty(t, srv.Binds())
	})

	t.Run("rejects a blank username", func(t *testing.T) {
		srv, p := setup(t, nil)
		_, err := p.Authenticate(ctx, "  ", "pw")
		assert.ErrorIs(t, err, ldap.ErrInvalidCredentials)
		assert.Empty(t, srv.Binds())
	})

	t.Run("escapes filter metacharacters", func(t *testing.T) {
		_, p := setup(t, nil)
		_, err := p.Authenticate(ctx, "jan*", "janet-pw")
		assert.ErrorIs(t, err, ldap.ErrInvalidCredentials)
		_, err = p.Authenticate(ctx, "jane)(uid=*", "jane-pw")
		assert.ErrorIs(t, err, ldap.ErrInvalidCredentials)
	})

	t.Run("rejects an ambiguous username", func(t *testing.T) {
		_, p := setup(t, nil)
		_, err := p.Authenticate(ctx, "dup", "dup-pw")
		assert.ErrorIs(t, err, ldap.ErrInvalidCredentials)
	})

	t.Run("searches anonymously without a bind DN", func(t *testing.T) {
		srv, p := setup(t, func(c *ldap.Config) {
			c.BindDN = ""
			c.BindPassword = ""
		})
		claims, err := p.Authenticate(ctx, "jane", "jane-pw")
		require.NoError(t, err)
		assert.Equal(t, janeUUID, claims.Subject)
		assert.Equal(t, []string{janeDN}, srv.Binds())
	})

	t.Run("reads groups from a group search", func(t *testing.T) {
		_, p := setup(t, func(c *ldap.Config) {
			c.GroupSearchFilter = "(member={dn})"
			c.GroupBaseDN = "ou=groups,dc=example,dc=org"
		})
		claims, err := p.Authenticate(ctx, "jane", "jane-pw")
		require.NoError(t, err)
		assert.Equal(t, []string{"cn=ops,ou=groups,dc=example,dc=org"}, claims.Groups)
	})

	t.Run("reports a missing ID attribute as a directory error", func(t *testing.T) {
		_, p := setup(t, nil)
		_, err := p.Authenticate(ctx, "noid", "noid-pw")
		assert.ErrorIs(t, err, ldap.ErrDirectory)
		assert.False(t, errors.Is(err, ldap.ErrInvalidCredentials))
	})

	t.Run("reports an unreachable directory", func(t *testing.T) {
		_, p := setup(t, func(c *ldap.Config) { c.URL = "ldap://127.0.0.1:1" })
		_, err := p.Authenticate(ctx, "jane", "jane-pw")
		assert.ErrorIs(t, err, ldap.ErrDirectory)
	})

	t.Run("reports a wrong service password as a directory error", func(t *testing.T) {
		_, p := setup(t, func(c *ldap.Config) { c.BindPassword = "nope" })
		_, err := p.Authenticate(ctx, "jane", "jane-pw")
		assert.ErrorIs(t, err, ldap.ErrDirectory)
		assert.False(t, errors.Is(err, ldap.ErrInvalidCredentials))
		assert.NotContains(t, err.Error(), "nope")
	})

	t.Run("never includes the password in errors", func(t *testing.T) {
		_, p := setup(t, nil)
		_, err := p.Authenticate(ctx, "jane", "s3cret-guess")
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "s3cret-guess")
	})
}

func TestValidate(t *testing.T) {
	valid := ldap.Config{URL: "ldap://dir.example", UserBaseDN: "dc=example,dc=org"}
	for _, tc := range []struct {
		name   string
		mutate func(*ldap.Config)
		want   string
	}{
		{"valid", func(*ldap.Config) {}, ""},
		{"disabled", func(c *ldap.Config) { *c = ldap.Config{} }, ""},
		{"ldaps", func(c *ldap.Config) { c.URL = "ldaps://dir.example" }, ""},
		{"missing base DN", func(c *ldap.Config) { c.UserBaseDN = "" }, "CROW_LDAP_URL requires CROW_LDAP_USER_BASE_DN"},
		{"bad scheme", func(c *ldap.Config) { c.URL = "https://dir.example" }, "CROW_LDAP_URL must start with ldap:// or ldaps://"},
		{"start TLS with ldaps", func(c *ldap.Config) {
			c.URL = "ldaps://dir.example"
			c.StartTLS = true
		}, "CROW_LDAP_START_TLS cannot be used with an ldaps:// URL"},
		{"user filter", func(c *ldap.Config) { c.UserFilter = "(uid=jane)" }, "CROW_LDAP_USER_FILTER must contain {username}"},
		{"group search filter", func(c *ldap.Config) { c.GroupSearchFilter = "(member=x)" }, "CROW_LDAP_GROUP_SEARCH_FILTER must contain {dn} or {username}"},
		{"group search filter with username", func(c *ldap.Config) { c.GroupSearchFilter = "(memberUid={username})" }, ""},
		{"long URL", func(c *ldap.Config) { c.URL = "ldap://" + strings.Repeat("a", 249) }, "CROW_LDAP_URL must not be longer than 255 characters"},
		{"bind DN without password", func(c *ldap.Config) { c.BindDN = "cn=svc" }, "CROW_LDAP_BIND_DN requires CROW_LDAP_BIND_PASSWORD"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.mutate(&cfg)
			err := cfg.Validate()
			if tc.want == "" {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tc.want)
		})
	}
}

func TestProviderMapping(t *testing.T) {
	p, err := ldap.New(ldap.Config{URL: "LDAP://Dir.Example:389", UserBaseDN: "dc=x", AdminGroups: []string{"crow-admins"}})
	require.NoError(t, err)
	assert.Equal(t, "ldap://dir.example", p.Issuer())

	// Switching to TLS or another port keeps the identities.
	for _, u := range []string{"ldaps://dir.example:636", "ldap://DIR.example", "ldaps://dir.example"} {
		other, err := ldap.New(ldap.Config{URL: u, UserBaseDN: "dc=x"})
		require.NoError(t, err)
		assert.Equal(t, "ldap://dir.example", other.Issuer(), u)
	}
	assert.Equal(t, "ldap", p.Name())
	assert.Equal(t, "LDAP", p.DisplayName())
	assert.True(t, p.ManagesAdmin())
	assert.True(t, p.IsAdmin([]string{"cn=crow-admins,ou=groups,dc=x"}))
	assert.False(t, p.IsAdmin([]string{"cn=other,ou=groups,dc=x"}))
}

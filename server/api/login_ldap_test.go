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

package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/api"
	"codefloe.com/crowci/crow/v6/server/auth/ldap"
	"codefloe.com/crowci/crow/v6/server/auth/ldap/ldaptest"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/services/permissions"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

const (
	ldapJaneDN   = "uid=jane,ou=people,dc=example,dc=org"
	ldapJaneUUID = "6d1b5f0e-0000-4000-8000-000000000001"
	ldapClientIP = "203.0.113.7"
)

// setupLDAP starts a directory with jane and a few other users, configures
// the LDAP provider against it and starts from clean limiters.
func setupLDAP(t *testing.T, mutate func(*ldap.Config)) *ldaptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	users := []ldaptest.User{{DN: ldapJaneDN, Password: "jane-pw", Attrs: map[string][]string{
		"uid":       {"jane"},
		"mail":      {"jane@example.org"},
		"entryUUID": {ldapJaneUUID},
		"memberOf":  {"cn=crow-admins,ou=groups,dc=example,dc=org"},
	}}}
	for i := range 40 {
		uid := fmt.Sprintf("user%d", i)
		users = append(users, ldaptest.User{DN: "uid=" + uid + ",ou=people,dc=example,dc=org", Password: uid + "-pw", Attrs: map[string][]string{
			"uid": {uid}, "entryUUID": {fmt.Sprintf("6d1b5f0e-0000-4000-8000-%012d", 100+i)},
		}})
	}
	srv := ldaptest.New(t, users, nil)
	configureLDAP(t, srv.URL, mutate)

	server.Config.Server.Host = "https://crow.example"
	server.Config.Server.JWTSecret = "jwt-secret"
	server.Config.Server.SessionExpires = time.Hour
	server.Config.Permissions.Open = false
	server.Config.Permissions.Admins = permissions.NewAdmins(nil)
	api.ResetLDAPLimiters()
	t.Cleanup(func() { server.Config.Services.LDAP = nil })
	return srv
}

func configureLDAP(t *testing.T, url string, mutate func(*ldap.Config)) {
	t.Helper()
	cfg := ldap.Config{
		URL:          url,
		BindDN:       ldaptest.ServiceDN,
		BindPassword: ldaptest.ServicePassword,
		UserBaseDN:   "dc=example,dc=org",
	}
	if mutate != nil {
		mutate(&cfg)
	}
	p, err := ldap.New(cfg)
	require.NoError(t, err)
	server.Config.Services.LDAP = p
}

func ldapBody(t *testing.T, username, password string) string {
	t.Helper()
	b, err := json.Marshal(map[string]string{"username": username, "password": password})
	require.NoError(t, err)
	return string(b)
}

func postLDAP(t *testing.T, body, contentType, origin string, _store *store_mocks.MockStore) *httptest.ResponseRecorder {
	t.Helper()
	return postLDAPFrom(t, ldapClientIP, body, contentType, origin, _store)
}

func postLDAPFrom(t *testing.T, clientIP, body, contentType, origin string, _store *store_mocks.MockStore) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/authorize/ldap", strings.NewReader(body))
	if contentType != "" {
		c.Request.Header.Set("Content-Type", contentType)
	}
	if origin != "" {
		c.Request.Header.Set("Origin", origin)
	}
	c.Request.RemoteAddr = net.JoinHostPort(clientIP, "1234")
	c.Set("store", _store)
	api.HandleLDAPLogin(c)
	return w
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body.Error
}

// ldapIssuer is the issuer of the ldaptest server: its host, without the
// random port.
const ldapIssuer = "ldap://127.0.0.1"

// returningJane expects the store calls of a sign-in of jane's existing identity.
func returningJane(user *model.User) *store_mocks.MockStore {
	_store := new(store_mocks.MockStore)
	_store.On("UserIdentityGetByProviderSubject", "ldap", ldapIssuer, ldapJaneUUID).Return(&model.UserIdentity{ID: 3, UserID: 42, Provider: "ldap"}, nil)
	_store.On("GetUser", int64(42)).Return(user, nil)
	_store.On("UserIdentityUpdate", mock.MatchedBy(func(i *model.UserIdentity) bool {
		return i.Email == "jane@example.org" && i.Username == "jane" && i.LastLogin > 0
	})).Return(nil)
	_store.On("UserForgeList", int64(42)).Return([]*model.UserForge{}, nil)
	return _store
}

func janeUser() *model.User { return &model.User{ID: 42, Login: "jane", Hash: "hash"} }

func countBinds(srv *ldaptest.Server, dn string) int {
	n := 0
	for _, b := range srv.Binds() {
		if b == dn {
			n++
		}
	}
	return n
}

func TestHandleLDAPLogin(t *testing.T) {
	const jsonType = "application/json"

	t.Run("404 when LDAP is not configured", func(t *testing.T) {
		server.Config.Services.LDAP = nil
		w := postLDAP(t, ldapBody(t, "jane", "jane-pw"), jsonType, "", new(store_mocks.MockStore))
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("signs in an existing identity", func(t *testing.T) {
		setupLDAP(t, nil)
		_store := returningJane(janeUser())

		w := postLDAP(t, ldapBody(t, "jane", "jane-pw"), jsonType, "", _store)

		assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
		assert.True(t, hasSessionCookie(w))
		_store.AssertExpectations(t)
	})

	t.Run("401 for a wrong password", func(t *testing.T) {
		setupLDAP(t, nil)
		w := postLDAP(t, ldapBody(t, "jane", "wrong"), jsonType, "", new(store_mocks.MockStore))
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, "invalid_credentials", errorCode(t, w))
		assert.False(t, hasSessionCookie(w))
	})

	t.Run("400 for a malformed body", func(t *testing.T) {
		setupLDAP(t, nil)
		w := postLDAP(t, "{", jsonType, "", new(store_mocks.MockStore))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "invalid_request", errorCode(t, w))
	})

	t.Run("415 for a form post", func(t *testing.T) {
		srv := setupLDAP(t, nil)
		w := postLDAP(t, "username=jane&password=jane-pw", "application/x-www-form-urlencoded", "", new(store_mocks.MockStore))
		assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
		assert.Equal(t, "unsupported_media_type", errorCode(t, w))
		assert.False(t, hasSessionCookie(w))
		assert.Empty(t, srv.Binds())
	})

	t.Run("403 for a foreign origin", func(t *testing.T) {
		srv := setupLDAP(t, nil)
		w := postLDAP(t, ldapBody(t, "jane", "jane-pw"), jsonType, "https://evil.example", new(store_mocks.MockStore))
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "invalid_origin", errorCode(t, w))
		assert.False(t, hasSessionCookie(w))
		assert.Empty(t, srv.Binds())
	})

	t.Run("204 for the own origin", func(t *testing.T) {
		setupLDAP(t, nil)
		w := postLDAP(t, ldapBody(t, "jane", "jane-pw"), "application/json; charset=utf-8", "https://crow.example", returningJane(janeUser()))
		assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
		assert.True(t, hasSessionCookie(w))
	})

	t.Run("default ports and case do not change the origin", func(t *testing.T) {
		for _, tc := range []struct{ host, origin string }{
			{"https://crow.example:443", "https://crow.example"},
			{"http://crow.example:80", "http://crow.example"},
			{"https://crow.example", "https://crow.example:443"},
			{"HTTPS://Crow.Example", "https://crow.example"},
		} {
			t.Run(tc.host+" "+tc.origin, func(t *testing.T) {
				setupLDAP(t, nil)
				server.Config.Server.Host = tc.host
				w := postLDAP(t, ldapBody(t, "jane", "jane-pw"), jsonType, tc.origin, returningJane(janeUser()))
				assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
			})
		}
	})

	t.Run("403 for another port or scheme", func(t *testing.T) {
		for _, tc := range []struct{ host, origin string }{
			{"https://crow.example:8443", "https://crow.example"},
			{"https://crow.example", "https://crow.example:8443"},
			{"https://crow.example", "http://crow.example"},
			{"http://crow.example:443", "https://crow.example"},
		} {
			t.Run(tc.host+" "+tc.origin, func(t *testing.T) {
				srv := setupLDAP(t, nil)
				server.Config.Server.Host = tc.host
				w := postLDAP(t, ldapBody(t, "jane", "jane-pw"), jsonType, tc.origin, new(store_mocks.MockStore))
				assert.Equal(t, http.StatusForbidden, w.Code)
				assert.Equal(t, "invalid_origin", errorCode(t, w))
				assert.Empty(t, srv.Binds())
			})
		}
	})

	t.Run("429 after five failures for one username", func(t *testing.T) {
		srv := setupLDAP(t, nil)
		for range 5 {
			w := postLDAP(t, ldapBody(t, "jane", "wrong"), jsonType, "", new(store_mocks.MockStore))
			require.Equal(t, http.StatusUnauthorized, w.Code)
		}
		binds := len(srv.Binds())

		for _, username := range []string{"jane", " JANE ", "Jane"} {
			w := postLDAP(t, ldapBody(t, username, "jane-pw"), jsonType, "", new(store_mocks.MockStore))
			assert.Equal(t, http.StatusTooManyRequests, w.Code, username)
			assert.Equal(t, "too_many_attempts", errorCode(t, w))
			retry, err := strconv.Atoi(w.Header().Get("Retry-After"))
			require.NoError(t, err)
			assert.Positive(t, retry)
			assert.False(t, hasSessionCookie(w))
		}
		assert.Len(t, srv.Binds(), binds, "the directory is not contacted while rate limited")
	})

	t.Run("a username blocked from one address still signs in from another", func(t *testing.T) {
		setupLDAP(t, nil)
		for range 5 {
			w := postLDAP(t, ldapBody(t, "jane", "wrong"), jsonType, "", new(store_mocks.MockStore))
			require.Equal(t, http.StatusUnauthorized, w.Code)
		}
		w := postLDAP(t, ldapBody(t, "jane", "jane-pw"), jsonType, "", new(store_mocks.MockStore))
		require.Equal(t, http.StatusTooManyRequests, w.Code)

		w = postLDAPFrom(t, "198.51.100.9", ldapBody(t, "jane", "jane-pw"), jsonType, "", returningJane(janeUser()))
		assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	})

	t.Run("429 after fifty failures for one username from many addresses", func(t *testing.T) {
		srv := setupLDAP(t, nil)
		for i := range 10 {
			for range 5 {
				w := postLDAPFrom(t, fmt.Sprintf("198.51.100.%d", i), ldapBody(t, "jane", "wrong"), jsonType, "", new(store_mocks.MockStore))
				require.Equal(t, http.StatusUnauthorized, w.Code)
			}
		}
		binds := len(srv.Binds())

		w := postLDAPFrom(t, "192.0.2.1", ldapBody(t, "jane", "jane-pw"), jsonType, "", new(store_mocks.MockStore))
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Equal(t, "too_many_attempts", errorCode(t, w))
		assert.Len(t, srv.Binds(), binds)
	})

	t.Run("IPv6 addresses in one /64 share the per-IP limit", func(t *testing.T) {
		setupLDAP(t, nil)
		for i := range 30 {
			ip := fmt.Sprintf("2001:db8:1:2::%x", i+1)
			w := postLDAPFrom(t, ip, ldapBody(t, fmt.Sprintf("user%d", i), "wrong"), jsonType, "", new(store_mocks.MockStore))
			require.Equal(t, http.StatusUnauthorized, w.Code)
		}
		w := postLDAPFrom(t, "2001:db8:1:2:ffff::1", ldapBody(t, "jane", "jane-pw"), jsonType, "", new(store_mocks.MockStore))
		assert.Equal(t, http.StatusTooManyRequests, w.Code)

		w = postLDAPFrom(t, "2001:db8:1:3::1", ldapBody(t, "jane", "jane-pw"), jsonType, "", returningJane(janeUser()))
		assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	})

	t.Run("403 ldap_access_denied outside the allowed groups", func(t *testing.T) {
		setupLDAP(t, func(c *ldap.Config) { c.AllowedGroups = []string{"crow-users"} })
		w := postLDAP(t, ldapBody(t, "jane", "jane-pw"), jsonType, "", new(store_mocks.MockStore))
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "ldap_access_denied", errorCode(t, w))
		assert.False(t, hasSessionCookie(w))
	})

	t.Run("registers a new user when registration is open", func(t *testing.T) {
		setupLDAP(t, nil)
		server.Config.Permissions.Open = true
		t.Cleanup(func() { server.Config.Permissions.Open = false })
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "ldap", ldapIssuer, ldapJaneUUID).Return(nil, types.RecordNotExist)
		_store.On("UserCreateWithIdentity", mock.MatchedBy(func(u *model.User) bool {
			return u.Login == "jane" && u.Email == "jane@example.org" && u.Hash != "" && !u.Admin
		}), mock.MatchedBy(func(i *model.UserIdentity) bool {
			return i.Provider == "ldap" && i.Issuer == ldapIssuer && i.Subject == ldapJaneUUID
		})).Run(func(args mock.Arguments) {
			if u, ok := args.Get(0).(*model.User); ok {
				u.ID = 42
			}
		}).Return(nil)
		_store.On("UserForgeList", int64(42)).Return([]*model.UserForge{}, nil)

		w := postLDAP(t, ldapBody(t, "jane", "jane-pw"), jsonType, "", _store)

		assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
		assert.True(t, hasSessionCookie(w))
		_store.AssertExpectations(t)
	})

	t.Run("403 registration_closed when registration is closed", func(t *testing.T) {
		setupLDAP(t, nil)
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "ldap", ldapIssuer, ldapJaneUUID).Return(nil, types.RecordNotExist)

		w := postLDAP(t, ldapBody(t, "jane", "jane-pw"), jsonType, "", _store)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "registration_closed", errorCode(t, w))
		assert.False(t, hasSessionCookie(w))
		_store.AssertNotCalled(t, "UserCreateWithIdentity", mock.Anything, mock.Anything)
	})

	t.Run("503 when the directory is unreachable, not counted as a failure", func(t *testing.T) {
		srv := setupLDAP(t, nil)
		configureLDAP(t, "ldap://127.0.0.1:1", nil)
		for range 6 {
			w := postLDAP(t, ldapBody(t, "jane", "jane-pw"), jsonType, "", new(store_mocks.MockStore))
			require.Equal(t, http.StatusServiceUnavailable, w.Code)
			assert.Equal(t, "ldap_unavailable", errorCode(t, w))
		}

		configureLDAP(t, srv.URL, nil)
		w := postLDAP(t, ldapBody(t, "jane", "jane-pw"), jsonType, "", returningJane(janeUser()))
		assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	})

	t.Run("concurrent wrong passwords cannot exceed the limit", func(t *testing.T) {
		srv := setupLDAP(t, nil)
		codes := make([]int, 20)
		var wg sync.WaitGroup
		for i := range codes {
			wg.Go(func() {
				codes[i] = postLDAP(t, ldapBody(t, "jane", "wrong"), jsonType, "", new(store_mocks.MockStore)).Code
			})
		}
		wg.Wait()

		assert.LessOrEqual(t, countBinds(srv, ldapJaneDN), 5)
		limited := 0
		for _, code := range codes {
			assert.Contains(t, []int{http.StatusUnauthorized, http.StatusTooManyRequests}, code)
			if code == http.StatusTooManyRequests {
				limited++
			}
		}
		assert.GreaterOrEqual(t, limited, 15)
	})

	t.Run("a successful sign-in does not reset the IP count", func(t *testing.T) {
		setupLDAP(t, nil)
		for i := range 29 {
			w := postLDAP(t, ldapBody(t, fmt.Sprintf("user%d", i), "wrong"), jsonType, "", new(store_mocks.MockStore))
			require.Equal(t, http.StatusUnauthorized, w.Code)
		}
		w := postLDAP(t, ldapBody(t, "jane", "jane-pw"), jsonType, "", returningJane(janeUser()))
		require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

		w = postLDAP(t, ldapBody(t, "user30", "wrong"), jsonType, "", new(store_mocks.MockStore))
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		w = postLDAP(t, ldapBody(t, "user31", "user31-pw"), jsonType, "", new(store_mocks.MockStore))
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	})

	t.Run("sets admin from directory groups", func(t *testing.T) {
		setupLDAP(t, func(c *ldap.Config) { c.AdminGroups = []string{"crow-admins"} })
		_store := returningJane(janeUser())
		_store.On("UpdateUser", mock.MatchedBy(func(u *model.User) bool { return u.Admin })).Return(nil).Once()

		w := postLDAP(t, ldapBody(t, "jane", "jane-pw"), jsonType, "", _store)

		assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
		_store.AssertExpectations(t)
	})
}

func linkLDAP(t *testing.T, _store *store_mocks.MockStore, user *model.User, password string) (*api.LinkLDAPIdentityOutput, error) {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/identities/ldap", nil)
	c.Request.RemoteAddr = ldapClientIP + ":1234"
	c.Set("store", _store)
	c.Set("user", user)
	input := &api.LinkLDAPIdentityInput{}
	input.Body.Username = "jane"
	input.Body.Password = password
	return api.LinkLDAPIdentity(context.WithValue(c.Request.Context(), api.GinContextKey(), c), input)
}

func requireStatus(t *testing.T, err error, status int) {
	t.Helper()
	var se huma.StatusError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, status, se.GetStatus())
}

func TestLinkLDAPIdentity(t *testing.T) {
	user := &model.User{ID: 42, Login: "jane", ForgeRemoteID: "1001"}

	t.Run("404 when LDAP is not configured", func(t *testing.T) {
		server.Config.Services.LDAP = nil
		_, err := linkLDAP(t, new(store_mocks.MockStore), user, "jane-pw")
		requireStatus(t, err, http.StatusNotFound)
	})

	t.Run("links a new identity", func(t *testing.T) {
		setupLDAP(t, nil)
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "ldap", ldapIssuer, ldapJaneUUID).Return(nil, types.RecordNotExist)
		_store.On("UserIdentityList", int64(42)).Return([]*model.UserIdentity{}, nil)
		_store.On("UserIdentityCreate", mock.MatchedBy(func(i *model.UserIdentity) bool {
			return i.UserID == 42 && i.Provider == "ldap" && i.Issuer == ldapIssuer && i.Subject == ldapJaneUUID
		})).Return(nil)

		out, err := linkLDAP(t, _store, user, "jane-pw")

		require.NoError(t, err)
		assert.Equal(t, "ldap", out.Body.Provider)
		assert.Equal(t, ldapJaneUUID, out.Body.Subject)
		_store.AssertExpectations(t)
	})

	t.Run("409 identity_already_linked", func(t *testing.T) {
		setupLDAP(t, nil)
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "ldap", ldapIssuer, ldapJaneUUID).Return(nil, types.RecordNotExist)
		_store.On("UserIdentityList", int64(42)).Return([]*model.UserIdentity{
			{ID: 7, UserID: 42, Provider: "ldap", Issuer: "ldap://old.example", Subject: "old"},
		}, nil)

		_, err := linkLDAP(t, _store, user, "jane-pw")

		requireStatus(t, err, http.StatusConflict)
		assert.Contains(t, err.Error(), "identity_already_linked")
		_store.AssertNotCalled(t, "UserIdentityCreate", mock.Anything)
	})

	t.Run("401 for a wrong password", func(t *testing.T) {
		setupLDAP(t, nil)
		_, err := linkLDAP(t, new(store_mocks.MockStore), user, "wrong")
		requireStatus(t, err, http.StatusUnauthorized)
		assert.Contains(t, err.Error(), "invalid_credentials")
	})

	t.Run("429 with Retry-After when rate limited", func(t *testing.T) {
		setupLDAP(t, nil)
		for range 5 {
			_, err := linkLDAP(t, new(store_mocks.MockStore), user, "wrong")
			requireStatus(t, err, http.StatusUnauthorized)
		}
		_, err := linkLDAP(t, new(store_mocks.MockStore), user, "jane-pw")
		requireStatus(t, err, http.StatusTooManyRequests)
		var he huma.HeadersError
		require.ErrorAs(t, err, &he)
		retry, convErr := strconv.Atoi(he.GetHeaders().Get("Retry-After"))
		require.NoError(t, convErr)
		assert.Positive(t, retry)
	})

	t.Run("403 ldap_access_denied outside the allowed groups", func(t *testing.T) {
		setupLDAP(t, func(c *ldap.Config) { c.AllowedGroups = []string{"crow-users"} })
		_, err := linkLDAP(t, new(store_mocks.MockStore), user, "jane-pw")
		requireStatus(t, err, http.StatusForbidden)
	})

	t.Run("a user with an OIDC identity links LDAP", func(t *testing.T) {
		setupLDAP(t, nil)
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "ldap", ldapIssuer, ldapJaneUUID).Return(nil, types.RecordNotExist)
		_store.On("UserIdentityList", int64(42)).Return([]*model.UserIdentity{
			{ID: 3, UserID: 42, Provider: "oidc", Issuer: "https://idp.example", Subject: "sub-jane"},
		}, nil)
		_store.On("UserIdentityCreate", mock.MatchedBy(func(i *model.UserIdentity) bool {
			return i.Provider == "ldap" && i.Subject == ldapJaneUUID
		})).Return(nil).Once()

		out, err := linkLDAP(t, _store, &model.User{ID: 42, Login: "jane"}, "jane-pw")

		require.NoError(t, err)
		assert.Equal(t, "ldap", out.Body.Provider)
		_store.AssertExpectations(t)
	})
}

func TestClientIPKey(t *testing.T) {
	for ip, want := range map[string]string{
		"203.0.113.7":               "203.0.113.7",
		"::ffff:203.0.113.7":        "203.0.113.7",
		"2001:db8:1:2:3:4:5:6":      "2001:db8:1:2::/64",
		"2001:db8:1:2:ffff:ffff::1": "2001:db8:1:2::/64",
		"fe80::1%eth0":              "fe80::/64",
		"not an address":            "not an address",
	} {
		assert.Equal(t, want, api.ClientIPKey(ip), ip)
	}
}

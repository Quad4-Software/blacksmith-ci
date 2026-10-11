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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/api"
	"codefloe.com/crowci/crow/v6/server/auth/oidc"
	"codefloe.com/crowci/crow/v6/server/auth/oidc/oidctest"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/services/permissions"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

type oidcFlow struct {
	t    *testing.T
	idp  *oidctest.Server
	auth *http.Cookie
	url  string
}

func setupOIDC(t *testing.T, mutate func(*oidc.Config)) *oidctest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	idp := oidctest.New(t)
	idp.UserInfo = map[string]any{"sub": "sub-jane"}
	cfg := oidc.Config{
		Issuer:       idp.Issuer(),
		ClientID:     idp.ClientID,
		ClientSecret: idp.ClientSecret,
		RedirectURL:  "https://crow.example/authorize/oidc/callback",
	}
	if mutate != nil {
		mutate(&cfg)
	}
	server.Config.Services.OIDC = oidc.New(cfg)
	server.Config.Server.JWTSecret = "jwt-secret"
	server.Config.Server.SessionExpires = time.Hour
	server.Config.Permissions.Open = false
	server.Config.Permissions.Admins = permissions.NewAdmins(nil)
	t.Cleanup(func() { server.Config.Services.OIDC = nil })
	return idp
}

// start runs the first leg, optionally with a signed-in user.
func start(t *testing.T, idp *oidctest.Server, sessionUser *model.User) *oidcFlow {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/authorize/oidc", nil)
	if sessionUser != nil {
		c.Set("user", sessionUser)
	}
	api.HandleOIDCAuth(c)
	require.Equal(t, http.StatusSeeOther, w.Code)

	var authCookie *http.Cookie
	for _, ck := range w.Result().Cookies() {
		if ck.Name == "oidc_auth" {
			authCookie = ck
		}
	}
	require.NotNil(t, authCookie, "oidc_auth cookie is set")
	return &oidcFlow{t: t, idp: idp, auth: authCookie, url: w.Header().Get("Location")}
}

// finish completes the flow at the callback and returns the redirect target.
func (f *oidcFlow) finish(_store *store_mocks.MockStore, claims map[string]any, sessionUser *model.User, withCookie bool) *httptest.ResponseRecorder {
	f.t.Helper()
	authURL, err := url.Parse(f.url)
	require.NoError(f.t, err)
	code := f.idp.IssueCode(f.t, f.url, claims)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	q := url.Values{"code": {code}, "state": {authURL.Query().Get("state")}}
	c.Request = httptest.NewRequest(http.MethodGet, "/authorize/oidc/callback?"+q.Encode(), nil)
	if withCookie {
		c.Request.AddCookie(f.auth)
	}
	c.Set("store", _store)
	if sessionUser != nil {
		c.Set("user", sessionUser)
	}
	api.HandleOIDCCallback(c)
	return w
}

func hasSessionCookie(w *httptest.ResponseRecorder) bool {
	for _, ck := range w.Result().Cookies() {
		if ck.Name == "user_sess" && ck.Value != "" {
			return true
		}
	}
	return false
}

func TestHandleOIDC(t *testing.T) {
	jane := map[string]any{"sub": "sub-jane", "email": "jane@example.com", "preferred_username": "jane@corp.example", "groups": []string{"crow-users"}}

	t.Run("redirects to the IdP with state, nonce and PKCE", func(t *testing.T) {
		idp := setupOIDC(t, nil)
		f := start(t, idp, nil)
		assert.Contains(t, f.url, idp.Issuer()+"/authorize?")
		assert.Contains(t, f.url, "code_challenge=")
		assert.Contains(t, f.url, "nonce=")
		assert.True(t, f.auth.HttpOnly)
	})

	t.Run("redirects to oidc_error when OIDC is not configured", func(t *testing.T) {
		server.Config.Services.OIDC = nil
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/authorize/oidc", nil)
		api.HandleOIDCAuth(c)
		assert.Equal(t, "/login?error=oidc_error", w.Header().Get("Location"))
	})

	t.Run("registers a new user when allowed groups match", func(t *testing.T) {
		idp := setupOIDC(t, func(c *oidc.Config) { c.AllowedGroups = []string{"crow-users"} })
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "oidc", idp.Issuer(), "sub-jane").Return(nil, types.RecordNotExist)
		_store.On("UserCreateWithIdentity", mock.MatchedBy(func(u *model.User) bool {
			return u.Login == "jane" && u.Email == "jane@example.com" && u.Hash != "" && !u.Admin
		}), mock.MatchedBy(func(i *model.UserIdentity) bool {
			return i.Provider == "oidc" && i.Issuer == idp.Issuer() && i.Subject == "sub-jane" && i.Username == "jane@corp.example"
		})).Run(func(args mock.Arguments) {
			if u, ok := args.Get(0).(*model.User); ok {
				u.ID = 42
			}
		}).Return(nil)
		_store.On("UserForgeList", int64(42)).Return([]*model.UserForge{}, nil)

		w := start(t, idp, nil).finish(_store, jane, nil, true)

		assert.Equal(t, "/", w.Header().Get("Location"))
		assert.True(t, hasSessionCookie(w))
		_store.AssertExpectations(t)
	})

	t.Run("signs in a returning user and refreshes the identity", func(t *testing.T) {
		idp := setupOIDC(t, nil)
		_store := new(store_mocks.MockStore)
		identity := &model.UserIdentity{ID: 3, UserID: 42, Provider: "oidc", Subject: "sub-jane"}
		_store.On("UserIdentityGetByProviderSubject", "oidc", idp.Issuer(), "sub-jane").Return(identity, nil)
		_store.On("GetUser", int64(42)).Return(&model.User{ID: 42, Login: "jane", Hash: "hash"}, nil)
		_store.On("UserIdentityUpdate", mock.MatchedBy(func(i *model.UserIdentity) bool {
			return i.Email == "jane@example.com" && i.LastLogin > 0 && len(i.Groups) == 1
		})).Return(nil)
		_store.On("UserForgeList", int64(42)).Return([]*model.UserForge{}, nil)

		w := start(t, idp, nil).finish(_store, jane, nil, true)

		assert.Equal(t, "/", w.Header().Get("Location"))
		assert.True(t, hasSessionCookie(w))
	})

	t.Run("grants and revokes admin from admin groups", func(t *testing.T) {
		idp := setupOIDC(t, func(c *oidc.Config) { c.AdminGroups = []string{"crow-admins"} })
		for _, tc := range []struct {
			groups    []string
			wasAdmin  bool
			wantAdmin bool
		}{
			{[]string{"crow-admins"}, false, true},
			{[]string{"crow-users"}, true, false},
		} {
			_store := new(store_mocks.MockStore)
			_store.On("UserIdentityGetByProviderSubject", "oidc", idp.Issuer(), "sub-jane").Return(&model.UserIdentity{UserID: 42}, nil)
			_store.On("GetUser", int64(42)).Return(&model.User{ID: 42, Login: "jane", Hash: "hash", Admin: tc.wasAdmin}, nil)
			_store.On("UserIdentityUpdate", mock.Anything).Return(nil)
			_store.On("UpdateUser", mock.MatchedBy(func(u *model.User) bool { return u.Admin == tc.wantAdmin })).Return(nil).Once()
			_store.On("UserForgeList", int64(42)).Return([]*model.UserForge{}, nil)

			w := start(t, idp, nil).finish(_store, map[string]any{"sub": "sub-jane", "groups": tc.groups}, nil, true)

			assert.Equal(t, "/", w.Header().Get("Location"))
			_store.AssertExpectations(t)
		}
	})

	t.Run("does not register a CROW_ADMIN username when registration is closed", func(t *testing.T) {
		idp := setupOIDC(t, nil)
		server.Config.Permissions.Admins = permissions.NewAdmins([]string{"jane"})
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "oidc", idp.Issuer(), "sub-jane").Return(nil, types.RecordNotExist)
		w := start(t, idp, nil).finish(_store, map[string]any{"sub": "sub-jane", "preferred_username": "jane"}, nil, true)
		assert.Equal(t, "/login?error=registration_closed", w.Header().Get("Location"))
		_store.AssertNotCalled(t, "UserCreateWithIdentity", mock.Anything, mock.Anything)
	})

	t.Run("revokes admin of a CROW_ADMIN user when admin groups are managed", func(t *testing.T) {
		idp := setupOIDC(t, func(c *oidc.Config) { c.AdminGroups = []string{"crow-admins"} })
		server.Config.Permissions.Admins = permissions.NewAdmins([]string{"jane"})
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "oidc", idp.Issuer(), "sub-jane").Return(&model.UserIdentity{UserID: 42}, nil)
		_store.On("GetUser", int64(42)).Return(&model.User{ID: 42, Login: "jane", Hash: "hash", Admin: true}, nil)
		_store.On("UserIdentityUpdate", mock.Anything).Return(nil)
		_store.On("UpdateUser", mock.MatchedBy(func(u *model.User) bool { return !u.Admin })).Return(nil).Once()
		_store.On("UserForgeList", int64(42)).Return([]*model.UserForge{}, nil)
		w := start(t, idp, nil).finish(_store, map[string]any{"sub": "sub-jane", "groups": []string{"crow-users"}}, nil, true)
		assert.Equal(t, "/", w.Header().Get("Location"))
		_store.AssertExpectations(t)
	})

	t.Run("denies when groups are missing and sign-in is restricted", func(t *testing.T) {
		idp := setupOIDC(t, func(c *oidc.Config) { c.AllowedGroups = []string{"crow-users"} })
		w := start(t, idp, nil).finish(new(store_mocks.MockStore), map[string]any{"sub": "sub-jane"}, nil, true)
		assert.Equal(t, "/login?error=oidc_access_denied", w.Header().Get("Location"))
	})

	t.Run("refuses registration when it is closed", func(t *testing.T) {
		idp := setupOIDC(t, nil)
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "oidc", idp.Issuer(), "sub-jane").Return(nil, types.RecordNotExist)
		w := start(t, idp, nil).finish(_store, jane, nil, true)
		assert.Equal(t, "/login?error=registration_closed", w.Header().Get("Location"))
	})

	t.Run("refuses a login or org name that is taken", func(t *testing.T) {
		idp := setupOIDC(t, nil)
		server.Config.Permissions.Open = true
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "oidc", idp.Issuer(), "sub-jane").Return(nil, types.RecordNotExist)
		_store.On("UserCreateWithIdentity", mock.Anything, mock.Anything).Return(types.ErrNameTaken)
		w := start(t, idp, nil).finish(_store, jane, nil, true)
		assert.Equal(t, "/login?error=oidc_login_taken", w.Header().Get("Location"))
	})

	t.Run("rejects a subject longer than the OIDC spec allows", func(t *testing.T) {
		idp := setupOIDC(t, nil)
		server.Config.Permissions.Open = true
		_store := new(store_mocks.MockStore)
		w := start(t, idp, nil).finish(_store, map[string]any{"sub": strings.Repeat("s", 256), "preferred_username": "jane"}, nil, true)
		assert.Equal(t, "/login?error=oidc_error", w.Header().Get("Location"))
		_store.AssertNotCalled(t, "UserIdentityGetByProviderSubject", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("refuses an invalid username", func(t *testing.T) {
		idp := setupOIDC(t, nil)
		server.Config.Permissions.Open = true
		_store := new(store_mocks.MockStore)
		_store.On("UserIdentityGetByProviderSubject", "oidc", idp.Issuer(), "sub-jane").Return(nil, types.RecordNotExist)
		w := start(t, idp, nil).finish(_store, map[string]any{"sub": "sub-jane", "preferred_username": "Jane Doe"}, nil, true)
		assert.Equal(t, "/login?error=oidc_invalid_username", w.Header().Get("Location"))
	})

	t.Run("refuses a missing username", func(t *testing.T) {
		idp := setupOIDC(t, nil)
		server.Config.Permissions.Open = true
		for _, claims := range []map[string]any{
			{"sub": "sub-jane"},
			{"sub": "sub-jane", "preferred_username": ""},
		} {
			_store := new(store_mocks.MockStore)
			_store.On("UserIdentityGetByProviderSubject", "oidc", idp.Issuer(), "sub-jane").Return(nil, types.RecordNotExist)
			w := start(t, idp, nil).finish(_store, claims, nil, true)
			assert.Equal(t, "/login?error=oidc_missing_username", w.Header().Get("Location"))
			_store.AssertNotCalled(t, "UserCreateWithIdentity", mock.Anything, mock.Anything)
		}
	})

	t.Run("rejects a callback without the auth cookie", func(t *testing.T) {
		idp := setupOIDC(t, nil)
		w := start(t, idp, nil).finish(new(store_mocks.MockStore), jane, nil, false)
		assert.Equal(t, "/login?error=invalid_state", w.Header().Get("Location"))
	})

	t.Run("rejects a state mismatch", func(t *testing.T) {
		idp := setupOIDC(t, nil)
		f := start(t, idp, nil)
		other := start(t, idp, nil)
		f.auth = other.auth
		w := f.finish(new(store_mocks.MockStore), jane, nil, true)
		assert.Equal(t, "/login?error=invalid_state", w.Header().Get("Location"))
	})

	t.Run("links the identity to the signed-in user", func(t *testing.T) {
		idp := setupOIDC(t, nil)
		user := &model.User{ID: 42, Login: "jane"}
		_store := new(store_mocks.MockStore)
		_store.On("GetUser", int64(42)).Return(user, nil)
		_store.On("UserIdentityGetByProviderSubject", "oidc", idp.Issuer(), "sub-jane").Return(nil, types.RecordNotExist)
		_store.On("UserIdentityList", int64(42)).Return([]*model.UserIdentity{}, nil)
		_store.On("UserForgeList", int64(42)).Return([]*model.UserForge{{UserID: 42, ForgeID: 1}}, nil)
		_store.On("UserIdentityCreate", mock.MatchedBy(func(i *model.UserIdentity) bool { return i.UserID == 42 && i.Issuer == idp.Issuer() })).Return(nil)

		w := start(t, idp, user).finish(_store, jane, user, true)

		assert.Equal(t, "/user?identity_linked=true", w.Header().Get("Location"))
		assert.False(t, hasSessionCookie(w), "linking keeps the current session")
	})

	t.Run("refuses linking a legacy user without a forge link", func(t *testing.T) {
		// Forge sign-in finds such a user only by login, and that lookup skips
		// users with an identity, so linking would lock them out of forge sign-in.
		idp := setupOIDC(t, nil)
		user := &model.User{ID: 42, Login: "jane"}
		_store := new(store_mocks.MockStore)
		_store.On("GetUser", int64(42)).Return(user, nil)
		_store.On("UserIdentityGetByProviderSubject", "oidc", idp.Issuer(), "sub-jane").Return(nil, types.RecordNotExist)
		_store.On("UserIdentityList", int64(42)).Return([]*model.UserIdentity{}, nil)
		_store.On("UserForgeList", int64(42)).Return([]*model.UserForge{}, nil)

		w := start(t, idp, user).finish(_store, jane, user, true)

		assert.Equal(t, "/user?error=forge_sign_in_required", w.Header().Get("Location"))
		_store.AssertNotCalled(t, "UserIdentityCreate", mock.Anything)
	})

	t.Run("refuses linking an identity owned by someone else", func(t *testing.T) {
		idp := setupOIDC(t, nil)
		user := &model.User{ID: 42, Login: "jane"}
		_store := new(store_mocks.MockStore)
		_store.On("GetUser", int64(42)).Return(user, nil)
		_store.On("UserIdentityGetByProviderSubject", "oidc", idp.Issuer(), "sub-jane").Return(&model.UserIdentity{UserID: 7}, nil)

		w := start(t, idp, user).finish(_store, jane, user, true)

		assert.Equal(t, "/user?error=identity_taken", w.Header().Get("Location"))
	})

	t.Run("refuses linking a second identity", func(t *testing.T) {
		idp := setupOIDC(t, nil)
		user := &model.User{ID: 42, Login: "jane"}
		_store := new(store_mocks.MockStore)
		_store.On("GetUser", int64(42)).Return(user, nil)
		_store.On("UserIdentityGetByProviderSubject", "oidc", idp.Issuer(), "sub-jane").Return(nil, types.RecordNotExist)
		_store.On("UserIdentityList", int64(42)).Return([]*model.UserIdentity{{Provider: "oidc", Subject: "sub-old"}}, nil)

		w := start(t, idp, user).finish(_store, jane, user, true)

		assert.Equal(t, "/user?error=identity_already_linked", w.Header().Get("Location"))
	})

	t.Run("returns link failures to the user settings", func(t *testing.T) {
		idp := setupOIDC(t, func(c *oidc.Config) { c.AllowedGroups = []string{"crow-users"} })
		user := &model.User{ID: 42, Login: "jane"}

		w := start(t, idp, user).finish(new(store_mocks.MockStore), map[string]any{"sub": "sub-jane", "groups": []string{"other"}}, user, true)
		assert.Equal(t, "/user?error=oidc_access_denied", w.Header().Get("Location"))

		w = start(t, idp, user).finish(new(store_mocks.MockStore), map[string]any{"sub": "sub-jane", "nonce": "replayed"}, user, true)
		assert.Equal(t, "/user?error=oidc_error", w.Header().Get("Location"))
	})

	t.Run("rejects linking when the session user changed", func(t *testing.T) {
		idp := setupOIDC(t, nil)
		w := start(t, idp, &model.User{ID: 42}).finish(new(store_mocks.MockStore), jane, &model.User{ID: 43}, true)
		assert.Equal(t, "/login?error=invalid_state", w.Header().Get("Location"))
	})

	t.Run("maps IdP errors to oidc_error", func(t *testing.T) {
		setupOIDC(t, nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/authorize/oidc/callback?error=access_denied", nil)
		api.HandleOIDCCallback(c)
		assert.Equal(t, "/login?error=oidc_error", w.Header().Get("Location"))
	})
}

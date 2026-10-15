package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/api"
	"codefloe.com/crowci/crow/v6/server/auth/oidc"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	forge_types "codefloe.com/crowci/crow/v6/server/forge/types"
	"codefloe.com/crowci/crow/v6/server/model"
	mocks_services "codefloe.com/crowci/crow/v6/server/services/mocks"
	"codefloe.com/crowci/crow/v6/server/services/permissions"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	"codefloe.com/crowci/crow/v6/server/store/types"
	"codefloe.com/crowci/crow/v6/shared/token"
)

func TestHandleAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &model.User{
		ID:            1,
		OrgID:         1,
		ForgeID:       1,
		ForgeRemoteID: "remote-id-1",
		Login:         "test",
		Email:         "test@example.com",
		Admin:         false,
	}
	org := &model.Org{
		ID:   1,
		Name: user.Login,
	}

	server.Config.Server.SessionExpires = time.Hour

	t.Run("should handle errors from the callback", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		query := url.Values{}
		query.Set("error", "invalid_scope")
		query.Set("error_description", "The requested scope is invalid, unknown, or malformed")
		query.Set("error_uri", "https://developer.atlassian.com/cloud/jira/platform/rest/#api-group-OAuth2-ErrorHandling")

		c.Request = &http.Request{
			Header: make(http.Header),
			Method: http.MethodGet,
			URL: &url.URL{
				Scheme:   "https",
				Path:     "/authorize",
				RawQuery: query.Encode(),
			},
		}

		api.HandleAuth(c)

		assert.Equal(t, http.StatusSeeOther, c.Writer.Status())
		assert.Equal(t, fmt.Sprintf("/login?%s", query.Encode()), c.Writer.Header().Get("Location"))
	})

	t.Run("should fail if the state is wrong", func(t *testing.T) {
		_manager := new(mocks_services.MockManager)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager
		server.Config.Permissions.Open = true
		server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
		server.Config.Permissions.Admins = permissions.NewAdmins(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)

		query := url.Values{}
		query.Set("code", "assumed_to_be_valid_code")

		wrongToken := token.New(token.OAuthStateToken)
		wrongToken.Set("forge_id", "1")
		signedWrongToken, _ := wrongToken.Sign("wrong_secret")
		query.Set("state", signedWrongToken)

		c.Request = &http.Request{
			Header: make(http.Header),
			URL: &url.URL{
				Scheme:   "https",
				RawQuery: query.Encode(),
			},
		}

		api.HandleAuth(c)

		assert.Equal(t, http.StatusSeeOther, c.Writer.Status())
		assert.Equal(t, "/login?error=invalid_state", c.Writer.Header().Get("Location"))
	})

	t.Run("should redirect to forge login page", func(t *testing.T) {
		// _manager := new(mocks_services.MockManager)
		_manager := new(mocks_services.MockManager)
		_forge := new(forge_mocks.MockForge)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager
		server.Config.Permissions.Open = true
		server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
		server.Config.Permissions.Admins = permissions.NewAdmins(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)
		c.Request = &http.Request{
			Header: make(http.Header),
			URL: &url.URL{
				Scheme: "https",
			},
		}

		_manager.On("ForgeByID", int64(1)).Return(_forge, nil)

		forgeRedirectURL := ""
		_forge.On("Login", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			state, ok := args.Get(1).(*forge_types.OAuthRequest)
			if ok {
				forgeRedirectURL = fmt.Sprintf("https://my-awesome-forge.com/oauth/authorize?client_id=client-id&state=%s", state.State)
			}
		}).Return(nil, func(context.Context, *forge_types.OAuthRequest) string {
			return forgeRedirectURL
		}, nil)

		api.HandleAuth(c)

		assert.Equal(t, http.StatusSeeOther, c.Writer.Status())
		assert.Equal(t, forgeRedirectURL, c.Writer.Header().Get("Location"))
	})

	t.Run("should register a new user", func(t *testing.T) {
		_manager := new(mocks_services.MockManager)
		_forge := new(forge_mocks.MockForge)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager
		server.Config.Permissions.Open = true
		server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
		server.Config.Permissions.Admins = permissions.NewAdmins(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)
		c.Request = &http.Request{
			Header: make(http.Header),
			URL: &url.URL{
				Scheme: "https",
			},
		}

		_manager.On("ForgeByID", int64(1)).Return(_forge, nil)
		_forge.On("Login", mock.Anything, mock.Anything).Return(user, "", nil)
		_store.On("GetUserByForgeRemoteID", int64(1), user.ForgeRemoteID, user.Login).Return(nil, types.RecordNotExist)
		_store.On("GetUserLogin", user.Login).Return(nil, types.RecordNotExist)
		// Multi-forge: check if UserForge exists for this remote ID
		_store.On("UserForgeGetByRemoteID", int64(1), user.ForgeRemoteID).Return(nil, types.RecordNotExist)
		_store.On("CreateUser", mock.Anything).Return(nil)
		// Multi-forge: create UserForge record for new user
		_store.On("UserForgeCreate", mock.Anything).Return(nil)
		// Multi-forge: update UserForge with refreshed tokens after user update
		_store.On("UserForgeUpdate", mock.Anything).Return(nil)
		_store.On("OrgFindByName", user.Login, user.ForgeID).Return(nil, nil)
		_store.On("OrgCreate", mock.Anything).Return(nil)
		_store.On("UpdateUser", mock.Anything).Return(nil)
		// updateRepoPermissions iterates all linked forges
		_store.On("UserForgeList", mock.Anything).Return([]*model.UserForge{{ForgeID: 1, AccessToken: "token"}}, nil)
		_forge.On("Repos", mock.Anything, mock.Anything).Return(nil, nil)

		api.HandleAuth(c)

		assert.Equal(t, http.StatusSeeOther, c.Writer.Status())
		assert.Equal(t, "/", c.Writer.Header().Get("Location"))
		assert.NotEmpty(t, c.Writer.Header().Get("Set-Cookie"))
	})

	// An account on a second forge must never be able to log in as an existing
	// user, even when it shares that user's login name. Otherwise anyone able to
	// register on any connected forge could take over an arbitrary Crow account
	// by claiming its username.
	t.Run("should not log in as an existing user with the same login on another forge", func(t *testing.T) {
		_manager := new(mocks_services.MockManager)
		_forge := new(forge_mocks.MockForge)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager
		server.Config.Permissions.Open = true
		server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
		server.Config.Permissions.Admins = permissions.NewAdmins(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)
		c.Request = &http.Request{
			Header: make(http.Header),
			URL: &url.URL{
				Scheme:   "https",
				RawQuery: "forge_id=2",
			},
		}

		// The victim is an admin registered on forge 1. The attacker registers the
		// same login on forge 2 and signs in there.
		victim := &model.User{ID: 1, OrgID: 1, ForgeID: 1, ForgeRemoteID: "remote-id-1", Login: "test", Admin: true}
		attacker := &model.User{ForgeID: 2, ForgeRemoteID: "remote-id-2", Login: "test"}

		_manager.On("ForgeByID", int64(2)).Return(_forge, nil)
		_forge.On("Login", mock.Anything, mock.Anything).Return(attacker, "", nil)
		_store.On("UserForgeGetByRemoteID", int64(2), attacker.ForgeRemoteID).Return(nil, types.RecordNotExist)
		// Forge-scoped legacy lookup must not resolve the forge 1 user.
		_store.On("GetUserByForgeRemoteID", int64(2), attacker.ForgeRemoteID, attacker.Login).Return(nil, types.RecordNotExist)
		// The login is already taken by the victim, so registration is refused.
		_store.On("GetUserLogin", attacker.Login).Return(victim, nil)

		api.HandleAuth(c)

		assert.Equal(t, http.StatusSeeOther, c.Writer.Status())
		assert.Equal(t, "/login?error=login_taken", c.Writer.Header().Get("Location"))
		assert.Empty(t, c.Writer.Header().Get("Set-Cookie"), "no session may be issued")
		_store.AssertNotCalled(t, "CreateUser", mock.Anything)
		_store.AssertNotCalled(t, "UserForgeCreate", mock.Anything)
	})

	t.Run("should login an existing user", func(t *testing.T) {
		_manager := new(mocks_services.MockManager)
		_forge := new(forge_mocks.MockForge)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager
		server.Config.Permissions.Open = true
		server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
		server.Config.Permissions.Admins = permissions.NewAdmins(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)
		c.Request = &http.Request{
			Header: make(http.Header),
			URL: &url.URL{
				Scheme: "https",
			},
		}

		// Multi-forge: UserForge lookup finds existing user
		existingUserForge := &model.UserForge{
			ID:            1,
			UserID:        user.ID,
			ForgeID:       user.ForgeID,
			ForgeRemoteID: user.ForgeRemoteID,
			IsPrimary:     true,
		}

		_manager.On("ForgeByID", int64(1)).Return(_forge, nil)
		_forge.On("Login", mock.Anything, mock.Anything).Return(user, "", nil)
		// Multi-forge: first try UserForge lookup (new path)
		_store.On("UserForgeGetByRemoteID", int64(1), user.ForgeRemoteID).Return(existingUserForge, nil)
		_store.On("GetUser", user.ID).Return(user, nil)
		// Multi-forge: update UserForge with refreshed tokens
		_store.On("UserForgeUpdate", mock.Anything).Return(nil)
		_store.On("OrgGet", org.ID).Return(org, nil)
		_store.On("UpdateUser", mock.Anything).Return(nil)
		// updateRepoPermissions iterates all linked forges
		_store.On("UserForgeList", mock.Anything).Return([]*model.UserForge{{ForgeID: 1, AccessToken: "token"}}, nil)
		_forge.On("Repos", mock.Anything, mock.Anything).Return(nil, nil)

		api.HandleAuth(c)

		assert.Equal(t, http.StatusSeeOther, c.Writer.Status())
		assert.Equal(t, "/", c.Writer.Header().Get("Location"))
		assert.NotEmpty(t, c.Writer.Header().Get("Set-Cookie"))
	})

	t.Run("should deny a new user if registration is closed", func(t *testing.T) {
		_manager := new(mocks_services.MockManager)
		_forge := new(forge_mocks.MockForge)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager
		server.Config.Permissions.Open = false
		server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
		server.Config.Permissions.Admins = permissions.NewAdmins(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)
		c.Request = &http.Request{
			Header: make(http.Header),
			URL: &url.URL{
				Scheme: "https",
			},
		}

		_manager.On("ForgeByID", int64(1)).Return(_forge, nil)
		_forge.On("Login", mock.Anything, mock.Anything).Return(user, "", nil)
		// Multi-forge: check if UserForge exists for this remote ID (new user doesn't exist)
		_store.On("UserForgeGetByRemoteID", int64(1), user.ForgeRemoteID).Return(nil, types.RecordNotExist)
		_store.On("GetUserByForgeRemoteID", int64(1), user.ForgeRemoteID, user.Login).Return(nil, types.RecordNotExist)
		_store.On("GetUserLogin", user.Login).Return(nil, types.RecordNotExist)

		api.HandleAuth(c)

		assert.Equal(t, http.StatusSeeOther, c.Writer.Status())
		assert.Equal(t, "/login?error=registration_closed", c.Writer.Header().Get("Location"))
	})

	t.Run("should deny a user with missing org access", func(t *testing.T) {
		_manager := new(mocks_services.MockManager)
		_forge := new(forge_mocks.MockForge)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager
		server.Config.Permissions.Open = true
		server.Config.Permissions.Orgs = permissions.NewOrgs([]string{"org1"})
		server.Config.Permissions.Admins = permissions.NewAdmins(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)
		c.Request = &http.Request{
			Header: make(http.Header),
			URL: &url.URL{
				Scheme: "https",
			},
		}

		_manager.On("ForgeByID", int64(1)).Return(_forge, nil)
		_forge.On("Login", mock.Anything, mock.Anything).Return(user, "", nil)
		_forge.On("Teams", mock.Anything, user).Return([]*model.Team{
			{
				Login: "org2",
			},
		}, nil)

		api.HandleAuth(c)

		assert.Equal(t, http.StatusSeeOther, c.Writer.Status())
		assert.Equal(t, "/login?error=org_access_denied", c.Writer.Header().Get("Location"))
	})

	t.Run("should create an user org if it does not exists", func(t *testing.T) {
		_manager := new(mocks_services.MockManager)
		_forge := new(forge_mocks.MockForge)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager
		server.Config.Permissions.Open = true
		server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
		server.Config.Permissions.Admins = permissions.NewAdmins(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)
		c.Request = &http.Request{
			Header: make(http.Header),
			URL: &url.URL{
				Scheme: "https",
			},
		}
		user.OrgID = 0

		_manager.On("ForgeByID", int64(1)).Return(_forge, nil)
		_forge.On("Login", mock.Anything, mock.Anything).Return(user, "", nil)
		// Multi-forge: legacy user doesn't have UserForge record yet
		_store.On("UserForgeGetByRemoteID", int64(1), user.ForgeRemoteID).Return(nil, types.RecordNotExist)
		_store.On("GetUserByForgeRemoteID", int64(1), user.ForgeRemoteID, user.Login).Return(user, nil)
		_store.On("UserForgeCreate", mock.Anything).Return(nil)
		_store.On("UserForgeUpdate", mock.Anything).Return(nil)
		_store.On("OrgFindByName", user.Login, user.ForgeID).Return(nil, types.RecordNotExist)
		_store.On("OrgCreate", mock.Anything).Return(nil)
		_store.On("UpdateUser", mock.Anything).Return(nil)
		// updateRepoPermissions iterates all linked forges
		_store.On("UserForgeList", mock.Anything).Return([]*model.UserForge{{ForgeID: 1, AccessToken: "token"}}, nil)
		_forge.On("Repos", mock.Anything, mock.Anything).Return(nil, nil)

		api.HandleAuth(c)

		assert.Equal(t, http.StatusSeeOther, c.Writer.Status())
		assert.Equal(t, "/", c.Writer.Header().Get("Location"))
		assert.NotEmpty(t, c.Writer.Header().Get("Set-Cookie"))
	})

	t.Run("should link an user org if it has the same name as the user", func(t *testing.T) {
		_manager := new(mocks_services.MockManager)
		_forge := new(forge_mocks.MockForge)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager
		server.Config.Permissions.Open = true
		server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
		server.Config.Permissions.Admins = permissions.NewAdmins(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)
		c.Request = &http.Request{
			Header: make(http.Header),
			URL: &url.URL{
				Scheme: "https",
			},
		}
		user.OrgID = 0

		_manager.On("ForgeByID", int64(1)).Return(_forge, nil)
		_forge.On("Login", mock.Anything, mock.Anything).Return(user, "", nil)
		// Multi-forge: legacy user doesn't have UserForge record yet
		_store.On("UserForgeGetByRemoteID", int64(1), user.ForgeRemoteID).Return(nil, types.RecordNotExist)
		_store.On("GetUserByForgeRemoteID", int64(1), user.ForgeRemoteID, user.Login).Return(user, nil)
		_store.On("UserForgeCreate", mock.Anything).Return(nil)
		_store.On("UserForgeUpdate", mock.Anything).Return(nil)
		_store.On("OrgFindByName", user.Login, user.ForgeID).Return(org, nil)
		_store.On("OrgUpdate", mock.Anything).Return(nil)
		_store.On("UpdateUser", mock.Anything).Return(nil)
		// updateRepoPermissions iterates all linked forges
		_store.On("UserForgeList", mock.Anything).Return([]*model.UserForge{{ForgeID: 1, AccessToken: "token"}}, nil)
		_forge.On("Repos", mock.Anything, mock.Anything).Return(nil, nil)

		api.HandleAuth(c)

		assert.Equal(t, http.StatusSeeOther, c.Writer.Status())
		assert.Equal(t, "/", c.Writer.Header().Get("Location"))
		assert.NotEmpty(t, c.Writer.Header().Get("Set-Cookie"))
	})

	t.Run("should update an user org if the user name was changed", func(t *testing.T) {
		_manager := new(mocks_services.MockManager)
		_forge := new(forge_mocks.MockForge)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager
		server.Config.Permissions.Open = true
		server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
		server.Config.Permissions.Admins = permissions.NewAdmins(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)
		c.Request = &http.Request{
			Header: make(http.Header),
			URL: &url.URL{
				Scheme: "https",
			},
		}
		org.Name = "not-the-user-name"

		_manager.On("ForgeByID", int64(1)).Return(_forge, nil)
		_forge.On("Login", mock.Anything, mock.Anything).Return(user, "", nil)
		// Multi-forge: legacy user doesn't have UserForge record yet
		_store.On("UserForgeGetByRemoteID", int64(1), user.ForgeRemoteID).Return(nil, types.RecordNotExist)
		_store.On("GetUserByForgeRemoteID", int64(1), user.ForgeRemoteID, user.Login).Return(user, nil)
		_store.On("UserForgeCreate", mock.Anything).Return(nil)
		_store.On("UserForgeUpdate", mock.Anything).Return(nil)
		_store.On("OrgGet", user.OrgID).Return(org, nil)
		_store.On("OrgUpdate", mock.Anything).Return(nil)
		_store.On("UpdateUser", mock.Anything).Return(nil)
		// updateRepoPermissions iterates all linked forges
		_store.On("UserForgeList", mock.Anything).Return([]*model.UserForge{{ForgeID: 1, AccessToken: "token"}}, nil)
		_forge.On("Repos", mock.Anything, mock.Anything).Return(nil, nil)

		api.HandleAuth(c)

		assert.Equal(t, http.StatusSeeOther, c.Writer.Status())
		assert.Equal(t, "/", c.Writer.Header().Get("Location"))
		assert.NotEmpty(t, c.Writer.Header().Get("Set-Cookie"))
	})

	t.Run("first linked forge becomes primary for a user without forges", func(t *testing.T) {
		_manager := new(mocks_services.MockManager)
		_forge := new(forge_mocks.MockForge)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager
		server.Config.Server.JWTSecret = "jwt-secret"
		server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
		server.Config.Permissions.Admins = permissions.NewAdmins(nil)

		ssoUser := &model.User{ID: 42, Login: "jane", Hash: "hash"}
		state := token.New(token.OAuthStateToken)
		state.Set("forge-id", "1")
		state.Set("linking-user-id", "42")
		signedState, _ := state.Sign("jwt-secret")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)
		c.Set("user", ssoUser)
		c.Request = &http.Request{Header: make(http.Header), URL: &url.URL{
			Scheme:   "https",
			RawQuery: url.Values{"code": {"code"}, "state": {signedState}}.Encode(),
		}}

		forgeUser := &model.User{Login: "jane-forge", ForgeRemoteID: "remote-42", AccessToken: "at", Email: "jane@forge.example", Avatar: "avatar"}
		_manager.On("ForgeByID", int64(1)).Return(_forge, nil)
		_forge.On("Login", mock.Anything, mock.Anything).Return(forgeUser, "", nil)
		_store.On("GetUser", int64(42)).Return(ssoUser, nil)
		_store.On("UserForgeGetByRemoteID", int64(1), forgeUser.ForgeRemoteID).Return(nil, types.RecordNotExist)
		_store.On("UserForgeList", int64(42)).Return([]*model.UserForge{}, nil).Once()
		_store.On("UserForgeCreate", mock.MatchedBy(func(uf *model.UserForge) bool { return uf.IsPrimary })).
			Run(func(args mock.Arguments) {
				if uf, ok := args.Get(0).(*model.UserForge); ok {
					uf.ID = 9
				}
			}).Return(nil)
		_store.On("OrgCreate", mock.Anything).Run(func(args mock.Arguments) {
			if o, ok := args.Get(0).(*model.Org); ok {
				o.ID = 5
			}
		}).Return(nil)
		_store.On("UpdateUser", mock.MatchedBy(func(u *model.User) bool {
			return u.PrimaryForgeID == 9 && u.ForgeID == 1 && u.ForgeRemoteID == "remote-42" && u.OrgID == 5 && u.AccessToken == "at"
		})).Return(nil)
		_store.On("UserForgeList", int64(42)).Return([]*model.UserForge{{ForgeID: 1, AccessToken: "at"}}, nil)
		_forge.On("Repos", mock.Anything, mock.Anything).Return(nil, nil)

		api.HandleAuth(c)

		assert.Equal(t, "/user?forge_linked=true", c.Writer.Header().Get("Location"))
		_store.AssertExpectations(t)
	})

	t.Run("first linked forge moves the SSO org onto the forge", func(t *testing.T) {
		_manager := new(mocks_services.MockManager)
		_forge := new(forge_mocks.MockForge)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager
		server.Config.Server.JWTSecret = "jwt-secret"
		server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
		server.Config.Permissions.Admins = permissions.NewAdmins(nil)

		ssoUser := &model.User{ID: 42, Login: "jane", Hash: "hash", OrgID: 5}
		state := token.New(token.OAuthStateToken)
		state.Set("forge-id", "1")
		state.Set("linking-user-id", "42")
		signedState, _ := state.Sign("jwt-secret")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)
		c.Set("user", ssoUser)
		c.Request = &http.Request{Header: make(http.Header), URL: &url.URL{
			Scheme:   "https",
			RawQuery: url.Values{"code": {"code"}, "state": {signedState}}.Encode(),
		}}

		forgeUser := &model.User{Login: "jane-forge", ForgeRemoteID: "remote-42", AccessToken: "at"}
		_manager.On("ForgeByID", int64(1)).Return(_forge, nil)
		_forge.On("Login", mock.Anything, mock.Anything).Return(forgeUser, "", nil)
		_store.On("GetUser", int64(42)).Return(ssoUser, nil)
		_store.On("UserForgeGetByRemoteID", int64(1), forgeUser.ForgeRemoteID).Return(nil, types.RecordNotExist)
		_store.On("UserForgeList", int64(42)).Return([]*model.UserForge{}, nil).Once()
		_store.On("UserForgeCreate", mock.Anything).Run(func(args mock.Arguments) {
			if uf, ok := args.Get(0).(*model.UserForge); ok {
				uf.ID = 9
			}
		}).Return(nil)
		_store.On("OrgGet", int64(5)).Return(&model.Org{ID: 5, Name: "jane", IsUser: true}, nil)
		_store.On("OrgFindByName", "jane-forge", int64(1)).Return(nil, types.RecordNotExist)
		_store.On("OrgUpdate", mock.MatchedBy(func(o *model.Org) bool {
			return o.ID == 5 && o.ForgeID == 1 && o.Name == "jane-forge"
		})).Return(nil)
		_store.On("UpdateUser", mock.MatchedBy(func(u *model.User) bool {
			return u.PrimaryForgeID == 9 && u.ForgeID == 1 && u.OrgID == 5
		})).Return(nil)
		_store.On("UserForgeList", int64(42)).Return([]*model.UserForge{{ForgeID: 1, AccessToken: "at"}}, nil)
		_forge.On("Repos", mock.Anything, mock.Anything).Return(nil, nil)

		api.HandleAuth(c)

		assert.Equal(t, "/user?forge_linked=true", c.Writer.Header().Get("Location"))
		_store.AssertNotCalled(t, "OrgFindByName", "jane", mock.Anything)
		_store.AssertNotCalled(t, "OrgCreate", mock.Anything)
		_store.AssertNotCalled(t, "OrgDelete", mock.Anything)
		_store.AssertExpectations(t)
	})

	t.Run("reconnecting finishes a failed primary forge adoption", func(t *testing.T) {
		_manager := new(mocks_services.MockManager)
		_forge := new(forge_mocks.MockForge)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager
		server.Config.Server.JWTSecret = "jwt-secret"
		server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
		server.Config.Permissions.Admins = permissions.NewAdmins(nil)

		// The primary forge link was stored, but adopting it failed, so the user
		// still has no forge identity.
		ssoUser := &model.User{ID: 42, Login: "jane", Hash: "hash"}
		primary := &model.UserForge{ID: 9, UserID: 42, ForgeID: 1, ForgeRemoteID: "remote-42", IsPrimary: true}
		state := token.New(token.OAuthStateToken)
		state.Set("forge-id", "1")
		state.Set("linking-user-id", "42")
		state.Set("reconnect", "true")
		signedState, _ := state.Sign("jwt-secret")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)
		c.Set("user", ssoUser)
		c.Request = &http.Request{Header: make(http.Header), URL: &url.URL{
			Scheme:   "https",
			RawQuery: url.Values{"code": {"code"}, "state": {signedState}}.Encode(),
		}}

		forgeUser := &model.User{Login: "jane-forge", ForgeRemoteID: "remote-42", AccessToken: "at"}
		_manager.On("ForgeByID", int64(1)).Return(_forge, nil)
		_forge.On("Login", mock.Anything, mock.Anything).Return(forgeUser, "", nil)
		_store.On("GetUser", int64(42)).Return(ssoUser, nil)
		_store.On("UserForgeGetByRemoteID", int64(1), forgeUser.ForgeRemoteID).Return(primary, nil)
		_store.On("UserForgeGet", int64(42), int64(1)).Return(primary, nil)
		_store.On("UserForgeUpdate", mock.Anything).Return(nil)
		_store.On("UserIdentityList", int64(42)).Return([]*model.UserIdentity{{ID: 1, UserID: 42, Provider: "oidc"}}, nil)
		_store.On("OrgCreate", mock.Anything).Run(func(args mock.Arguments) {
			if o, ok := args.Get(0).(*model.Org); ok {
				o.ID = 5
			}
		}).Return(nil)
		_store.On("UpdateUser", mock.MatchedBy(func(u *model.User) bool {
			return u.PrimaryForgeID == 9 && u.ForgeID == 1 && u.ForgeRemoteID == "remote-42" && u.OrgID == 5
		})).Return(nil)
		_store.On("UserForgeList", int64(42)).Return([]*model.UserForge{primary}, nil)
		_forge.On("Repos", mock.Anything, mock.Anything).Return(nil, nil)

		api.HandleAuth(c)

		assert.Equal(t, "/user?forge_reconnected=true", c.Writer.Header().Get("Location"))
		_store.AssertExpectations(t)
	})

	t.Run("rejects a forge link completed by a different session", func(t *testing.T) {
		for name, sessionUser := range map[string]*model.User{"no session": nil, "other user": {ID: 43, Login: "eve"}} {
			_manager := new(mocks_services.MockManager)
			_forge := new(forge_mocks.MockForge)
			_store := new(store_mocks.MockStore)
			server.Config.Services.Manager = _manager
			server.Config.Server.JWTSecret = "jwt-secret"

			state := token.New(token.OAuthStateToken)
			state.Set("forge-id", "1")
			state.Set("linking-user-id", "42")
			signedState, _ := state.Sign("jwt-secret")

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("store", _store)
			if sessionUser != nil {
				c.Set("user", sessionUser)
			}
			c.Request = &http.Request{Header: make(http.Header), URL: &url.URL{
				Scheme:   "https",
				RawQuery: url.Values{"code": {"code"}, "state": {signedState}}.Encode(),
			}}

			api.HandleAuth(c)

			assert.Equal(t, "/login?error=invalid_state", c.Writer.Header().Get("Location"), name)
			_manager.AssertNotCalled(t, "ForgeByID", mock.Anything)
			_forge.AssertNotCalled(t, "Login", mock.Anything, mock.Anything)
		}
	})

	t.Run("linking a second forge keeps the legacy forge identity of a user without user_forges", func(t *testing.T) {
		_manager := new(mocks_services.MockManager)
		_forge := new(forge_mocks.MockForge)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager
		server.Config.Server.JWTSecret = "jwt-secret"
		server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
		server.Config.Permissions.Admins = permissions.NewAdmins(nil)

		legacyUser := &model.User{ID: 42, Login: "jane", Hash: "hash", ForgeID: 1, ForgeRemoteID: "legacy-1", OrgID: 5}
		state := token.New(token.OAuthStateToken)
		state.Set("forge-id", "2")
		state.Set("linking-user-id", "42")
		signedState, _ := state.Sign("jwt-secret")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)
		c.Set("user", legacyUser)
		c.Request = &http.Request{Header: make(http.Header), URL: &url.URL{
			Scheme:   "https",
			RawQuery: url.Values{"code": {"code"}, "state": {signedState}}.Encode(),
		}}

		forgeUser := &model.User{Login: "jane-2", ForgeRemoteID: "remote-2", AccessToken: "at"}
		_manager.On("ForgeByID", int64(2)).Return(_forge, nil)
		_forge.On("Login", mock.Anything, mock.Anything).Return(forgeUser, "", nil)
		_store.On("GetUser", int64(42)).Return(legacyUser, nil)
		_store.On("UserForgeGetByRemoteID", int64(2), forgeUser.ForgeRemoteID).Return(nil, types.RecordNotExist)
		_store.On("UserForgeList", int64(42)).Return([]*model.UserForge{}, nil)
		_store.On("UserForgeCreate", mock.MatchedBy(func(uf *model.UserForge) bool { return !uf.IsPrimary })).Return(nil)

		api.HandleAuth(c)

		assert.Equal(t, "/user?forge_linked=true", c.Writer.Header().Get("Location"))
		_store.AssertNotCalled(t, "UpdateUser", mock.Anything)
		_store.AssertExpectations(t)
	})

	t.Run("blocks forge sign-in when it is disabled", func(t *testing.T) {
		server.Config.Services.OIDC = oidc.New(oidc.Config{Issuer: "https://idp.example"})
		server.Config.Server.ForgeLoginDisabled = true
		t.Cleanup(func() {
			server.Config.Services.OIDC = nil
			server.Config.Server.ForgeLoginDisabled = false
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", new(store_mocks.MockStore))
		c.Request = &http.Request{Header: make(http.Header), URL: &url.URL{Scheme: "https"}}

		api.HandleAuth(c)

		assert.Equal(t, "/login?error=forge_login_disabled", c.Writer.Header().Get("Location"))
	})

	t.Run("blocks add_account when forge sign-in is disabled", func(t *testing.T) {
		server.Config.Services.OIDC = oidc.New(oidc.Config{Issuer: "https://idp.example"})
		server.Config.Server.ForgeLoginDisabled = true
		t.Cleanup(func() {
			server.Config.Services.OIDC = nil
			server.Config.Server.ForgeLoginDisabled = false
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", new(store_mocks.MockStore))
		c.Set("user", &model.User{ID: 42, Login: "jane"})
		c.Request = &http.Request{Header: make(http.Header), URL: &url.URL{Scheme: "https", RawQuery: "add_account=true"}}

		api.HandleAuth(c)

		assert.Equal(t, "/login?error=forge_login_disabled", c.Writer.Header().Get("Location"))
	})

	t.Run("blocks a forge sign-in callback when forge sign-in is disabled", func(t *testing.T) {
		server.Config.Services.OIDC = oidc.New(oidc.Config{Issuer: "https://idp.example"})
		server.Config.Server.ForgeLoginDisabled = true
		server.Config.Server.JWTSecret = "jwt-secret"
		t.Cleanup(func() {
			server.Config.Services.OIDC = nil
			server.Config.Server.ForgeLoginDisabled = false
		})

		state := token.New(token.OAuthStateToken)
		state.Set("forge-id", "1")
		signedState, _ := state.Sign("jwt-secret")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", new(store_mocks.MockStore))
		c.Request = &http.Request{Header: make(http.Header), URL: &url.URL{
			Scheme:   "https",
			RawQuery: url.Values{"code": {"code"}, "state": {signedState}}.Encode(),
		}}

		api.HandleAuth(c)

		assert.Equal(t, "/login?error=forge_login_disabled", c.Writer.Header().Get("Location"))
	})

	t.Run("still allows linking a forge when forge sign-in is disabled", func(t *testing.T) {
		server.Config.Services.OIDC = oidc.New(oidc.Config{Issuer: "https://idp.example"})
		server.Config.Server.ForgeLoginDisabled = true
		t.Cleanup(func() {
			server.Config.Services.OIDC = nil
			server.Config.Server.ForgeLoginDisabled = false
		})
		_manager := new(mocks_services.MockManager)
		_forge := new(forge_mocks.MockForge)
		_store := new(store_mocks.MockStore)
		server.Config.Services.Manager = _manager

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("store", _store)
		c.Set("user", &model.User{ID: 42, Login: "jane"})
		c.Request = &http.Request{Header: make(http.Header), URL: &url.URL{Scheme: "https"}}

		_store.On("UserForgeGet", int64(42), int64(1)).Return(nil, types.RecordNotExist)
		_manager.On("ForgeByID", int64(1)).Return(_forge, nil)
		_forge.On("Login", mock.Anything, mock.Anything).Return(nil, "https://forge.example/oauth", nil)

		api.HandleAuth(c)

		assert.Equal(t, "https://forge.example/oauth", c.Writer.Header().Get("Location"))
	})
}

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
}

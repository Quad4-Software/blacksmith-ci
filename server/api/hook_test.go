package api_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/api"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pubsub"
	mocks_config_service "codefloe.com/crowci/crow/v6/server/services/config/mocks"
	mocks_services "codefloe.com/crowci/crow/v6/server/services/mocks"
	"codefloe.com/crowci/crow/v6/server/services/permissions"
	mocks_registry_service "codefloe.com/crowci/crow/v6/server/services/registry/mocks"
	mocks_secret_service "codefloe.com/crowci/crow/v6/server/services/secret/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
	"codefloe.com/crowci/crow/v6/shared/token"
)

func TestHook(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// _manager := new(mocks_services.MockManager)
	_manager := new(mocks_services.MockManager)
	_forge := new(forge_mocks.MockForge)
	_store := new(store_mocks.MockStore)
	// _configService := mocks_config_service.NewService(t)
	_configService := new(mocks_config_service.MockService)
	// _secretService := mocks_secret_service.NewService(t)
	_secretService := new(mocks_secret_service.MockService)
	// _registryService := mocks_registry_service.NewService(t)
	_registryService := new(mocks_registry_service.MockService)
	server.Config.Services.Manager = _manager
	server.Config.Permissions.Open = true
	server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
	server.Config.Permissions.Admins = permissions.NewAdmins(nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("store", _store)
	user := &model.User{
		ID: 123,
	}
	repo := &model.Repo{
		ID:            123,
		ForgeRemoteID: "123",
		Owner:         "owner",
		Name:          "name",
		IsActive:      true,
		UserID:        user.ID,
		Hash:          "secret-123-this-is-a-secret",
	}
	pipeline := &model.Pipeline{
		ID:     123,
		RepoID: repo.ID,
		Event:  model.EventPush,
	}

	repoToken := token.New(token.HookToken)
	repoToken.Set("repo-id", fmt.Sprintf("%d", repo.ID))
	signedToken, err := repoToken.Sign("secret-123-this-is-a-secret")
	assert.NoError(t, err)

	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", signedToken))
	c.Request = &http.Request{
		Header: header,
		URL: &url.URL{
			Scheme: "https",
		},
	}

	_manager.On("ForgeFromRepo", repo).Return(_forge, nil)
	_forge.On("Hook", mock.Anything, mock.Anything).Return(repo, pipeline, nil)
	_store.On("GetRepo", repo.ID).Return(repo, nil)
	_store.On("GetUser", user.ID).Return(user, nil)
	_store.On("UserForgeGet", user.ID, repo.ForgeID).Return(nil, nil)
	_store.On("UpdateRepo", repo).Return(nil)
	_store.On("CreatePipeline", mock.Anything).Return(nil)
	_manager.On("ConfigServiceFromRepo", repo).Return(_configService)
	_configService.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	_forge.On("Netrc", mock.Anything, mock.Anything).Return(&model.Netrc{}, nil)
	_store.On("GetPipelineLastBefore", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	_manager.On("SecretServiceFromRepo", repo).Return(_secretService)
	_secretService.On("SecretListPipeline", repo, mock.Anything, mock.Anything).Return(nil, nil)
	_manager.On("RegistryServiceFromRepo", repo).Return(_registryService)
	_registryService.On("RegistryListPipeline", repo, mock.Anything).Return(nil, nil)
	_manager.On("EnvironmentService").Return(nil)
	_store.On("DeletePipeline", mock.Anything).Return(nil)

	api.PostHook(c)

	assert.Equal(t, http.StatusNoContent, c.Writer.Status())
	assert.Equal(t, "true", w.Header().Get("Pipeline-Filtered"))
}

// TestHookConfigFetchError verifies the hook handler does not panic when
// the forge config fetch transiently fails (e.g. connectivity/timeout).
// Regression test for a nil pointer dereference in PostHook where
// pipeline.Create returned (nil, nil) on this path.
func TestHookConfigFetchError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_manager := new(mocks_services.MockManager)
	_forge := new(forge_mocks.MockForge)
	_store := new(store_mocks.MockStore)
	_configService := new(mocks_config_service.MockService)

	originalManager := server.Config.Services.Manager
	originalPubsub := server.Config.Services.Pubsub
	server.Config.Services.Manager = _manager
	server.Config.Services.Pubsub = pubsub.New()
	server.Config.Permissions.Open = true
	server.Config.Permissions.Orgs = permissions.NewOrgs(nil)
	server.Config.Permissions.Admins = permissions.NewAdmins(nil)
	t.Cleanup(func() {
		server.Config.Services.Manager = originalManager
		server.Config.Services.Pubsub = originalPubsub
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("store", _store)
	user := &model.User{ID: 123}
	repo := &model.Repo{
		ID:            123,
		ForgeRemoteID: "123",
		Owner:         "owner",
		Name:          "name",
		IsActive:      true,
		UserID:        user.ID,
		Hash:          "secret-123-this-is-a-secret",
	}
	pipeline := &model.Pipeline{
		ID:     123,
		RepoID: repo.ID,
		Event:  model.EventPush,
	}

	repoToken := token.New(token.HookToken)
	repoToken.Set("repo-id", fmt.Sprintf("%d", repo.ID))
	signedToken, err := repoToken.Sign("secret-123-this-is-a-secret")
	assert.NoError(t, err)

	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", signedToken))
	c.Request = &http.Request{
		Header: header,
		URL:    &url.URL{Scheme: "https"},
	}

	_manager.On("ForgeFromRepo", repo).Return(_forge, nil)
	_forge.On("Hook", mock.Anything, mock.Anything).Return(repo, pipeline, nil)
	_store.On("GetRepo", repo.ID).Return(repo, nil)
	_store.On("GetUser", user.ID).Return(user, nil)
	_store.On("UserForgeGet", user.ID, repo.ForgeID).Return(nil, nil)
	_store.On("UpdateRepo", repo).Return(nil)
	_store.On("CreatePipeline", mock.Anything).Return(nil)
	_store.On("UpdatePipeline", mock.Anything).Return(nil)
	_manager.On("ConfigServiceFromRepo", repo).Return(_configService)
	_configService.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("connection timeout fetching config"))
	_forge.On("Status", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	api.PostHook(c)

	assert.Equal(t, http.StatusOK, c.Writer.Status(),
		"hook handler must not crash and must report success when forge config fetch fails")
}

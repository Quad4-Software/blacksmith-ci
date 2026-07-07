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

package api

import (
	"context"
	"fmt"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
)

type GetIntegrationListInput struct {
	PaginationParams
}

type GetIntegrationListOutput struct {
	Body []*model.Integration
}

func GetIntegrationList(ctx context.Context, input *GetIntegrationListInput) (*GetIntegrationListOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	integrationService := server.Config.Services.IntegrationService

	opts := &model.ListOptions{Page: input.Page, PerPage: input.PerPage}

	// Instance admins can see every integration on the server, regardless of
	// owner. Regular users only see the integrations they own.
	var list []*model.Integration
	var err error
	if user.Admin {
		list, err = integrationService.IntegrationListAll(opts)
	} else {
		list, err = integrationService.IntegrationList(user.ID, opts)
	}
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting integration list. %s", err))
	}

	// Return integrations with redacted config
	result := make([]*model.Integration, len(list))
	for i, integration := range list {
		result[i] = integration.CopyWithRedactedConfig()
	}

	return &GetIntegrationListOutput{Body: result}, nil
}

type GetAccessibleIntegrationsInput struct {
	PaginationParams
}

type GetAccessibleIntegrationsOutput struct {
	Body []*model.Integration
}

func GetAccessibleIntegrations(ctx context.Context, input *GetAccessibleIntegrationsInput) (*GetAccessibleIntegrationsOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)
	integrationService := server.Config.Services.IntegrationService

	// Get user's org IDs
	orgIDs, err := getUserOrgIDs(c, user)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting user organizations. %s", err))
	}

	// Get user's repo IDs
	repoIDs, err := getUserRepoIDs(c, user)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting user repositories. %s", err))
	}

	list, err := integrationService.IntegrationListAccessible(user.ID, orgIDs, repoIDs, &model.ListOptions{Page: input.Page, PerPage: input.PerPage})
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Error getting accessible integrations. %s", err))
	}

	// Return integrations with redacted config
	result := make([]*model.Integration, len(list))
	for i, integration := range list {
		result[i] = integration.CopyWithRedactedConfig()
	}

	return &GetAccessibleIntegrationsOutput{Body: result}, nil
}

type GetIntegrationInput struct {
	IntegrationID int64 `path:"integration_id" doc:"the integration's id"`
}

type GetIntegrationOutput struct {
	Body *model.Integration
}

func GetIntegration(ctx context.Context, input *GetIntegrationInput) (*GetIntegrationOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)

	integrationService := server.Config.Services.IntegrationService
	integration, err := integrationService.IntegrationFind(user.ID, input.IntegrationID)
	if err != nil {
		return nil, dbErr(err)
	}

	return &GetIntegrationOutput{Body: integration.CopyWithRedactedConfig()}, nil
}

type PostIntegrationBody struct {
	Name           string                `json:"name" minLength:"1" doc:"integration name"`
	Type           model.IntegrationType `json:"type" doc:"integration type"`
	Description    string                `json:"description,omitempty" doc:"integration description"`
	Config         string                `json:"config,omitempty" doc:"integration configuration"`
	AllowedUserIDs []int64               `json:"allowed_user_ids,omitempty" doc:"allowed user IDs"`
	AllowedOrgIDs  []int64               `json:"allowed_org_ids,omitempty" doc:"allowed org IDs"`
	AllowedRepoIDs []int64               `json:"allowed_repo_ids,omitempty" doc:"allowed repo IDs"`
}

type PatchIntegrationBody struct {
	Name           string  `json:"name,omitempty" doc:"integration name"`
	Description    string  `json:"description,omitempty" doc:"integration description"`
	Config         string  `json:"config,omitempty" doc:"integration configuration"`
	IsActive       bool    `json:"is_active" doc:"whether integration is active"`
	AllowedUserIDs []int64 `json:"allowed_user_ids,omitempty" doc:"allowed user IDs"`
	AllowedOrgIDs  []int64 `json:"allowed_org_ids,omitempty" doc:"allowed org IDs"`
	AllowedRepoIDs []int64 `json:"allowed_repo_ids,omitempty" doc:"allowed repo IDs"`
}

type PostIntegrationInput struct {
	Body PostIntegrationBody
}

type PostIntegrationOutput struct {
	Body *model.Integration
}

func PostIntegration(ctx context.Context, input *PostIntegrationInput) (*PostIntegrationOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)

	integration := &model.Integration{
		UserID:         user.ID,
		Name:           input.Body.Name,
		Type:           input.Body.Type,
		Description:    input.Body.Description,
		Config:         input.Body.Config,
		AllowedUserIDs: input.Body.AllowedUserIDs,
		AllowedOrgIDs:  input.Body.AllowedOrgIDs,
		AllowedRepoIDs: input.Body.AllowedRepoIDs,
		IsActive:       true,
	}

	integrationService := server.Config.Services.IntegrationService
	if err := integrationService.IntegrationCreate(integration); err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Error creating integration. %s", err))
	}

	return &PostIntegrationOutput{Body: integration.CopyWithRedactedConfig()}, nil
}

type PatchIntegrationInput struct {
	IntegrationID int64 `path:"integration_id" doc:"the integration's id"`
	Body          PatchIntegrationBody
}

type PatchIntegrationOutput struct {
	Body *model.Integration
}

func PatchIntegration(ctx context.Context, input *PatchIntegrationInput) (*PatchIntegrationOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)

	integrationService := server.Config.Services.IntegrationService
	integration, err := integrationService.IntegrationFind(user.ID, input.IntegrationID)
	if err != nil {
		return nil, dbErr(err)
	}

	// Update fields
	if input.Body.Name != "" {
		integration.Name = input.Body.Name
	}
	if input.Body.Description != "" {
		integration.Description = input.Body.Description
	}
	if input.Body.Config != "" {
		// Merge config, preserving sensitive fields that weren't provided
		log.Debug().
			Int64("integration_id", integration.ID).
			Str("old_config_length", strconv.Itoa(len(integration.Config))).
			Str("new_config_length", strconv.Itoa(len(input.Body.Config))).
			Msg("Merging integration config")

		mergedConfig, err := mergeIntegrationConfig(integration.Type, integration.Config, input.Body.Config)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Error merging config. %s", err))
		}

		log.Debug().
			Int64("integration_id", integration.ID).
			Str("merged_config_length", strconv.Itoa(len(mergedConfig))).
			Msg("Config merged successfully")

		integration.Config = mergedConfig
	} else {
		log.Debug().
			Int64("integration_id", integration.ID).
			Msg("No config in request, preserving existing config")
	}
	log.Debug().
		Int64("integration_id", integration.ID).
		Interface("old_allowed_user_ids", integration.AllowedUserIDs).
		Interface("new_allowed_user_ids", input.Body.AllowedUserIDs).
		Bool("new_is_nil", input.Body.AllowedUserIDs == nil).
		Msg("Updating allowlists")

	if input.Body.AllowedUserIDs != nil {
		integration.AllowedUserIDs = input.Body.AllowedUserIDs
	}
	if input.Body.AllowedOrgIDs != nil {
		integration.AllowedOrgIDs = input.Body.AllowedOrgIDs
	}
	if input.Body.AllowedRepoIDs != nil {
		integration.AllowedRepoIDs = input.Body.AllowedRepoIDs
	}

	log.Debug().
		Int64("integration_id", integration.ID).
		Interface("final_allowed_user_ids", integration.AllowedUserIDs).
		Interface("final_allowed_org_ids", integration.AllowedOrgIDs).
		Interface("final_allowed_repo_ids", integration.AllowedRepoIDs).
		Msg("Allowlists after update")
	// IsActive can be explicitly set to false
	integration.IsActive = input.Body.IsActive

	if err := integrationService.IntegrationUpdate(integration); err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Error updating integration. %s", err))
	}

	return &PatchIntegrationOutput{Body: integration.CopyWithRedactedConfig()}, nil
}

type DeleteIntegrationInput struct {
	IntegrationID int64 `path:"integration_id" doc:"the integration's id"`
}

func DeleteIntegration(ctx context.Context, input *DeleteIntegrationInput) (*struct{}, error) {
	c := ginCtx(ctx)
	user := session.User(c)

	integrationService := server.Config.Services.IntegrationService
	if err := integrationService.IntegrationDelete(user.ID, input.IntegrationID); err != nil {
		return nil, dbErr(err)
	}

	return nil, nil
}

type TestIntegrationInput struct {
	IntegrationID int64 `path:"integration_id" doc:"the integration's id"`
}

type TestIntegrationResponse struct {
	Status string `json:"status"`
}

type TestIntegrationOutput struct {
	Body TestIntegrationResponse
}

func TestIntegration(ctx context.Context, input *TestIntegrationInput) (*TestIntegrationOutput, error) {
	c := ginCtx(ctx)
	user := session.User(c)

	integrationService := server.Config.Services.IntegrationService
	integration, err := integrationService.IntegrationFind(user.ID, input.IntegrationID)
	if err != nil {
		return nil, dbErr(err)
	}

	if err := integrationService.TestConnection(integration); err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("Connection test failed: %s", err))
	}

	return &TestIntegrationOutput{Body: TestIntegrationResponse{Status: "connected"}}, nil
}

type GetIntegrationTypesInput struct{}

type IntegrationTypeInfo struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type GetIntegrationTypesOutput struct {
	Body []IntegrationTypeInfo
}

func GetIntegrationTypes(ctx context.Context, _ *GetIntegrationTypesInput) (*GetIntegrationTypesOutput, error) {
	types := []IntegrationTypeInfo{
		{Type: string(model.IntegrationTypeVault), Name: "HashiCorp Vault / OpenBao"},
		// Future types will be added here
	}
	return &GetIntegrationTypesOutput{Body: types}, nil
}

// getUserOrgIDs returns the IDs of organizations the user belongs to.
func getUserOrgIDs(c *gin.Context, user *model.User) ([]int64, error) {
	_forge, err := server.Config.Services.Manager.ForgeFromUser(user)
	if err != nil {
		return nil, err
	}

	_store := server.Config.Services.Manager.Store()

	// Build a user with the latest forge credentials if available,
	// otherwise the session user may carry a stale token from the
	// deprecated users.access_token field.
	forgeUser := user
	if uf, ufErr := _store.UserForgeGet(user.ID, user.ForgeID); ufErr == nil && uf != nil {
		forgeUserCopy := *user
		forgeUserCopy.AccessToken = uf.AccessToken
		forgeUserCopy.RefreshToken = uf.RefreshToken
		forgeUserCopy.Expiry = uf.Expiry
		forgeUser = &forgeUserCopy
	}

	teams, err := _forge.Teams(c, forgeUser)
	if err != nil {
		return nil, err
	}

	// Get org IDs from our database by matching org names
	orgIDs := make([]int64, 0, len(teams))
	for _, team := range teams {
		dbOrg, err := _store.OrgFindByName(team.Login, user.ForgeID)
		if err == nil && dbOrg != nil && !dbOrg.IsUser {
			orgIDs = append(orgIDs, dbOrg.ID)
		}
	}

	return orgIDs, nil
}

// getUserRepoIDs returns the IDs of repositories the user has access to.
func getUserRepoIDs(_ *gin.Context, user *model.User) ([]int64, error) {
	_store := server.Config.Services.Manager.Store()

	repos, err := _store.RepoList(user, false, false)
	if err != nil {
		return nil, err
	}

	repoIDs := make([]int64, len(repos))
	for i, repo := range repos {
		repoIDs[i] = repo.ID
	}

	return repoIDs, nil
}

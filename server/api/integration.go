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
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
)

// GetIntegrationList
//
//	@Summary	List user's integrations
//	@Router		/user/integrations [get]
//	@Produce	json
//	@Success	200	{array}	Integration
//	@Tags		Integrations
//	@Param		Authorization	header	string	true	"Insert your personal access token"				default(Bearer <personal access token>)
//	@Param		page			query	int		false	"for response pagination, page offset number"	default(1)
//	@Param		perPage			query	int		false	"for response pagination, max items per page"	default(50)
func GetIntegrationList(c *gin.Context) {
	user := session.User(c)
	integrationService := server.Config.Services.IntegrationService

	list, err := integrationService.IntegrationList(user.ID, session.Pagination(c))
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting integration list. %s", err)
		return
	}

	// Return integrations with redacted config
	result := make([]*model.Integration, len(list))
	for i, integration := range list {
		result[i] = integration.CopyWithRedactedConfig()
	}

	c.JSON(http.StatusOK, result)
}

// GetAccessibleIntegrations
//
//	@Summary		List integrations accessible to the user
//	@Description	Returns integrations owned by the user plus those shared via allowlists
//	@Router			/user/integrations/accessible [get]
//	@Produce		json
//	@Success		200	{array}	Integration
//	@Tags			Integrations
//	@Param			Authorization	header	string	true	"Insert your personal access token"				default(Bearer <personal access token>)
//	@Param			page			query	int		false	"for response pagination, page offset number"	default(1)
//	@Param			perPage			query	int		false	"for response pagination, max items per page"	default(50)
func GetAccessibleIntegrations(c *gin.Context) {
	user := session.User(c)
	integrationService := server.Config.Services.IntegrationService

	// Get user's org IDs
	orgIDs, err := getUserOrgIDs(c, user)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting user organizations. %s", err)
		return
	}

	// Get user's repo IDs
	repoIDs, err := getUserRepoIDs(c, user)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting user repositories. %s", err)
		return
	}

	list, err := integrationService.IntegrationListAccessible(user.ID, orgIDs, repoIDs, session.Pagination(c))
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting accessible integrations. %s", err)
		return
	}

	// Return integrations with redacted config
	result := make([]*model.Integration, len(list))
	for i, integration := range list {
		result[i] = integration.CopyWithRedactedConfig()
	}

	c.JSON(http.StatusOK, result)
}

// GetIntegration
//
//	@Summary	Get an integration by ID
//	@Router		/user/integrations/{integration_id} [get]
//	@Produce	json
//	@Success	200	{object}	Integration
//	@Tags		Integrations
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		integration_id	path	int		true	"the integration's id"
func GetIntegration(c *gin.Context) {
	user := session.User(c)
	integrationID, err := strconv.ParseInt(c.Param("integration_id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid integration ID")
		return
	}

	integrationService := server.Config.Services.IntegrationService
	integration, err := integrationService.IntegrationFind(user.ID, integrationID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	c.JSON(http.StatusOK, integration.CopyWithRedactedConfig())
}

// PostIntegration
//
//	@Summary	Create an integration
//	@Router		/user/integrations [post]
//	@Produce	json
//	@Success	200	{object}	Integration
//	@Tags		Integrations
//	@Param		Authorization	header	string		true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		integration		body	Integration	true	"the integration object data"
func PostIntegration(c *gin.Context) {
	user := session.User(c)

	in := new(model.Integration)
	if err := c.Bind(in); err != nil {
		c.String(http.StatusBadRequest, "Error parsing integration. %s", err)
		return
	}

	integration := &model.Integration{
		UserID:         user.ID,
		Name:           in.Name,
		Type:           in.Type,
		Description:    in.Description,
		Config:         in.Config,
		AllowedUserIDs: in.AllowedUserIDs,
		AllowedOrgIDs:  in.AllowedOrgIDs,
		AllowedRepoIDs: in.AllowedRepoIDs,
		IsActive:       true,
	}

	integrationService := server.Config.Services.IntegrationService
	if err := integrationService.IntegrationCreate(integration); err != nil {
		c.String(http.StatusBadRequest, "Error creating integration. %s", err)
		return
	}

	c.JSON(http.StatusOK, integration.CopyWithRedactedConfig())
}

// PatchIntegration
//
//	@Summary	Update an integration
//	@Router		/user/integrations/{integration_id} [patch]
//	@Produce	json
//	@Success	200	{object}	Integration
//	@Tags		Integrations
//	@Param		Authorization	header	string		true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		integration_id	path	int			true	"the integration's id"
//	@Param		integration		body	Integration	true	"the integration's data"
func PatchIntegration(c *gin.Context) {
	user := session.User(c)
	integrationID, err := strconv.ParseInt(c.Param("integration_id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid integration ID")
		return
	}

	in := new(model.Integration)
	if err := c.Bind(in); err != nil {
		c.String(http.StatusBadRequest, "Error parsing integration. %s", err)
		return
	}

	integrationService := server.Config.Services.IntegrationService
	integration, err := integrationService.IntegrationFind(user.ID, integrationID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	// Update fields
	if in.Name != "" {
		integration.Name = in.Name
	}
	if in.Description != "" {
		integration.Description = in.Description
	}
	if in.Config != "" {
		// Merge config, preserving sensitive fields that weren't provided
		log.Debug().
			Int64("integration_id", integration.ID).
			Str("old_config_length", strconv.Itoa(len(integration.Config))).
			Str("new_config_length", strconv.Itoa(len(in.Config))).
			Msg("Merging integration config")

		mergedConfig, err := mergeIntegrationConfig(integration.Type, integration.Config, in.Config)
		if err != nil {
			c.String(http.StatusBadRequest, "Error merging config. %s", err)
			return
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
		Interface("new_allowed_user_ids", in.AllowedUserIDs).
		Bool("new_is_nil", in.AllowedUserIDs == nil).
		Msg("Updating allowlists")

	if in.AllowedUserIDs != nil {
		integration.AllowedUserIDs = in.AllowedUserIDs
	}
	if in.AllowedOrgIDs != nil {
		integration.AllowedOrgIDs = in.AllowedOrgIDs
	}
	if in.AllowedRepoIDs != nil {
		integration.AllowedRepoIDs = in.AllowedRepoIDs
	}

	log.Debug().
		Int64("integration_id", integration.ID).
		Interface("final_allowed_user_ids", integration.AllowedUserIDs).
		Interface("final_allowed_org_ids", integration.AllowedOrgIDs).
		Interface("final_allowed_repo_ids", integration.AllowedRepoIDs).
		Msg("Allowlists after update")
	// IsActive can be explicitly set to false
	integration.IsActive = in.IsActive

	if err := integrationService.IntegrationUpdate(integration); err != nil {
		c.String(http.StatusBadRequest, "Error updating integration. %s", err)
		return
	}

	c.JSON(http.StatusOK, integration.CopyWithRedactedConfig())
}

// DeleteIntegration
//
//	@Summary	Delete an integration
//	@Router		/user/integrations/{integration_id} [delete]
//	@Produce	plain
//	@Success	204
//	@Tags		Integrations
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		integration_id	path	int		true	"the integration's id"
func DeleteIntegration(c *gin.Context) {
	user := session.User(c)
	integrationID, err := strconv.ParseInt(c.Param("integration_id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid integration ID")
		return
	}

	integrationService := server.Config.Services.IntegrationService
	if err := integrationService.IntegrationDelete(user.ID, integrationID); err != nil {
		handleDBError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// TestIntegration
//
//	@Summary		Test integration connection
//	@Description	Tests the connection to the configured secret store
//	@Router			/user/integrations/{integration_id}/test [post]
//	@Produce		json
//	@Success		200	{object}	object{status=string}
//	@Failure		400	{string}	string	"Connection test failed"
//	@Tags			Integrations
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param			integration_id	path	int		true	"the integration's id"
func TestIntegration(c *gin.Context) {
	user := session.User(c)
	integrationID, err := strconv.ParseInt(c.Param("integration_id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid integration ID")
		return
	}

	integrationService := server.Config.Services.IntegrationService
	integration, err := integrationService.IntegrationFind(user.ID, integrationID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	if err := integrationService.TestConnection(integration); err != nil {
		c.String(http.StatusBadRequest, "Connection test failed: %s", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "connected"})
}

// GetIntegrationTypes
//
//	@Summary	Get supported integration types
//	@Router		/integrations/types [get]
//	@Produce	json
//	@Success	200	{array}	object{type=string,name=string}
//	@Tags		Integrations
func GetIntegrationTypes(c *gin.Context) {
	types := []gin.H{
		{"type": model.IntegrationTypeVault, "name": "HashiCorp Vault / OpenBao"},
		// Future types will be added here
	}
	c.JSON(http.StatusOK, types)
}

// getUserOrgIDs returns the IDs of organizations the user belongs to.
func getUserOrgIDs(c *gin.Context, user *model.User) ([]int64, error) {
	_forge, err := server.Config.Services.Manager.ForgeFromUser(user)
	if err != nil {
		return nil, err
	}

	teams, err := _forge.Teams(c, user)
	if err != nil {
		return nil, err
	}

	// Get org IDs from our database by matching org names
	_store := server.Config.Services.Manager.Store()
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

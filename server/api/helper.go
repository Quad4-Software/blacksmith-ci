// Copyright 2022 Woodpecker Authors
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
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/forge"
	forge_types "codefloe.com/crowci/crow/v5/server/forge/types"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/pipeline"
	"codefloe.com/crowci/crow/v5/server/store"
	"codefloe.com/crowci/crow/v5/server/store/types"
)

func handlePipelineErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, &pipeline.ErrNotFound{}):
		c.String(http.StatusNotFound, "%s", err)
	case errors.Is(err, &pipeline.ErrBadRequest{}):
		c.String(http.StatusBadRequest, "%s", err)
	case errors.Is(err, pipeline.ErrFiltered):
		// for debugging purpose we add a header
		c.Writer.Header().Add("Pipeline-Filtered", "true")
		c.Status(http.StatusNoContent)
	default:
		c.String(http.StatusInternalServerError, "%s", err)
	}
}

func handleDBError(c *gin.Context, err error) {
	if errors.Is(err, types.RecordNotExist) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	_ = c.AbortWithError(http.StatusInternalServerError, err)
}

// handleForgeError handles errors returned from forge operations.
// If the error is a token refresh failure, it returns 401 to trigger re-authentication.
// Otherwise, it returns 500.
func handleForgeError(c *gin.Context, err error) {
	if errors.Is(err, &forge_types.ErrTokenRefreshFailed{}) {
		log.Error().Err(err).Msg("Token refresh failed")
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusUnauthorized, "Your authorization has expired and couldn't be refreshed automatically. Please log in again")
		c.Abort()
		return
	}
	_ = c.AbortWithError(http.StatusInternalServerError, err)
}

// refreshUserTokenForForge refreshes the user's token for a specific forge.
// If isSecondaryForge is true, it uses RefreshUserForge for proper secondary forge handling.
func refreshUserTokenForForge(c *gin.Context, user *model.User, forgeID int64, isSecondaryForge bool) {
	_store := store.FromContext(c)
	_forge, err := server.Config.Services.Manager.ForgeFromUser(user)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from user")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var refreshErr error
	if isSecondaryForge && forgeID > 0 {
		refreshErr = forge.RefreshUserForge(c, _forge, _store, user, forgeID)
	} else {
		refreshErr = forge.Refresh(c, _forge, _store, user)
	}

	if refreshErr != nil {
		log.Error().Err(refreshErr).Msg("Failed to refresh user auth token")
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusUnauthorized, "Your authorization has expired and couldn't be refreshed automatically. Please log in again")
		c.Abort()
		return
	}
}

// pipelineDeleteAllowed checks if the given pipeline can be deleted based on its status.
// It returns a bool indicating if delete is allowed, and the pipeline's status.
func pipelineDeleteAllowed(pl *model.Pipeline) bool {
	switch pl.Status {
	case model.StatusRunning, model.StatusPending, model.StatusBlocked:
		return false
	}

	return true
}

// mergeIntegrationConfig merges new config with old config, preserving sensitive
// fields that were redacted in the API response.
func mergeIntegrationConfig(integrationType model.IntegrationType, oldConfig, newConfig string) (string, error) {
	switch integrationType {
	case model.IntegrationTypeVault:
		return mergeVaultConfig(oldConfig, newConfig)
	default:
		return newConfig, nil
	}
}

// mergeVaultConfig preserves sensitive fields from old config when they appear redacted in new config.
func mergeVaultConfig(oldConfig, newConfig string) (string, error) {
	var oldCfg, newCfg model.VaultConfig

	if err := json.Unmarshal([]byte(oldConfig), &oldCfg); err != nil {
		return newConfig, nil // If old config is invalid, just use new config
	}

	if err := json.Unmarshal([]byte(newConfig), &newCfg); err != nil {
		return "", err
	}

	// Preserve secret_id if the new value is the redacted placeholder
	if newCfg.SecretID == "***" || newCfg.SecretID == "" {
		newCfg.SecretID = oldCfg.SecretID
	}

	// Preserve role_id if it appears to be abbreviated (contains "...")
	if strings.Contains(newCfg.RoleID, "...") || newCfg.RoleID == "" {
		newCfg.RoleID = oldCfg.RoleID
	}

	// Preserve TLS sensitive fields if redacted
	if newCfg.TLSConfig != nil && oldCfg.TLSConfig != nil {
		if newCfg.TLSConfig.ClientKey == "***" || newCfg.TLSConfig.ClientKey == "" {
			newCfg.TLSConfig.ClientKey = oldCfg.TLSConfig.ClientKey
		}
		if newCfg.TLSConfig.CACert == "[certificate present]" {
			newCfg.TLSConfig.CACert = oldCfg.TLSConfig.CACert
		}
		if newCfg.TLSConfig.ClientCert == "[certificate present]" {
			newCfg.TLSConfig.ClientCert = oldCfg.TLSConfig.ClientCert
		}
	}

	merged, err := json.Marshal(newCfg)
	if err != nil {
		return "", err
	}

	return string(merged), nil
}

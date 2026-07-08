// Copyright 2018 Drone.IO Inc.
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

package token

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/forge"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
)

func Refresh(c *gin.Context) {
	user := session.User(c)
	if user != nil {
		_store := store.FromContext(c)

		// Refresh tokens for all linked forges, not just the user's primary forge
		userForges, err := _store.UserForgeList(user.ID)
		if err != nil {
			log.Error().Err(err).Msgf("failed to get user forges for user '%s'", user.Login)
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		allFailed := len(userForges) > 0
		for _, uf := range userForges {
			_forge, err := server.Config.Services.Manager.ForgeByID(uf.ForgeID)
			if err != nil {
				log.Error().Err(err).Int64("forge_id", uf.ForgeID).Msgf("cannot get forge by id for token refresh")
				continue
			}

			// Build a user with this forge's credentials for the refresh call
			forgeUser := *user
			forgeUser.ForgeID = uf.ForgeID
			forgeUser.AccessToken = uf.AccessToken
			forgeUser.RefreshToken = uf.RefreshToken
			forgeUser.Expiry = uf.Expiry

			if err := forge.RefreshUserForge(c, _forge, _store, &forgeUser, uf.ForgeID); err != nil {
				log.Error().Err(err).Int64("forge_id", uf.ForgeID).Msgf("failed to refresh OAuth token for user '%s' on forge %d", user.Login, uf.ForgeID)
			} else {
				allFailed = false
			}
		}

		if allFailed {
			log.Error().Msgf("failed to refresh OAuth tokens for user '%s' on all forges", user.Login)
			c.Header("Content-Type", "application/json; charset=utf-8")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Your authorization has expired and could not be refreshed. Please log in again.",
				"code":  "TOKEN_REFRESH_FAILED",
			})
			c.Abort()
			return
		}
	}

	c.Next()
}

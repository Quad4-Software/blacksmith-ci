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

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/forge"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/store"
)

func Refresh(c *gin.Context) {
	user := session.User(c)
	if user != nil {
		_forge, err := server.Config.Services.Manager.ForgeFromUser(user)
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if err := forge.Refresh(c, _forge, store.FromContext(c), user); err != nil {
			log.Error().Err(err).Msgf("failed to refresh OAuth token for user '%s'", user.Login)

			// Always return JSON for token refresh errors to ensure proper frontend handling
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

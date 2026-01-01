// Copyright 2025 Crow CI Authors
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

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v5/server"
	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/router/middleware/session"
	"codeberg.org/crowci/crow/v5/server/store"
)

// GetUserOrgList
//
//	@Summary		List organizations the current user is a member of
//	@Description	Returns all organizations where the authenticated user has membership.
//	@Router			/user/orgs [get]
//	@Produce		json
//	@Success		200	{array}	Org
//	@Tags			User
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
func GetUserOrgList(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)

	_forge, err := server.Config.Services.Manager.ForgeFromUser(user)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get forge from user")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Fetch teams from the forge
	teams, err := _forge.Teams(c, user)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get user teams")
		c.String(http.StatusInternalServerError, "Error fetching teams. %s", err)
		return
	}

	log.Debug().Int("team_count", len(teams)).Msg("Fetched teams from forge for org list")

	// Map teams to orgs that exist in Crow
	orgs := make([]*model.Org, 0, len(teams))
	for _, team := range teams {
		org, err := _store.OrgFindByName(team.Login, user.ForgeID)
		if err != nil {
			log.Debug().Err(err).Str("org", team.Login).Msg("Org not found in Crow database, skipping")
			continue
		}
		// Skip user's personal org (IsUser=true)
		if org.IsUser {
			log.Debug().Str("org_name", org.Name).Msg("Skipping personal org")
			continue
		}
		log.Debug().Int64("org_id", org.ID).Str("org_name", org.Name).Msg("Added org to list")
		orgs = append(orgs, org)
	}

	log.Debug().Int("org_count", len(orgs)).Msg("Returning user orgs")

	c.JSON(http.StatusOK, orgs)
}

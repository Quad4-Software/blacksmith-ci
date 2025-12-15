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
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v5/server"
	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/router/middleware/session"
	"codeberg.org/crowci/crow/v5/server/store"
)

// GetUserRegistryList
//
//	@Summary	List all registries accessible by the current user
//	@Router		/user/registries [get]
//	@Produce	json
//	@Success	200	{array}	RegistryWithOrg
//	@Tags		User
//	@Param		Authorization	header	string	true	"Insert your personal access token"			default(Bearer <personal access token>)
//	@Param		scope			query	string	false	"Comma-separated scopes: user,org,global"	default("user,org,global")
//	@Param		search			query	string	false	"Search term for registry address"
//	@Param		page			query	int		false	"for response pagination, page offset number"	default(1)
//	@Param		perPage			query	int		false	"for response pagination, max items per page"	default(50)
func GetUserRegistryList(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)

	// Parse scope filter (default: all scopes)
	scopeParam := c.DefaultQuery("scope", "user,org,global")
	scopes := strings.Split(scopeParam, ",")
	includeUser := contains(scopes, "user")
	includeOrg := contains(scopes, "org")
	includeGlobal := contains(scopes, "global")

	searchTerm := c.Query("search")

	// Build org ID list
	orgIDs := make([]int64, 0)

	// Include user's personal org
	if includeUser {
		orgIDs = append(orgIDs, user.OrgID)
	}

	// Include member orgs
	if includeOrg {
		_forge, err := server.Config.Services.Manager.ForgeFromUser(user)
		if err != nil {
			log.Error().Err(err).Msg("Cannot get forge from user")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		teams, err := _forge.Teams(c, user)
		if err != nil {
			log.Error().Err(err).Msg("Cannot get user teams")
			c.String(http.StatusInternalServerError, "Error fetching teams. %s", err)
			return
		}

		// Map teams to org IDs
		for _, team := range teams {
			org, err := _store.OrgFindByName(team.Login, user.ForgeID)
			if err != nil {
				log.Debug().Err(err).Str("org", team.Login).Msg("Org not found, skipping")
				continue
			}
			// Skip if it's the user's personal org (already added)
			if org.ID != user.OrgID {
				orgIDs = append(orgIDs, org.ID)
			}
		}
	}

	// Fetch registries from datastore
	registries, err := _store.UserRegistryList(orgIDs, includeGlobal, searchTerm, session.Pagination(c))
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting registry list. %s", err)
		return
	}

	// Enrich with org names and scope types
	orgCache := make(map[int64]string)
	enrichedRegistries := make([]*model.RegistryWithOrg, 0, len(registries))

	for _, registry := range registries {
		orgName := "Global"
		isPersonalOrg := false

		if registry.OrgID > 0 {
			// Check if cached
			if name, cached := orgCache[registry.OrgID]; cached {
				orgName = name
			} else {
				org, err := _store.OrgGet(registry.OrgID)
				if err == nil {
					orgName = org.Name
					orgCache[registry.OrgID] = orgName
				}
			}

			// Check if this is the user's personal org
			isPersonalOrg = registry.OrgID == user.OrgID
		}

		enriched := registry.WithOrg(orgName, isPersonalOrg)
		enrichedRegistries = append(enrichedRegistries, enriched)
	}

	// Copy to remove sensitive fields
	result := make([]*model.RegistryWithOrg, len(enrichedRegistries))
	for i, r := range enrichedRegistries {
		copied := r.Copy()
		result[i] = &model.RegistryWithOrg{
			Registry:  copied,
			OrgName:   r.OrgName,
			ScopeType: r.ScopeType,
		}
	}

	c.JSON(http.StatusOK, result)
}

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

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/store"
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

	// Enrich with org names, scope types, and forge info
	type orgInfo struct {
		name    string
		forgeID int64
	}
	type forgeInfo struct {
		name  string
		icon  string
		ftype string
	}
	orgCache := make(map[int64]orgInfo)
	forgeCache := make(map[int64]forgeInfo)
	enrichedRegistries := make([]*model.RegistryWithOrg, 0, len(registries))

	// Helper function to get forge info
	getForgeInfo := func(forgeID int64) (string, string, string) {
		if forgeID == 0 {
			return "", "", ""
		}
		if info, cached := forgeCache[forgeID]; cached {
			return info.name, info.icon, info.ftype
		}
		forge, err := _store.ForgeGet(forgeID)
		if err == nil && forge != nil {
			// Extract hostname from forge URL
			forgeName := forge.URL
			if len(forgeName) > 0 {
				// Try to extract just the hostname
				if idx := len("https://"); len(forgeName) > idx && forgeName[:idx] == "https://" {
					forgeName = forgeName[idx:]
				} else if idx := len("http://"); len(forgeName) > idx && forgeName[:idx] == "http://" {
					forgeName = forgeName[idx:]
				}
				// Remove trailing slash
				if len(forgeName) > 0 && forgeName[len(forgeName)-1] == '/' {
					forgeName = forgeName[:len(forgeName)-1]
				}
			}
			forgeCache[forgeID] = forgeInfo{name: forgeName, icon: forge.Icon, ftype: string(forge.Type)}
			return forgeName, forge.Icon, string(forge.Type)
		}
		forgeCache[forgeID] = forgeInfo{}
		return "", "", ""
	}

	for _, registry := range registries {
		orgName := "Global"
		isPersonalOrg := false
		var forgeID int64

		if registry.OrgID > 0 {
			// Check if cached
			if info, cached := orgCache[registry.OrgID]; cached {
				orgName = info.name
				forgeID = info.forgeID
			} else {
				org, err := _store.OrgGet(registry.OrgID)
				if err == nil {
					orgName = org.Name
					forgeID = org.ForgeID
					orgCache[registry.OrgID] = orgInfo{name: orgName, forgeID: forgeID}
				}
			}

			// Check if this is the user's personal org
			isPersonalOrg = registry.OrgID == user.OrgID
		}

		forgeName, forgeIcon, forgeType := getForgeInfo(forgeID)
		enriched := registry.WithOrg(orgName, isPersonalOrg, forgeID, forgeName, forgeIcon, forgeType)
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
			ForgeID:   r.ForgeID,
			ForgeName: r.ForgeName,
			ForgeIcon: r.ForgeIcon,
			ForgeType: r.ForgeType,
		}
	}

	c.JSON(http.StatusOK, result)
}

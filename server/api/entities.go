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
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store"
)

// GetEntities
//
//	@Summary		List all entities (users and organizations)
//	@Description	Returns a combined list of users and organizations for admin management. Requires admin rights.
//	@Router			/entities [get]
//	@Produce		json
//	@Success		200	{array}	Entity
//	@Tags			Admin
//	@Param			Authorization	header	string	true	"Insert your personal access token"				default(Bearer <personal access token>)
//	@Param			page			query	int		false	"for response pagination, page offset number"	default(1)
//	@Param			perPage			query	int		false	"for response pagination, max items per page"	default(15)
//	@Param			entity_type		query	string	false	"filter by entity type (user, org, or comma-separated)"
//	@Param			is_admin		query	string	false	"filter by admin status (true, false, or empty for all)"
//	@Param			search			query	string	false	"search term for name/login/email"
func GetEntities(c *gin.Context) {
	_store := store.FromContext(c)

	// Parse query parameters
	entityTypesParam := c.Query("entity_type")
	isAdminParam := c.Query("is_admin")
	searchParam := strings.ToLower(strings.TrimSpace(c.Query("search")))

	// Fetch all users and orgs (we'll filter and paginate after merging)
	users, err := _store.GetUserList(&model.ListOptions{All: true})
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting user list: %s", err)
		return
	}

	orgs, err := _store.OrgList(&model.ListOptions{All: true})
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting org list: %s", err)
		return
	}

	// Convert to entities
	var entities []*model.Entity

	// Parse entity type filter
	entityTypes := make(map[model.EntityType]bool)
	if entityTypesParam != "" {
		for _, et := range strings.Split(entityTypesParam, ",") {
			et = strings.TrimSpace(et)
			switch et {
			case "user":
				entityTypes[model.EntityTypeUser] = true
			case "org":
				entityTypes[model.EntityTypeOrg] = true
			}
		}
	}

	// Parse admin filter
	var isAdminFilter *bool
	if isAdminParam != "" {
		if val, err := strconv.ParseBool(isAdminParam); err == nil {
			isAdminFilter = &val
		}
	}

	// Add users if not filtered out
	if len(entityTypes) == 0 || entityTypes[model.EntityTypeUser] {
		for _, user := range users {
			// Apply admin filter
			if isAdminFilter != nil && user.Admin != *isAdminFilter {
				continue
			}

			// Apply search filter
			if searchParam != "" {
				loginMatch := strings.Contains(strings.ToLower(user.Login), searchParam)
				emailMatch := strings.Contains(strings.ToLower(user.Email), searchParam)
				if !loginMatch && !emailMatch {
					continue
				}
			}

			entities = append(entities, model.UserToEntity(user))
		}
	}

	// Add orgs if not filtered out
	if len(entityTypes) == 0 || entityTypes[model.EntityTypeOrg] {
		for _, org := range orgs {
			// Admin filter doesn't apply to orgs
			// Apply search filter
			if searchParam != "" {
				nameMatch := strings.Contains(strings.ToLower(org.Name), searchParam)
				if !nameMatch {
					continue
				}
			}

			entities = append(entities, model.OrgToEntity(org))
		}
	}

	// Sort by ID (primary) and entity_type (secondary)
	sort.Slice(entities, func(i, j int) bool {
		if entities[i].ID != entities[j].ID {
			return entities[i].ID < entities[j].ID
		}
		return entities[i].EntityType < entities[j].EntityType
	})

	// Apply pagination
	page := 1
	perPage := 15
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if perPageStr := c.Query("perPage"); perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 50 {
			perPage = pp
		}
	}

	// Calculate pagination bounds
	totalEntities := len(entities)
	startIdx := (page - 1) * perPage
	endIdx := startIdx + perPage

	// Ensure bounds are valid
	if startIdx >= totalEntities {
		c.JSON(http.StatusOK, []*model.Entity{})
		return
	}
	if endIdx > totalEntities {
		endIdx = totalEntities
	}

	// Return paginated results
	paginatedEntities := entities[startIdx:endIdx]
	c.JSON(http.StatusOK, paginatedEntities)
}

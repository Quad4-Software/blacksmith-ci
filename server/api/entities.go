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
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store"
)

type GetEntitiesInput struct {
	Page       int    `query:"page" default:"1" minimum:"1" doc:"for response pagination, page offset number"`
	PerPage    int    `query:"perPage" default:"15" minimum:"1" maximum:"50" doc:"for response pagination, max items per page"`
	EntityType string `query:"entity_type" doc:"filter by entity type (user, org, or comma-separated)"`
	IsAdmin    string `query:"is_admin" doc:"filter by admin status (true, false, or empty for all)"`
	Search     string `query:"search" doc:"search term for name/login/email"`
}

type GetEntitiesOutput struct {
	Body []*model.Entity
}

func GetEntities(ctx context.Context, input *GetEntitiesInput) (*GetEntitiesOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)

	// Parse query parameters
	searchParam := strings.ToLower(strings.TrimSpace(input.Search))

	// Fetch all users and orgs (we'll filter and paginate after merging)
	users, err := _store.GetUserList(&model.ListOptions{All: true})
	if err != nil {
		return nil, huma.Error500InternalServerError("Error getting user list", err)
	}

	orgs, err := _store.OrgList(&model.ListOptions{All: true})
	if err != nil {
		return nil, huma.Error500InternalServerError("Error getting org list", err)
	}

	// Convert to entities
	var entities []*model.Entity

	// Parse entity type filter
	entityTypes := make(map[model.EntityType]bool)
	if input.EntityType != "" {
		for _, et := range strings.Split(input.EntityType, ",") {
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
	if input.IsAdmin != "" {
		if val, err := strconv.ParseBool(input.IsAdmin); err == nil {
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
	page := input.Page
	perPage := input.PerPage

	// Calculate pagination bounds
	totalEntities := len(entities)
	startIdx := (page - 1) * perPage
	endIdx := startIdx + perPage

	// Ensure bounds are valid
	if startIdx >= totalEntities {
		return &GetEntitiesOutput{Body: []*model.Entity{}}, nil
	}
	if endIdx > totalEntities {
		endIdx = totalEntities
	}

	// Return paginated results
	paginatedEntities := entities[startIdx:endIdx]
	return &GetEntitiesOutput{Body: paginatedEntities}, nil
}

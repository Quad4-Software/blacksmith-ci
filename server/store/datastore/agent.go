// Copyright 2021 Woodpecker Authors
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

package datastore

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"codefloe.com/crowci/crow/v5/server/model"
)

var ErrNoTokenProvided = errors.New("please provide a token")

const (
	// AgentHealthyThresholdSeconds defines how many seconds ago an agent must have contacted
	// the server to be considered "healthy" (green status).
	agentHealthyThresholdSeconds = 60
)

func (s storage) AgentList(opts *model.AgentListOptions) (agents []*model.Agent, _ error) {
	// If opts is nil, initialize with defaults
	if opts == nil {
		opts = &model.AgentListOptions{}
	}

	q := s.db.NewSelect().Model(&agents)

	// Apply platform filter
	if opts.Platform != "" {
		q = q.Where("platform = ?", opts.Platform)
	}

	// Apply backend filter
	if opts.Backend != "" {
		q = q.Where("backend = ?", opts.Backend)
	}

	// Apply search across multiple fields
	if opts.Search != "" {
		searchPattern := "%" + opts.Search + "%"
		q = q.Where(
			"name LIKE ? OR platform LIKE ? OR backend LIKE ? OR custom_labels LIKE ? OR "+s.castAsText("id")+" LIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern, searchPattern,
		)
	}

	// Apply sorting
	sortField := "id"
	sortOrder := "ASC"

	if opts.Sort != "" {
		switch opts.Sort {
		case "name", "platform", "backend", "capacity":
			sortField = opts.Sort
		case "last_contact":
			sortField = opts.Sort
		case "status":
			// Stable sort by health status:
			// 1. Check if agent contacted within last 60 seconds (healthy/green)
			// 2. Within each health group, sort by ID for stable ordering
			healthyThreshold := time.Now().Unix() - agentHealthyThresholdSeconds

			if opts.Order == "desc" {
				// Reverse: unhealthy first, then ID descending within each group
				q = q.OrderExpr("CASE WHEN last_contact IS NOT NULL AND last_contact > 0 AND last_contact > " +
					fmt.Sprint(healthyThreshold) + " THEN 1 ELSE 0 END DESC, id DESC")
			} else {
				// Default: healthy first, then by ID ascending
				q = q.OrderExpr("CASE WHEN last_contact IS NOT NULL AND last_contact > 0 AND last_contact > " +
					fmt.Sprint(healthyThreshold) + " THEN 0 ELSE 1 END ASC, id ASC")
			}
		}
	}

	if opts.Sort != "status" {
		if opts.Order == "desc" {
			sortOrder = "DESC"
		}
		q = q.Order(sortField + " " + sortOrder)
	}

	// Apply pagination
	q = s.paginate(q, &opts.ListOptions)

	return agents, q.Scan(context.Background())
}

func (s storage) AgentFind(id int64) (*model.Agent, error) {
	agent := new(model.Agent)
	return agent, wrapGet(s.db.NewSelect().Model(agent).Where("id = ?", id).Scan(context.Background()))
}

func (s storage) AgentFindByToken(token string) (*model.Agent, error) {
	// Searching with an empty token would result in an empty where clause and therefore returning first item
	if token == "" {
		return nil, ErrNoTokenProvided
	}
	agent := new(model.Agent)
	return agent, wrapGet(s.db.NewSelect().Model(agent).Where("token = ?", token).Scan(context.Background()))
}

func (s storage) AgentCreate(agent *model.Agent) error {
	// only Insert set auto created ID back to object
	_, err := s.db.NewInsert().Model(agent).Exec(context.Background())
	return err
}

func (s storage) AgentUpdate(agent *model.Agent) error {
	_, err := s.db.NewUpdate().Model(agent).WherePK().Exec(context.Background())
	return err
}

func (s storage) AgentDelete(agent *model.Agent) error {
	return wrapDelete(s.db.NewDelete().Model((*model.Agent)(nil)).Where("id = ?", agent.ID).Exec(context.Background()))
}

func (s storage) AgentListForOrg(orgID int64, p *model.ListOptions) ([]*model.Agent, error) {
	var all []*model.Agent
	if err := s.db.NewSelect().Model(&all).Order("id ASC").Scan(context.Background()); err != nil {
		return nil, err
	}

	filtered := make([]*model.Agent, 0, len(all))
	for _, agent := range all {
		if slices.Contains(agent.OrgIDs, orgID) {
			filtered = append(filtered, agent)
		}
	}

	if p != nil {
		filtered = model.ApplyPagination(p, filtered)
	}
	return filtered, nil
}

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
	"errors"
	"fmt"
	"time"

	"xorm.io/xorm"

	"codefloe.com/crowci/crow/v5/server/model"
)

var ErrNoTokenProvided = errors.New("please provide a token")

const (
	// AgentHealthyThresholdSeconds defines how many seconds ago an agent must have contacted
	// the server to be considered "healthy" (green status).
	agentHealthyThresholdSeconds = 60
)

func (s storage) AgentList(opts *model.AgentListOptions) (agents []*model.Agent, _ error) {
	sess := s.engine.NewSession()

	// If opts is nil, initialize with defaults
	if opts == nil {
		opts = &model.AgentListOptions{}
	}

	// Apply org filter if specified
	if opts.OrgID != nil {
		sess = sess.Where("org_id = ?", *opts.OrgID)
	}

	// Apply platform filter
	if opts.Platform != "" {
		sess = sess.Where("platform = ?", opts.Platform)
	}

	// Apply backend filter
	if opts.Backend != "" {
		sess = sess.Where("backend = ?", opts.Backend)
	}

	// Apply search across multiple fields
	if opts.Search != "" {
		searchPattern := "%" + opts.Search + "%"
		sess = sess.Where(
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
			// This prevents the list from constantly reordering as agents reconnect

			// Calculate the threshold timestamp in Go
			// This is database-agnostic since last_contact is stored as Unix timestamp (int64)
			healthyThreshold := time.Now().Unix() - agentHealthyThresholdSeconds

			// Use xorm's builder to create CASE expression with the pre-calculated threshold
			sortField = "CASE WHEN last_contact IS NOT NULL AND last_contact > 0 AND last_contact > " +
				fmt.Sprint(healthyThreshold) + " THEN 0 ELSE 1 END, id"

			// When sorting by status, healthy (0) come first by default
			sortOrder = "ASC" // 0 (healthy) before 1 (not healthy), then by ID ascending
			if opts.Order == "desc" {
				// Reverse: unhealthy first, then ID descending within each group
				sortField = "CASE WHEN last_contact IS NOT NULL AND last_contact > 0 AND last_contact > " +
					fmt.Sprint(healthyThreshold) + " THEN 1 ELSE 0 END DESC, id"
				sortOrder = "DESC"
			}
		}
	}

	if opts.Order == "desc" && opts.Sort != "status" {
		sortOrder = "DESC"
	}

	sess = sess.OrderBy(sortField + " " + sortOrder)

	// Apply pagination
	sess = s.paginateSession(sess, &opts.ListOptions)

	return agents, sess.Find(&agents)
}

// paginateSession applies pagination to an existing session.
func (s storage) paginateSession(sess *xorm.Session, p *model.ListOptions) *xorm.Session {
	if p.All {
		return sess
	}
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 {
		p.PerPage = 50
	}
	sess = sess.Limit(p.PerPage, p.PerPage*(p.Page-1))
	return sess
}

func (s storage) AgentFind(id int64) (*model.Agent, error) {
	agent := new(model.Agent)
	return agent, wrapGet(s.engine.ID(id).Get(agent))
}

func (s storage) AgentFindByToken(token string) (*model.Agent, error) {
	// Searching with an empty token would result in an empty where clause and therefore returning first item
	if token == "" {
		return nil, ErrNoTokenProvided
	}
	agent := new(model.Agent)
	return agent, wrapGet(s.engine.Where("token = ?", token).Get(agent))
}

func (s storage) AgentCreate(agent *model.Agent) error {
	// only Insert set auto created ID back to object
	_, err := s.engine.Insert(agent)
	return err
}

func (s storage) AgentUpdate(agent *model.Agent) error {
	_, err := s.engine.ID(agent.ID).AllCols().Update(agent)
	return err
}

func (s storage) AgentDelete(agent *model.Agent) error {
	return wrapDelete(s.engine.ID(agent.ID).Delete(new(model.Agent)))
}

func (s storage) AgentListForOrg(orgID int64, p *model.ListOptions) (agents []*model.Agent, _ error) {
	return agents, s.paginate(p).Where("org_id = ?", orgID).OrderBy("id").Find(&agents)
}

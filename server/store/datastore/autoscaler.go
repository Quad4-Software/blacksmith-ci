// Copyright 2025 Crow Authors
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
	"fmt"
	"time"

	"codefloe.com/crowci/crow/v5/server/model"
)

const (
	// AutoscalerHealthyThresholdSeconds defines how many seconds ago an autoscaler must have contacted
	// the server to be considered "healthy" (green status).
	autoscalerHealthyThresholdSeconds = 120
)

func (s storage) AutoscalerList(opts *model.AutoscalerListOptions) (autoscalers []*model.Autoscaler, _ error) {
	sess := s.engine.NewSession()

	// If opts is nil, initialize with defaults
	if opts == nil {
		opts = &model.AutoscalerListOptions{}
	}

	// Apply org filter if specified
	if opts.OrgID != nil {
		sess = sess.Where("org_id = ?", *opts.OrgID)
	}

	// Apply enabled filter
	if opts.Enabled != nil {
		sess = sess.Where("enabled = ?", *opts.Enabled)
	}

	// Apply provider filter
	if opts.Provider != "" {
		sess = sess.Where("provider = ?", opts.Provider)
	}

	// Apply search across multiple fields
	if opts.Search != "" {
		searchPattern := "%" + opts.Search + "%"
		sess = sess.Where(
			"name LIKE ? OR provider LIKE ? OR region LIKE ? OR "+s.castAsText("id")+" LIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern,
		)
	}

	// Apply sorting
	sortField := "id"
	sortOrder := "ASC"

	if opts.Sort != "" {
		switch opts.Sort {
		case "name", "provider", "last_contact":
			sortField = opts.Sort
		case "status":
			// Stable sort by health status:
			// 1. Check if autoscaler contacted within last 120 seconds (healthy/green)
			// 2. Within each health group, sort by ID for stable ordering
			// This prevents the list from constantly reordering as autoscalers reconnect

			// Calculate the threshold timestamp in Go
			// This is database-agnostic since last_contact is stored as Unix timestamp (int64)
			healthyThreshold := time.Now().Unix() - autoscalerHealthyThresholdSeconds

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

	return autoscalers, sess.Find(&autoscalers)
}

func (s storage) AutoscalerFind(id int64) (*model.Autoscaler, error) {
	autoscaler := new(model.Autoscaler)
	return autoscaler, wrapGet(s.engine.ID(id).Get(autoscaler))
}

func (s storage) AutoscalerFindByName(name string) (*model.Autoscaler, error) {
	if name == "" {
		return nil, ErrNoTokenProvided // Reuse existing error or create a new one
	}
	autoscaler := new(model.Autoscaler)
	return autoscaler, wrapGet(s.engine.Where("name = ?", name).Get(autoscaler))
}

func (s storage) AutoscalerFindByToken(token string) (*model.Autoscaler, error) {
	// Searching with an empty token would result in an empty where clause and therefore returning first item
	if token == "" {
		return nil, ErrNoTokenProvided
	}
	autoscaler := new(model.Autoscaler)
	return autoscaler, wrapGet(s.engine.Where("token = ?", token).Get(autoscaler))
}

func (s storage) AutoscalerCreate(autoscaler *model.Autoscaler) error {
	// only Insert set auto created ID back to object
	_, err := s.engine.Insert(autoscaler)
	return err
}

func (s storage) AutoscalerUpdate(autoscaler *model.Autoscaler) error {
	_, err := s.engine.ID(autoscaler.ID).AllCols().Update(autoscaler)
	return err
}

func (s storage) AutoscalerDelete(autoscaler *model.Autoscaler) error {
	return wrapDelete(s.engine.ID(autoscaler.ID).Delete(new(model.Autoscaler)))
}

func (s storage) AutoscalerUpdateHeartbeat(id int64, activeAgents, pendingAgents int32, version string) error {
	_, err := s.engine.ID(id).Cols("last_contact", "active_agents", "pending_agents", "version").Update(&model.Autoscaler{
		LastContact:   time.Now().Unix(),
		ActiveAgents:  activeAgents,
		PendingAgents: pendingAgents,
		Version:       version,
	})
	return err
}

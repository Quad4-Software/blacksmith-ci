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
	"context"
	"fmt"
	"time"

	"codefloe.com/crowci/crow/v6/server/model"
)

const (
	// AutoscalerHealthyThresholdSeconds defines how many seconds ago an autoscaler must have contacted
	// the server to be considered "healthy" (green status).
	autoscalerHealthyThresholdSeconds = 120
)

func (s storage) AutoscalerList(opts *model.AutoscalerListOptions) (autoscalers []*model.Autoscaler, _ error) {
	// If opts is nil, initialize with defaults
	if opts == nil {
		opts = &model.AutoscalerListOptions{}
	}

	q := s.db.NewSelect().Model(&autoscalers)

	// Apply org filter if specified
	if opts.OrgID != nil {
		q = q.Where("org_id = ?", *opts.OrgID)
	}

	// Apply enabled filter
	if opts.Enabled != nil {
		q = q.Where("enabled = ?", *opts.Enabled)
	}

	// Apply provider filter
	if opts.Provider != "" {
		q = q.Where("provider = ?", opts.Provider)
	}

	// Apply search across multiple fields
	if opts.Search != "" {
		searchPattern := "%" + opts.Search + "%"
		q = q.Where(
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
			healthyThreshold := time.Now().Unix() - autoscalerHealthyThresholdSeconds

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

	return autoscalers, q.Scan(context.Background())
}

func (s storage) AutoscalerFind(id int64) (*model.Autoscaler, error) {
	autoscaler := new(model.Autoscaler)
	return autoscaler, wrapGet(s.db.NewSelect().Model(autoscaler).Where("id = ?", id).Scan(context.Background()))
}

func (s storage) AutoscalerFindByName(name string) (*model.Autoscaler, error) {
	if name == "" {
		return nil, ErrNoTokenProvided // Reuse existing error or create a new one
	}
	autoscaler := new(model.Autoscaler)
	return autoscaler, wrapGet(s.db.NewSelect().Model(autoscaler).Where("name = ?", name).Scan(context.Background()))
}

func (s storage) AutoscalerFindByToken(token string) (*model.Autoscaler, error) {
	// Searching with an empty token would result in an empty where clause and therefore returning first item
	if token == "" {
		return nil, ErrNoTokenProvided
	}
	autoscaler := new(model.Autoscaler)
	return autoscaler, wrapGet(s.db.NewSelect().Model(autoscaler).Where("token = ?", token).Scan(context.Background()))
}

func (s storage) AutoscalerCreate(autoscaler *model.Autoscaler) error {
	// only Insert set auto created ID back to object
	_, err := s.db.NewInsert().Model(autoscaler).Exec(context.Background())
	return err
}

func (s storage) AutoscalerUpdate(autoscaler *model.Autoscaler) error {
	_, err := s.db.NewUpdate().Model(autoscaler).WherePK().Exec(context.Background())
	return err
}

func (s storage) AutoscalerDelete(autoscaler *model.Autoscaler) error {
	return wrapDelete(s.db.NewDelete().Model((*model.Autoscaler)(nil)).Where("id = ?", autoscaler.ID).Exec(context.Background()))
}

func (s storage) AutoscalerUpdateHeartbeat(id int64, activeAgents, pendingAgents int32, pendingSince int64, version string) error {
	_, err := s.db.NewUpdate().Model((*model.Autoscaler)(nil)).
		Set("last_contact = ?", time.Now().Unix()).
		Set("active_agents = ?", activeAgents).
		Set("pending_agents = ?", pendingAgents).
		Set("pending_since = ?", pendingSince).
		Set("version = ?", version).
		Where("id = ?", id).
		Exec(context.Background())
	return err
}

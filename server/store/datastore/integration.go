// Copyright 2026 Crow Authors
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
	"xorm.io/builder"

	"codeberg.org/crowci/crow/v5/server/model"
)

func (s storage) IntegrationFind(id int64) (*model.Integration, error) {
	integration := new(model.Integration)
	return integration, wrapGet(s.engine.ID(id).Get(integration))
}

func (s storage) IntegrationFindByName(userID int64, name string) (*model.Integration, error) {
	integration := new(model.Integration)
	return integration, wrapGet(s.engine.Where(
		builder.Eq{"user_id": userID, "name": name},
	).Get(integration))
}

func (s storage) IntegrationList(userID int64, opts *model.ListOptions) ([]*model.Integration, error) {
	integrations := make([]*model.Integration, 0)
	return integrations, s.paginate(opts).Where("user_id = ?", userID).
		OrderBy("name ASC").
		Find(&integrations)
}

func (s storage) IntegrationListAll() ([]*model.Integration, error) {
	integrations := make([]*model.Integration, 0)
	return integrations, s.engine.OrderBy("name ASC").Find(&integrations)
}

// IntegrationListAccessible returns integrations that the user can access.
// This includes:
// - Integrations owned by the user
// - Integrations where the user is in AllowedUserIDs
// - Integrations where any of the user's orgs are in AllowedOrgIDs
// - Integrations where any of the user's repos are in AllowedRepoIDs.
func (s storage) IntegrationListAccessible(userID int64, orgIDs []int64, repoIDs []int64, opts *model.ListOptions) ([]*model.Integration, error) {
	integrations := make([]*model.Integration, 0)

	// Build query to find accessible integrations
	// Note: JSON array containment is database-specific.
	// We fetch all active integrations and filter in Go for portability.
	err := s.paginate(opts).Where("is_active = ?", true).
		OrderBy("name ASC").
		Find(&integrations)
	if err != nil {
		return nil, err
	}

	// Filter for accessible integrations
	accessible := make([]*model.Integration, 0, len(integrations))
	for _, integration := range integrations {
		if integration.HasAccess(userID, 0, 0) {
			accessible = append(accessible, integration)
			continue
		}

		// Check org access
		for _, orgID := range orgIDs {
			if integration.HasAccess(userID, orgID, 0) {
				accessible = append(accessible, integration)
				break
			}
		}

		// Check repo access (if not already added)
		if len(accessible) == 0 || accessible[len(accessible)-1].ID != integration.ID {
			for _, repoID := range repoIDs {
				if integration.HasAccess(userID, 0, repoID) {
					accessible = append(accessible, integration)
					break
				}
			}
		}
	}

	return accessible, nil
}

func (s storage) IntegrationCreate(integration *model.Integration) error {
	// Set defaults before insert
	integration.BeforeInsert()

	_, err := s.engine.Insert(integration)
	return err
}

func (s storage) IntegrationUpdate(integration *model.Integration) error {
	_, err := s.engine.ID(integration.ID).AllCols().Update(integration)
	return err
}

func (s storage) IntegrationDelete(integration *model.Integration) error {
	return wrapDelete(s.engine.ID(integration.ID).Delete(new(model.Integration)))
}

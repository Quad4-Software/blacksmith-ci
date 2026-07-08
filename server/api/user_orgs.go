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

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/router/middleware/session"
	"codefloe.com/crowci/crow/v6/server/store"
)

type GetUserOrgListInput struct{}

type GetUserOrgListOutput struct {
	Body []*model.Org
}

func GetUserOrgList(ctx context.Context, _ *GetUserOrgListInput) (*GetUserOrgListOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	// Iterate all linked forges to collect teams from each
	userForges, err := _store.UserForgeList(user.ID)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get user forges")
		return nil, huma.Error500InternalServerError("Cannot get user forges", err)
	}

	orgs := make([]*model.Org, 0)
	seenOrgs := make(map[int64]bool)

	for _, uf := range userForges {
		_forge, err := server.Config.Services.Manager.ForgeByID(uf.ForgeID)
		if err != nil {
			log.Error().Err(err).Int64("forge_id", uf.ForgeID).Msg("Cannot get forge by ID, skipping")
			continue
		}

		// Build a user with this forge's credentials
		forgeUser := *user
		forgeUser.ForgeID = uf.ForgeID
		forgeUser.AccessToken = uf.AccessToken
		forgeUser.RefreshToken = uf.RefreshToken
		forgeUser.Expiry = uf.Expiry

		teams, err := _forge.Teams(c, &forgeUser)
		if err != nil {
			log.Error().Err(err).Int64("forge_id", uf.ForgeID).Msg("Cannot get user teams from forge, skipping")
			continue
		}

		log.Debug().Int("team_count", len(teams)).Int64("forge_id", uf.ForgeID).Msg("Fetched teams from forge for org list")

		for _, team := range teams {
			org, err := _store.OrgFindByName(team.Login, uf.ForgeID)
			if err != nil {
				log.Debug().Err(err).Str("org", team.Login).Int64("forge_id", uf.ForgeID).Msg("Org not found in Crow database, skipping")
				continue
			}
			if org.IsUser {
				continue
			}
			if seenOrgs[org.ID] {
				continue
			}
			seenOrgs[org.ID] = true
			orgs = append(orgs, org)
		}
	}

	log.Debug().Int("org_count", len(orgs)).Msg("Returning user orgs")

	return &GetUserOrgListOutput{Body: orgs}, nil
}

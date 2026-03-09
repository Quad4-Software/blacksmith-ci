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
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/store"
)

type GetUserSecretListInput struct {
	PaginationParams
	Scope  string `query:"scope" default:"user,org,global" doc:"Comma-separated scopes: user,org,global"`
	Search string `query:"search" doc:"Search term for secret name"`
}

type GetUserSecretListOutput struct {
	Body []*model.SecretWithOrg
}

func GetUserSecretList(ctx context.Context, input *GetUserSecretListInput) (*GetUserSecretListOutput, error) {
	c := ginCtx(ctx)
	_store := store.FromContext(c)
	user := session.User(c)

	// Parse scope filter (default: all scopes)
	scopes := strings.Split(input.Scope, ",")
	includeUser := contains(scopes, "user")
	includeOrg := contains(scopes, "org")
	includeGlobal := contains(scopes, "global")

	searchTerm := input.Search

	// Build org ID list
	orgIDs := make([]int64, 0)

	// Include user's personal org
	if includeUser {
		orgIDs = append(orgIDs, user.OrgID)
	}

	// Include member orgs from all user forges
	if includeOrg {
		// Get all forge connections for this user
		userForges, err := _store.UserForgeList(user.ID)
		if err != nil {
			log.Error().Err(err).Msg("Cannot get user forges")
			return nil, huma.Error500InternalServerError("Cannot get user forges", err)
		}

		log.Debug().Int("user_forge_count", len(userForges)).Msg("Fetched user forges")

		// Process each forge connection
		for _, userForge := range userForges {
			_forge, err := server.Config.Services.Manager.ForgeByID(userForge.ForgeID)
			if err != nil {
				log.Error().Err(err).Int64("forge_id", userForge.ForgeID).Msg("Cannot get forge by ID, skipping")
				continue
			}

			// Create a temporary user with this forge's tokens
			forgeUser := &model.User{
				ID:            user.ID,
				AccessToken:   userForge.AccessToken,
				RefreshToken:  userForge.RefreshToken,
				Expiry:        userForge.Expiry,
				ForgeID:       userForge.ForgeID,
				ForgeRemoteID: userForge.ForgeRemoteID,
			}

			teams, err := _forge.Teams(c, forgeUser)
			if err != nil {
				log.Debug().Err(err).Int64("forge_id", userForge.ForgeID).Msg("Cannot get teams from forge, skipping")
				continue
			}

			log.Debug().Int("team_count", len(teams)).Int64("forge_id", userForge.ForgeID).Msg("Fetched teams from forge")

			// Map teams to org IDs
			for _, team := range teams {
				log.Debug().Str("team_login", team.Login).Int64("forge_id", userForge.ForgeID).Msg("Processing team")
				org, err := _store.OrgFindByName(team.Login, userForge.ForgeID)
				if err != nil {
					log.Debug().Err(err).Str("org", team.Login).Int64("forge_id", userForge.ForgeID).Msg("Org not found in Crow database, skipping")
					continue
				}
				// Skip if it's the user's personal org (already added)
				if org.ID != user.OrgID {
					log.Debug().Int64("org_id", org.ID).Str("org_name", org.Name).Int64("forge_id", userForge.ForgeID).Msg("Added org to secret list")
					orgIDs = append(orgIDs, org.ID)
				} else {
					log.Debug().Str("org_name", org.Name).Msg("Skipping personal org (already included)")
				}
			}
		}

		log.Debug().Int("total_org_count", len(orgIDs)).Msg("Total orgs to fetch secrets for")
	}

	// Build repo ID list
	repoIDs := make([]int64, 0)
	repos, err := _store.RepoList(user, false, true)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get user repos")
		return nil, huma.Error500InternalServerError("Error fetching repos", err)
	}

	log.Debug().Int("repo_count", len(repos)).Msg("Fetched repos from database")

	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}

	// Fetch secrets from datastore
	log.Debug().
		Interface("org_ids", orgIDs).
		Interface("repo_ids", repoIDs).
		Bool("include_global", includeGlobal).
		Str("search_term", searchTerm).
		Msg("Calling UserSecretList")

	secrets, err := _store.UserSecretList(orgIDs, repoIDs, includeGlobal, searchTerm, &model.ListOptions{
		Page:    input.Page,
		PerPage: input.PerPage,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("Error getting secret list", err)
	}

	log.Debug().Int("secret_count", len(secrets)).Msg("UserSecretList returned secrets")

	// Enrich with org names, repo names, scope types, and forge info
	type orgInfo struct {
		name    string
		forgeID int64
	}
	type repoInfo struct {
		fullName string
		forgeID  int64
	}
	type forgeInfo struct {
		name  string
		icon  string
		ftype string
	}
	orgCache := make(map[int64]orgInfo)
	repoCache := make(map[int64]repoInfo)
	forgeCache := make(map[int64]forgeInfo)
	enrichedSecrets := make([]*model.SecretWithOrg, 0, len(secrets))

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

	for _, secret := range secrets {
		var enriched *model.SecretWithOrg

		if secret.RepoID > 0 {
			// Repo-level secret
			var info repoInfo
			if cached, ok := repoCache[secret.RepoID]; ok {
				info = cached
			} else {
				repo, err := _store.GetRepo(secret.RepoID)
				if err == nil {
					info = repoInfo{fullName: repo.FullName, forgeID: repo.ForgeID}
					repoCache[secret.RepoID] = info
				}
			}
			forgeName, forgeIcon, forgeType := getForgeInfo(info.forgeID)
			enriched = secret.WithRepo(info.fullName, info.forgeID, forgeName, forgeIcon, forgeType)
		} else {
			// Org or global secret
			orgName := "Global"
			isPersonalOrg := false
			var forgeID int64

			if secret.OrgID > 0 {
				// Check if cached
				if info, cached := orgCache[secret.OrgID]; cached {
					orgName = info.name
					forgeID = info.forgeID
				} else {
					org, err := _store.OrgGet(secret.OrgID)
					if err == nil {
						orgName = org.Name
						forgeID = org.ForgeID
						orgCache[secret.OrgID] = orgInfo{name: orgName, forgeID: forgeID}
					}
				}

				// Check if this is the user's personal org
				isPersonalOrg = secret.OrgID == user.OrgID
			}

			forgeName, forgeIcon, forgeType := getForgeInfo(forgeID)
			enriched = secret.WithOrg(orgName, isPersonalOrg, forgeID, forgeName, forgeIcon, forgeType)
		}

		enrichedSecrets = append(enrichedSecrets, enriched)
	}

	// Copy to remove sensitive fields
	result := make([]*model.SecretWithOrg, len(enrichedSecrets))
	for i, s := range enrichedSecrets {
		copied := s.Copy()
		result[i] = &model.SecretWithOrg{
			Secret:    copied,
			OrgName:   s.OrgName,
			RepoName:  s.RepoName,
			ScopeType: s.ScopeType,
			ForgeID:   s.ForgeID,
			ForgeName: s.ForgeName,
			ForgeIcon: s.ForgeIcon,
			ForgeType: s.ForgeType,
		}
	}

	log.Debug().
		Int("enriched_count", len(enrichedSecrets)).
		Int("result_count", len(result)).
		Msg("Returning secrets response")

	return &GetUserSecretListOutput{Body: result}, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.TrimSpace(s) == item {
			return true
		}
	}
	return false
}

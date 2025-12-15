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

	"codeberg.org/crowci/crow/v5/server"
	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/router/middleware/session"
	"codeberg.org/crowci/crow/v5/server/store"
)

// GetUserSecretList
//
//	@Summary	List all secrets accessible by the current user
//	@Router		/user/secrets [get]
//	@Produce	json
//	@Success	200	{array}	SecretWithOrg
//	@Tags		User
//	@Param		Authorization	header	string	true	"Insert your personal access token"			default(Bearer <personal access token>)
//	@Param		scope			query	string	false	"Comma-separated scopes: user,org,global"	default("user,org,global")
//	@Param		search			query	string	false	"Search term for secret name"
//	@Param		page			query	int		false	"for response pagination, page offset number"	default(1)
//	@Param		perPage			query	int		false	"for response pagination, max items per page"	default(50)
func GetUserSecretList(c *gin.Context) {
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

		log.Debug().Int("team_count", len(teams)).Msg("Fetched teams from forge")

		// Map teams to org IDs
		for _, team := range teams {
			log.Debug().Str("team_login", team.Login).Msg("Processing team")
			org, err := _store.OrgFindByName(team.Login, user.ForgeID)
			if err != nil {
				log.Debug().Err(err).Str("org", team.Login).Msg("Org not found in Crow database, skipping")
				continue
			}
			// Skip if it's the user's personal org (already added)
			if org.ID != user.OrgID {
				log.Debug().Int64("org_id", org.ID).Str("org_name", org.Name).Msg("Added org to secret list")
				orgIDs = append(orgIDs, org.ID)
			} else {
				log.Debug().Str("org_name", org.Name).Msg("Skipping personal org (already included)")
			}
		}

		log.Debug().Int("total_org_count", len(orgIDs)).Msg("Total orgs to fetch secrets for")
	}

	// Build repo ID list
	repoIDs := make([]int64, 0)
	repos, err := _store.RepoList(user, false, true)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get user repos")
		c.String(http.StatusInternalServerError, "Error fetching repos. %s", err)
		return
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

	secrets, err := _store.UserSecretList(orgIDs, repoIDs, includeGlobal, searchTerm, session.Pagination(c))
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting secret list. %s", err)
		return
	}

	log.Debug().Int("secret_count", len(secrets)).Msg("UserSecretList returned secrets")

	// Enrich with org names, repo names, and scope types
	orgCache := make(map[int64]string)
	repoCache := make(map[int64]string)
	enrichedSecrets := make([]*model.SecretWithOrg, 0, len(secrets))

	for _, secret := range secrets {
		var enriched *model.SecretWithOrg

		if secret.RepoID > 0 {
			// Repo-level secret
			repoName := ""
			if name, cached := repoCache[secret.RepoID]; cached {
				repoName = name
			} else {
				repo, err := _store.GetRepo(secret.RepoID)
				if err == nil {
					repoName = repo.FullName
					repoCache[secret.RepoID] = repoName
				}
			}
			enriched = secret.WithRepo(repoName)
		} else {
			// Org or global secret
			orgName := "Global"
			isPersonalOrg := false

			if secret.OrgID > 0 {
				// Check if cached
				if name, cached := orgCache[secret.OrgID]; cached {
					orgName = name
				} else {
					org, err := _store.OrgGet(secret.OrgID)
					if err == nil {
						orgName = org.Name
						orgCache[secret.OrgID] = orgName
					}
				}

				// Check if this is the user's personal org
				isPersonalOrg = secret.OrgID == user.OrgID
			}

			enriched = secret.WithOrg(orgName, isPersonalOrg)
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
		}
	}

	log.Debug().
		Int("enriched_count", len(enrichedSecrets)).
		Int("result_count", len(result)).
		Msg("Returning secrets response")

	c.JSON(http.StatusOK, result)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.TrimSpace(s) == item {
			return true
		}
	}
	return false
}

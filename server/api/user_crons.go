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

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/router/middleware/session"
	"codefloe.com/crowci/crow/v5/server/store"
)

// GetUserCronList
//
//	@Summary	List all crons from repos accessible by the current user
//	@Router		/user/crons [get]
//	@Produce	json
//	@Success	200	{array}	model.CronWithRepo
//	@Tags		User
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
//	@Param		search			query	string	false	"Search term for cron name"
//	@Param		page			query	int		false	"for response pagination, page offset number"	default(1)
//	@Param		perPage			query	int		false	"for response pagination, max items per page"	default(50)
func GetUserCronList(c *gin.Context) {
	_store := store.FromContext(c)
	user := session.User(c)

	searchTerm := c.Query("search")

	// Build repo ID list (all repos user has access to)
	repos, err := _store.RepoList(user, false, true)
	if err != nil {
		log.Error().Err(err).Msg("Cannot get user repos")
		c.String(http.StatusInternalServerError, "Error fetching repos. %s", err)
		return
	}

	log.Debug().Int("repo_count", len(repos)).Msg("Fetched repos from database")

	repoIDs := make([]int64, 0, len(repos))
	type repoInfo struct {
		fullName      string
		owner         string
		defaultBranch string
		forgeID       int64
	}
	repoCache := make(map[int64]repoInfo, len(repos))
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
		repoCache[repo.ID] = repoInfo{fullName: repo.FullName, owner: repo.Owner, defaultBranch: repo.Branch, forgeID: repo.ForgeID}
	}

	// Fetch crons from datastore
	log.Debug().
		Interface("repo_ids", repoIDs).
		Str("search_term", searchTerm).
		Msg("Calling UserCronList")

	crons, err := _store.UserCronList(repoIDs, searchTerm, session.Pagination(c))
	if err != nil {
		c.String(http.StatusInternalServerError, "Error getting cron list. %s", err)
		return
	}

	log.Debug().Int("cron_count", len(crons)).Msg("UserCronList returned crons")

	// Enrich with repo names, owners, and forge info
	type forgeInfo struct {
		name  string
		icon  string
		ftype string
	}
	forgeCache := make(map[int64]forgeInfo)

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

	enrichedCrons := make([]*model.CronWithRepo, 0, len(crons))
	for _, cron := range crons {
		var info repoInfo
		if cached, ok := repoCache[cron.RepoID]; ok {
			info = cached
		} else {
			// Fallback: fetch from DB if not in cache (shouldn't happen)
			repo, err := _store.GetRepo(cron.RepoID)
			if err == nil {
				info = repoInfo{fullName: repo.FullName, owner: repo.Owner, defaultBranch: repo.Branch, forgeID: repo.ForgeID}
				repoCache[cron.RepoID] = info
			}
		}

		forgeName, forgeIcon, forgeType := getForgeInfo(info.forgeID)
		enriched := cron.WithRepo(info.fullName, info.owner, info.defaultBranch, info.forgeID, forgeName, forgeIcon, forgeType)
		enrichedCrons = append(enrichedCrons, enriched)
	}

	log.Debug().
		Int("enriched_count", len(enrichedCrons)).
		Msg("Returning crons response")

	c.JSON(http.StatusOK, enrichedCrons)
}

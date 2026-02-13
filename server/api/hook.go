// Copyright 2022 Woodpecker Authors
// Copyright 2021 Informatyka Boguslawski sp. z o.o. sp.k., http://www.ib.pl/
// Copyright 2018 Drone.IO Inc.
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
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/forge"
	"codefloe.com/crowci/crow/v5/server/forge/types"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/pipeline"
	"codefloe.com/crowci/crow/v5/server/store"
	"codefloe.com/crowci/crow/v5/shared/token"
)

// GetQueueInfo
//
//	@Summary		Get pipeline queue information
//	@Description	Returns pipeline queue information with agent details
//	@Router			/queue/info [get]
//	@Produce		json
//	@Success		200	{object}	QueueInfo
//	@Tags			Pipeline queues
//	@Param			Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
func GetQueueInfo(c *gin.Context) {
	info := server.Config.Services.Queue.Info(c)
	_store := store.FromContext(c)

	// Create a map to store agent names by ID
	agentNameMap := make(map[int64]string)

	// Process tasks and add agent names
	pendingWithAgents, err := processQueueTasks(_store, info.Pending, agentNameMap)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	waitingWithAgents, err := processQueueTasks(_store, info.WaitingOnDeps, agentNameMap)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	runningWithAgents, err := processQueueTasks(_store, info.Running, agentNameMap)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Create response with agent-enhanced tasks
	response := model.QueueInfo{
		Pending:       pendingWithAgents,
		WaitingOnDeps: waitingWithAgents,
		Running:       runningWithAgents,
		Stats: struct {
			WorkerCount        int `json:"worker_count"`
			PendingCount       int `json:"pending_count"`
			WaitingOnDepsCount int `json:"waiting_on_deps_count"`
			RunningCount       int `json:"running_count"`
		}{
			WorkerCount:        info.Stats.Workers,
			PendingCount:       info.Stats.Pending,
			WaitingOnDepsCount: info.Stats.WaitingOnDeps,
			RunningCount:       info.Stats.Running,
		},
		Paused: info.Paused,
	}

	c.IndentedJSON(http.StatusOK, response)
}

// getAgentName finds an agent's name, utilizing a map as a cache.
func getAgentName(store store.Store, agentNameMap map[int64]string, agentID int64) (string, bool) {
	// 1. Check the cache first.
	name, exists := agentNameMap[agentID]
	if exists {
		return name, true
	}

	// 2. If not in cache, query the store.
	agent, err := store.AgentFind(agentID)
	if err != nil || agent == nil {
		// Agent not found or an error occurred.
		return "", false
	}

	// 3. Found the agent, update the cache and return the name.
	if agent.Name != "" {
		agentNameMap[agentID] = agent.Name
		return agent.Name, true
	}

	return "", false
}

// processQueueTasks converts tasks to QueueTask structs and adds agent names.
func processQueueTasks(store store.Store, tasks []*model.Task, agentNameMap map[int64]string) ([]model.QueueTask, error) {
	result := make([]model.QueueTask, 0, len(tasks))

	for _, task := range tasks {
		taskResponse := model.QueueTask{
			Task: *task,
		}

		if task.AgentID != 0 {
			name, ok := getAgentName(store, agentNameMap, task.AgentID)
			if ok {
				taskResponse.AgentName = name
			} else {
				// Agent not found (possibly deleted), use a placeholder
				taskResponse.AgentName = fmt.Sprintf("agent-%d (deleted)", task.AgentID)
			}
		}

		if task.PipelineID != 0 {
			p, err := store.GetPipeline(task.PipelineID)
			if err != nil {
				return nil, fmt.Errorf("pipeline not found for task %s", task.ID)
			}

			taskResponse.PipelineNumber = p.Number
			taskResponse.PipelineCreated = p.Created
			taskResponse.PipelineStarted = p.Started
		}

		result = append(result, taskResponse)
	}
	return result, nil
}

// PauseQueue
//
//	@Summary	Pause the pipeline queue
//	@Router		/queue/pause [post]
//	@Produce	plain
//	@Success	204
//	@Tags		Pipeline queues
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
func PauseQueue(c *gin.Context) {
	server.Config.Services.Queue.Pause()
	c.Status(http.StatusNoContent)
}

// ResumeQueue
//
//	@Summary	Resume the pipeline queue
//	@Router		/queue/resume [post]
//	@Produce	plain
//	@Success	204
//	@Tags		Pipeline queues
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
func ResumeQueue(c *gin.Context) {
	server.Config.Services.Queue.Resume()
	c.Status(http.StatusNoContent)
}

// BlockTilQueueHasRunningItem
//
//	@Summary	Block til pipeline queue has a running item
//	@Router		/queue/norunningpipelines [get]
//	@Produce	plain
//	@Success	204
//	@Tags		Pipeline queues
//	@Param		Authorization	header	string	true	"Insert your personal access token"	default(Bearer <personal access token>)
func BlockTilQueueHasRunningItem(c *gin.Context) {
	for {
		info := server.Config.Services.Queue.Info(c)
		if info.Stats.Running == 0 {
			break
		}
	}
	c.Status(http.StatusNoContent)
}

// PostHook
//
//	@Summary	Incoming webhook from forge
//	@Router		/hook [post]
//	@Produce	plain
//	@Success	200
//	@Tags		System
//	@Param		hook	body	object	true	"the webhook payload; forge is automatically detected"
func PostHook(c *gin.Context) {
	log.Trace().Msg("PostHook: entered handler")
	_store := store.FromContext(c)

	//
	// 1. Check if the webhook is valid and authorized
	//

	var repo *model.Repo

	_, err := token.ParseRequest([]token.Type{token.HookToken}, c.Request, func(t *token.Token) (string, error) {
		var err error
		repo, err = getRepoFromToken(_store, t)
		if err != nil {
			return "", err
		}

		return repo.Hash, nil
	})
	if err != nil {
		msg := "failure to parse token from hook"
		log.Error().Err(err).Msg(msg)
		c.String(http.StatusBadRequest, msg)
		return
	}

	if repo == nil {
		msg := "failure to get repo from token"
		log.Error().Msg(msg)
		c.String(http.StatusBadRequest, msg)
		return
	}

	log.Trace().Str("repo", repo.FullName).Int64("repo-id", repo.ID).Msg("PostHook: token parsed, got repo")

	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Int64("repo-id", repo.ID).Msgf("Cannot get forge with id: %d", repo.ForgeID)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	//
	// 2. Parse the webhook data
	//

	repoFromForge, pipelineFromForge, err := _forge.Hook(c, c.Request)
	log.Trace().
		Bool("has-repo", repoFromForge != nil).
		Bool("has-pipeline", pipelineFromForge != nil).
		Err(err).
		Msg("PostHook: forge.Hook returned")
	if err != nil {
		if errors.Is(err, &types.ErrIgnoreEvent{}) {
			msg := fmt.Sprintf("forge driver: %s", err)
			log.Debug().Err(err).Msg(msg)
			c.String(http.StatusOK, msg)
			return
		}

		msg := fmt.Sprintf("failure to parse hook: %v", err)
		log.Error().Err(err).Msg("failure to parse hook")
		c.String(http.StatusBadRequest, msg)
		return
	}

	if pipelineFromForge == nil {
		msg := "ignoring hook: hook parsing resulted in empty pipeline"
		log.Debug().Msg(msg)
		c.String(http.StatusOK, msg)
		return
	}
	if repoFromForge == nil {
		msg := "failure to ascertain repo from hook"
		log.Debug().Msg(msg)
		c.String(http.StatusBadRequest, msg)
		return
	}

	//
	// 3. Check the repo from the token is matching the repo returned by the forge
	//

	if repo.ForgeRemoteID != repoFromForge.ForgeRemoteID {
		log.Warn().Msgf("ignoring hook: repo %s does not match the repo from the token", repo.FullName)
		c.String(http.StatusBadRequest, "failure to parse token from hook")
		return
	}

	//
	// 4. Check if the repo is active and has an owner
	//

	if !repo.IsActive {
		log.Debug().Msgf("ignoring hook: repo %s is inactive", repoFromForge.FullName)
		c.Status(http.StatusNoContent)
		return
	}

	if repo.UserID == 0 {
		log.Warn().Msgf("ignoring hook. repo %s has no owner.", repo.FullName)
		c.Status(http.StatusNoContent)
		return
	}

	user, err := _store.GetUser(repo.UserID)
	if err != nil {
		handleDBError(c, err)
		return
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	// This ensures we use the correct OAuth tokens for the forge the repo belongs to.
	var isSecondaryForge bool
	if userForge, ufErr := _store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
		isSecondaryForge = !userForge.IsPrimary
		log.Debug().
			Int64("user_id", user.ID).
			Int64("forge_id", repo.ForgeID).
			Str("forge_login", userForge.ForgeLogin).
			Bool("is_secondary", isSecondaryForge).
			Msg("using UserForge tokens for webhook")
	}

	// Use appropriate refresh function based on whether this is a secondary forge
	var refreshErr error
	if isSecondaryForge {
		refreshErr = forge.RefreshUserForge(c, _forge, _store, user, repo.ForgeID)
	} else {
		refreshErr = forge.Refresh(c, _forge, _store, user)
	}
	if refreshErr != nil {
		log.Error().Err(refreshErr).Msg("Failed to refresh user auth token")
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusUnauthorized, "Your authorization has expired and couldn't be refreshed automatically. Please log in again")
		c.Abort()
		return
	}

	//
	// 4. Update the repo
	//

	if repo.FullName != repoFromForge.FullName {
		// create a redirection
		err = _store.CreateRedirection(&model.Redirection{ForgeID: repo.ForgeID, RepoID: repo.ID, FullName: repo.FullName})
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	repo.Update(repoFromForge)
	err = _store.UpdateRepo(repo)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	//
	// 5. Check if pull requests are allowed for this repo
	//

	if (pipelineFromForge.Event == model.EventPull || pipelineFromForge.Event == model.EventPullClosed || pipelineFromForge.Event == model.EventPullEdited) && !repo.AllowPull {
		log.Debug().Str("repo", repo.FullName).Msg("ignoring hook: pull requests are disabled for this repo in crow")
		c.Status(http.StatusNoContent)
		return
	}

	//
	// 6. Finally create a pipeline
	//

	log.Trace().Str("repo", repo.FullName).Str("event", string(pipelineFromForge.Event)).Msg("PostHook: about to create pipeline")
	pl, err := pipeline.Create(c, _store, repo, pipelineFromForge)
	if err != nil {
		log.Trace().Err(err).Str("repo", repo.FullName).Msg("PostHook: pipeline creation failed")
		handlePipelineErr(c, err)
	} else {
		log.Trace().Str("repo", repo.FullName).Int64("pipeline-id", pl.ID).Msg("PostHook: pipeline created successfully")
		c.JSON(http.StatusOK, pl)
	}
}

func getRepoFromToken(store store.Store, t *token.Token) (*model.Repo, error) {
	if t.Get("repo-forge-remote-id") != "" {
		remoteID := model.ForgeRemoteID(t.Get("repo-forge-remote-id"))

		// Use both forge ID and remote ID for proper multi-forge support
		if forgeIDStr := t.Get("forge-id"); forgeIDStr != "" {
			forgeID, err := strconv.ParseInt(forgeIDStr, 10, 64)
			if err != nil {
				return nil, err
			}
			return store.GetRepoByForgeAndRemoteID(forgeID, remoteID)
		}

		// Fallback for legacy tokens without forge-id
		return store.GetRepoForgeID(remoteID)
	}

	// get the repo by the repo-id
	// TODO: remove in next major
	repoID, err := strconv.ParseInt(t.Get("repo-id"), 10, 64)
	if err != nil {
		return nil, err
	}
	return store.GetRepo(repoID)
}

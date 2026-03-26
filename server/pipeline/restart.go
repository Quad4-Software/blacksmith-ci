// Copyright 2022 Woodpecker Authors
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

package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server"
	forge_types "codefloe.com/crowci/crow/v5/server/forge/types"
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store"
)

// Restart a pipeline by creating a new one out of the old and start it.
func Restart(ctx context.Context, store store.Store, lastPipeline *model.Pipeline, user *model.User, repo *model.Repo, envs map[string]string) (*model.Pipeline, error) {
	// Debug logging
	log.Debug().
		Str("event", string(lastPipeline.Event)).
		Int64("number", lastPipeline.Number).
		Interface("envs", envs).
		Msg("Restart function called")
	forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		msg := fmt.Sprintf("failure to load forge for repo '%s'", repo.FullName)
		log.Error().Err(err).Str("repo", repo.FullName).Msg(msg)
		return nil, errors.New(msg)
	}

	if lastPipeline.Status == model.StatusBlocked {
		return nil, &ErrBadRequest{Msg: "cannot restart a pipeline with status blocked"}
	}

	// fetch the old pipeline config from the database
	configs, err := store.ConfigsForPipeline(lastPipeline.ID)
	if err != nil {
		log.Error().Err(err).Msgf("failure to get pipeline config for %s", repo.FullName)
		return nil, &ErrNotFound{Msg: fmt.Sprintf("failure to get pipeline config for %s. %s", repo.FullName, err)}
	}

	var pipelineFiles []*forge_types.FileMeta
	for _, y := range configs {
		pipelineFiles = append(pipelineFiles, &forge_types.FileMeta{Data: y.Data, Name: y.Name})
	}

	// If the config service is active we should refetch the config in case something changed
	configService := server.Config.Services.Manager.ConfigServiceFromRepo(repo)
	pipelineFiles, err = configService.Fetch(ctx, forge, user, repo, lastPipeline, pipelineFiles, true)
	if err != nil {
		return nil, &ErrBadRequest{
			Msg: fmt.Sprintf("On fetching external pipeline config: %s", err),
		}
	}

	newPipeline := createNewOutOfOld(lastPipeline)

	// Check if this is a deployment triggered from a non-deployment pipeline
	// The API passes CROW_ORIGINAL_EVENT when converting a non-deployment to deployment
	originalEventStr, isDeploymentFromOtherEvent := envs["CROW_ORIGINAL_EVENT"]

	// Log for debugging
	if isDeploymentFromOtherEvent {
		log.Debug().
			Str("original_event", originalEventStr).
			Str("new_event", string(newPipeline.Event)).
			Int64("pipeline_number", lastPipeline.Number).
			Msg("Creating fresh deployment from non-deployment event - not setting parent")
	} else {
		log.Debug().
			Str("event", string(newPipeline.Event)).
			Int64("pipeline_number", lastPipeline.Number).
			Bool("has_marker", isDeploymentFromOtherEvent).
			Msg("Standard restart - setting parent")
	}

	// Only set parent for actual restarts
	// Don't set parent for fresh deployments (when CROW_ORIGINAL_EVENT is present)
	if !isDeploymentFromOtherEvent {
		newPipeline.Parent = lastPipeline.Number
		log.Debug().
			Int64("parent_set_to", lastPipeline.Number).
			Msg("Setting parent for restart")
	} else {
		log.Debug().
			Int64("parent_value", newPipeline.Parent).
			Msg("NOT setting parent for fresh deployment - parent should be 0")
	}

	// Clean up the temporary marker
	delete(envs, "CROW_ORIGINAL_EVENT")

	log.Debug().
		Int64("final_parent_value", newPipeline.Parent).
		Str("event", string(newPipeline.Event)).
		Msg("About to create pipeline with parent value")

	err = store.CreatePipeline(newPipeline)
	if err != nil {
		msg := fmt.Sprintf("failure to save pipeline for %s", repo.FullName)
		log.Error().Err(err).Msg(msg)
		return nil, errors.New(msg)
	}

	// If the original pipeline had no configs saved (e.g., due to a forge connectivity
	// error during the initial run), try to use the freshly-fetched configs instead.
	if len(configs) == 0 {
		if len(pipelineFiles) == 0 {
			newPipeline, uErr := UpdateToStatusError(store, *newPipeline, errors.New("failed to load pipeline config from database or forge"))
			if uErr != nil {
				log.Debug().Err(uErr).Msg("failure to update pipeline status")
			} else {
				updatePipelineStatus(ctx, forge, newPipeline, repo, user)
			}
			return newPipeline, nil
		}

		log.Info().Str("repo", repo.FullName).Msg("original pipeline had no configs saved, using freshly-fetched configs from forge")
		for _, pf := range pipelineFiles {
			config, persistErr := findOrPersistPipelineConfig(store, newPipeline, pf)
			if persistErr != nil {
				msg := fmt.Sprintf("failure to persist pipeline config for %s", repo.FullName)
				log.Error().Err(persistErr).Msg(msg)
				return nil, errors.New(msg)
			}
			configs = append(configs, config)
		}
	}
	if err := linkPipelineConfigs(store, configs, newPipeline.ID); err != nil {
		msg := fmt.Sprintf("failure to persist pipeline config for %s.", repo.FullName)
		log.Error().Err(err).Msg(msg)
		return nil, errors.New(msg)
	}

	// For manual pipelines, preserve the original workflow filter so that
	// restarting only re-runs the workflows that were selected, not all
	// workflows with a manual trigger.
	var workflowFilter []string
	if lastPipeline.Event == model.EventManual {
		workflows, wErr := store.WorkflowGetTree(lastPipeline)
		if wErr == nil {
			for _, w := range workflows {
				workflowFilter = append(workflowFilter, w.Name)
			}
		}
	}

	newPipeline, pipelineItems, err := createPipelineItems(ctx, forge, store, newPipeline, user, repo, pipelineFiles, envs, workflowFilter)
	if err != nil {
		log.Error().Err(err).Msgf("failure to createPipelineItems for %s", repo.FullName)
		return nil, err
	}

	if err := prepareStart(ctx, forge, store, newPipeline, user, repo); err != nil {
		msg := fmt.Sprintf("failure to prepare pipeline for %s", repo.FullName)
		log.Error().Err(err).Msg(msg)
		return nil, errors.New(msg)
	}

	newPipeline, err = start(ctx, forge, store, newPipeline, user, repo, pipelineItems)
	if err != nil {
		msg := fmt.Sprintf("failure to start pipeline for %s", repo.FullName)
		log.Error().Err(err).Msg(msg)
		return nil, errors.New(msg)
	}

	return newPipeline, nil
}

func linkPipelineConfigs(store store.Store, configs []*model.Config, pipelineID int64) error {
	for _, conf := range configs {
		pipelineConfig := &model.PipelineConfig{
			ConfigID:   conf.ID,
			PipelineID: pipelineID,
		}
		err := store.PipelineConfigCreate(pipelineConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

func createNewOutOfOld(old *model.Pipeline) *model.Pipeline {
	newPipeline := *old
	newPipeline.ID = 0
	newPipeline.Number = 0
	newPipeline.Parent = 0 // Reset parent - will be set later if this is a restart
	newPipeline.Status = model.StatusPending
	newPipeline.Started = 0
	newPipeline.Finished = 0
	newPipeline.Errors = nil
	return &newPipeline
}

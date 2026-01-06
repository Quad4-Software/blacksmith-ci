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
	"regexp"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v5/pipeline"
	pipeline_errors "codeberg.org/crowci/crow/v5/pipeline/errors"
	"codeberg.org/crowci/crow/v5/server"
	"codeberg.org/crowci/crow/v5/server/forge"
	forge_types "codeberg.org/crowci/crow/v5/server/forge/types"
	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/pipeline/stepbuilder"
	"codeberg.org/crowci/crow/v5/server/store"
)

var skipPipelineRegex = regexp.MustCompile(`\[(?i:ci *skip|skip *ci)\]`)

// Create a new pipeline and start it.
func Create(ctx context.Context, _store store.Store, repo *model.Repo, pipeline *model.Pipeline) (*model.Pipeline, error) {
	repoUser, err := _store.GetUser(repo.UserID)
	if err != nil {
		msg := fmt.Sprintf("failure to find repo owner via id '%d'", repo.UserID)
		log.Error().Err(err).Str("repo", repo.FullName).Msg(msg)
		return nil, errors.New(msg)
	}

	if pipeline.Event == model.EventPush || pipeline.Event == model.EventPull || pipeline.Event == model.EventPullClosed || pipeline.Event == model.EventPullEdited {
		skipMatch := skipPipelineRegex.FindString(pipeline.Message)
		if len(skipMatch) > 0 {
			ref := pipeline.Commit
			if len(ref) == 0 {
				ref = pipeline.Ref
			}
			log.Debug().Str("repo", repo.FullName).Msgf("ignoring pipeline as skip-ci was found in the commit (%s) message '%s'", ref, pipeline.Message)
			return nil, ErrFiltered
		}
	}

	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		msg := fmt.Sprintf("failure to load forge for repo '%s'", repo.FullName)
		log.Error().Err(err).Str("repo", repo.FullName).Msg(msg)
		return nil, errors.New(msg)
	}

	// If the forge has a refresh token, the current access token
	// may be stale. Therefore, we should refresh prior to dispatching
	// the pipeline.
	if err := forge.Refresh(ctx, _forge, _store, repoUser); err != nil {
		log.Error().Err(err).Msgf("failed to refresh OAuth token for user '%s'", repoUser.Login)
		return nil, fmt.Errorf("failed to authenticate with version control system: %w", err)
	}

	// update some pipeline fields
	pipeline.RepoID = repo.ID
	pipeline.Status = model.StatusCreated
	setApprovalState(repo, pipeline)
	err = _store.CreatePipeline(pipeline)
	if err != nil {
		msg := fmt.Errorf("failed to save pipeline for %s", repo.FullName)
		log.Error().Str("repo", repo.FullName).Err(err).Msg(msg.Error())
		return nil, msg
	}

	// fetch the pipeline file from the forge
	configService := server.Config.Services.Manager.ConfigServiceFromRepo(repo)
	forgeYamlConfigs, configFetchErr := configService.Fetch(ctx, _forge, repoUser, repo, pipeline, nil, false)
	if errors.Is(configFetchErr, &forge_types.ErrConfigNotFound{}) {
		log.Debug().
			Str("repo", repo.FullName).
			Str("branch", pipeline.Ref).
			Str("commit", pipeline.Commit).
			Str("user", repoUser.Login).
			Err(configFetchErr).
			Msgf("skipping pipeline for branch '%s' - no config file found (searched for: %s)", pipeline.Ref, repo.Config)
		if err := _store.DeletePipeline(pipeline); err != nil {
			log.Error().Str("repo", repo.FullName).Err(err).Msg("failed to delete pipeline without config")
		}

		return nil, ErrFiltered
	} else if configFetchErr != nil {
		log.Error().Str("repo", repo.FullName).Err(configFetchErr).Msgf("error while fetching config '%s' in '%s' with user: '%s'", repo.Config, pipeline.Ref, repoUser.Login)
		return nil, updatePipelineWithErr(ctx, _forge, _store, pipeline, repo, repoUser, fmt.Errorf("There was an error retrieving the pipeline configuration from the forge. This is usually caused by connectivity/timeout issues on the forge side. Restarting the build will not help, a new (fresh) commit (or force-push) must be issued to trigger a clean reload.")) //nolint
	}

	// Persist the pipeline config BEFORE parsing. This ensures configs are available
	// for restarts even if parsing fails (e.g., due to secret event restrictions).
	// On restart, secrets are re-fetched from the database, allowing updated
	// secret configurations to take effect.
	var configs []*model.Config
	for _, forgeYamlConfig := range forgeYamlConfigs {
		config, err := findOrPersistPipelineConfig(_store, pipeline, forgeYamlConfig)
		if err != nil {
			msg := fmt.Sprintf("failed to find or persist pipeline config for %s", repo.FullName)
			log.Error().Err(err).Msg(msg)
			return nil, errors.New(msg)
		}
		configs = append(configs, config)
	}
	// Link pipeline to persisted configs
	if err := linkPipelineConfigs(_store, configs, pipeline.ID); err != nil {
		msg := fmt.Sprintf("failed to link pipeline config for %s", repo.FullName)
		log.Error().Err(err).Msg(msg)
		return nil, errors.New(msg)
	}

	pipelineItems, parseErr := parsePipeline(_forge, _store, pipeline, repoUser, repo, forgeYamlConfigs, nil) //nolint:contextcheck
	if pipeline_errors.HasBlockingErrors(parseErr) {
		log.Debug().Str("repo", repo.FullName).Err(parseErr).Msg("failed to parse yaml")
		return pipeline, updatePipelineWithErr(ctx, _forge, _store, pipeline, repo, repoUser, parseErr)
	} else if parseErr != nil {
		pipeline.Errors = pipeline_errors.GetPipelineErrors(parseErr)
	}

	if len(pipelineItems) == 0 {
		log.Debug().Str("repo", repo.FullName).Msg(ErrFiltered.Error())
		if err := _store.DeletePipeline(pipeline); err != nil {
			log.Error().Str("repo", repo.FullName).Err(err).Msg("failed to delete empty pipeline")
		}

		return nil, ErrFiltered
	}

	// Validate that agents exist for the required labels
	if err := validateAgentAvailability(_store, pipelineItems); err != nil {
		log.Debug().Str("repo", repo.FullName).Err(err).Msg("pipeline validation failed - no matching agents")
		return pipeline, updatePipelineWithErr(ctx, _forge, _store, pipeline, repo, repoUser, err)
	}

	pipeline = setPipelineStepsOnPipeline(pipeline, pipelineItems)

	if err := prepareStart(ctx, _forge, _store, pipeline, repoUser, repo); err != nil {
		log.Error().Err(err).Str("repo", repo.FullName).Msgf("error preparing pipeline for %s#%d", repo.FullName, pipeline.Number)
		return nil, err
	}

	if pipeline.Status == model.StatusBlocked {
		return pipeline, nil
	}

	if err := updatePipelinePending(ctx, _forge, _store, pipeline, repo, repoUser); err != nil {
		return nil, err
	}

	pipeline, err = start(ctx, _forge, _store, pipeline, repoUser, repo, pipelineItems)
	if err != nil {
		msg := fmt.Sprintf("failed to start pipeline for %s", repo.FullName)
		log.Error().Err(err).Msg(msg)
		return nil, errors.New(msg)
	}

	return pipeline, nil
}

func updatePipelineWithErr(ctx context.Context, _forge forge.Forge, _store store.Store, pipeline *model.Pipeline, repo *model.Repo, repoUser *model.User, err error) error {
	_pipeline, err := UpdateToStatusError(_store, *pipeline, err)
	if err != nil {
		return err
	}
	// update value in ref
	*pipeline = *_pipeline

	publishPipeline(ctx, _forge, pipeline, repo, repoUser)

	return nil
}

func updatePipelinePending(ctx context.Context, _forge forge.Forge, _store store.Store, pipeline *model.Pipeline, repo *model.Repo, repoUser *model.User) error {
	_pipeline, err := UpdateToStatusPending(_store, *pipeline, "")
	if err != nil {
		return err
	}
	// update value in ref
	*pipeline = *_pipeline

	publishPipeline(ctx, _forge, pipeline, repo, repoUser)

	return nil
}

// validateAgentAvailability checks if there are agents that can run the pipeline items.
// Returns an error with details if no matching agents are found and no autoscaler can provision them.
func validateAgentAvailability(_store store.Store, pipelineItems []*stepbuilder.Item) error {
	// Get all agents
	agents, err := _store.AgentList(nil)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list agents during validation - skipping agent validation")
		return nil // Don't fail pipeline creation if we can't check agents
	}

	// Get all enabled autoscalers
	enabledTrue := true
	autoscalers, err := _store.AutoscalerList(&model.AutoscalerListOptions{
		Enabled: &enabledTrue,
	})
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list autoscalers during validation - continuing without autoscaler check")
		autoscalers = nil // Continue with empty autoscaler list
	}

	// Check each pipeline item
	for _, item := range pipelineItems {
		// Skip items without labels (they can run on any agent)
		if len(item.Labels) == 0 {
			continue
		}

		// Extract user-defined labels (skip crowci/* internal labels)
		requiredLabels := make(map[string]string)
		for key, value := range item.Labels {
			if strings.HasPrefix(key, pipeline.InternalLabelPrefix) {
				continue // Skip internal labels
			}
			if value != "" {
				requiredLabels[key] = value
			}
		}

		// If no user-defined labels, skip
		if len(requiredLabels) == 0 {
			continue
		}

		// Check if any agent can match these labels
		hasMatchingAgent := false
		for _, agent := range agents {
			if agent.NoSchedule {
				continue // Skip agents marked as no-schedule
			}

			if agentCanMatch(agent, requiredLabels) {
				hasMatchingAgent = true
				break
			}
		}

		// If an agent exists, continue
		if hasMatchingAgent {
			continue
		}

		// No agent found, check if any autoscaler can provision one
		canAutoscalerProvision := false
		for _, autoscaler := range autoscalers {
			if autoscaler.CanProvision(requiredLabels) {
				canAutoscalerProvision = true
				break
			}
		}

		// If autoscaler can provision, allow pipeline to queue
		if canAutoscalerProvision {
			log.Debug().
				Str("workflow", item.Workflow.Name).
				Interface("required_labels", requiredLabels).
				Msg("No agent available, but autoscaler can provision one - allowing pipeline to queue")
			continue
		}

		// Neither agent nor autoscaler available
		return formatNoMatchingAgentError(item.Workflow.Name, requiredLabels, agents, autoscalers)
	}

	return nil
}

// agentCanMatch checks if an agent could potentially match the required labels.
func agentCanMatch(agent *model.Agent, requiredLabels map[string]string) bool {
	// Check platform
	if requiredPlatform, ok := requiredLabels["platform"]; ok {
		if agent.Platform != "" && agent.Platform != requiredPlatform {
			return false
		}
	}

	// Check backend
	if requiredBackend, ok := requiredLabels["backend"]; ok {
		if agent.Backend != "" && agent.Backend != requiredBackend {
			return false
		}
	}

	// Check agent name
	if requiredAgent, ok := requiredLabels["agent"]; ok {
		if agent.Name != "" && agent.Name != requiredAgent {
			return false
		}
	}

	// Check hostname
	if requiredHostname, ok := requiredLabels["hostname"]; ok {
		if agent.Hostname != "" && agent.Hostname != requiredHostname {
			return false
		}
	}

	// Check custom labels
	for labelKey, labelValue := range requiredLabels {
		// Skip standard labels we already checked
		if labelKey == "platform" || labelKey == "backend" || labelKey == "agent" || labelKey == "hostname" {
			continue
		}

		// Skip repo and org-id (these are handled specially with wildcards and pipes)
		if labelKey == "repo" || labelKey == "org-id" {
			continue
		}

		// Check if agent has this custom label with matching value
		if agentLabelValue, ok := agent.CustomLabels[labelKey]; ok {
			if agentLabelValue != labelValue {
				return false // Label exists but with different value
			}
		} else {
			return false // Agent doesn't have required label
		}
	}

	return true
}

// formatNoMatchingAgentError creates a user-friendly error message with markdown formatting.
func formatNoMatchingAgentError(workflowName string, requiredLabels map[string]string, agents []*model.Agent, autoscalers []*model.Autoscaler) error {
	var msg strings.Builder

	// Header
	msg.WriteString(fmt.Sprintf("### Workflow `%s` cannot be started\n\n", workflowName))
	msg.WriteString("❌ **No agent matching the required labels was found**\n\n")

	// Required labels section
	msg.WriteString("#### Required Labels\n\n")
	var labelList []string
	for key, value := range requiredLabels {
		labelList = append(labelList, fmt.Sprintf("`%s` = `%s`", key, value))
	}
	sort.Strings(labelList)
	for _, label := range labelList {
		msg.WriteString(fmt.Sprintf("- %s\n", label))
	}
	msg.WriteString("\n")

	// Available agents section - just show count and names
	if len(agents) == 0 {
		msg.WriteString("#### Available Agents\n\n")
		msg.WriteString("⚠️ No agents are currently registered\n")
	} else {
		activeAgents := 0
		var agentNames []string

		for _, agent := range agents {
			if agent.NoSchedule {
				continue
			}
			activeAgents++
			agentNames = append(agentNames, fmt.Sprintf("`%s`", agent.Name))
		}

		if activeAgents == 0 {
			msg.WriteString("#### Available Agents\n\n")
			msg.WriteString("⚠️ All agents are marked as NoSchedule\n")
		} else {
			msg.WriteString(fmt.Sprintf("#### Available Agents: %d\n\n", activeAgents))
			msg.WriteString(strings.Join(agentNames, ", "))
			msg.WriteString("\n\n")
		}
	}

	// Autoscaler section
	if len(autoscalers) == 0 {
		msg.WriteString("#### Autoscalers\n\n")
		msg.WriteString("⚠️ No autoscalers are configured. Consider setting up an autoscaler to automatically provision agents on demand.\n")
	} else {
		msg.WriteString("#### Autoscalers\n\n")

		// Check if any autoscaler could match but is at capacity
		var atCapacity []string
		for _, autoscaler := range autoscalers {
			if !autoscaler.Enabled {
				continue
			}

			// Check if labels would match if not at capacity
			if autoscaler.ActiveAgents >= autoscaler.MaxAgents {
				// Check label compatibility
				labelsMatch := true
				for key, requiredValue := range requiredLabels {
					autoscalerValue, exists := autoscaler.AgentLabels[key]
					if !exists || (autoscalerValue != requiredValue && autoscalerValue != "*") {
						labelsMatch = false
						break
					}
				}
				if labelsMatch {
					atCapacity = append(atCapacity, fmt.Sprintf("`%s` (%d/%d agents)", autoscaler.Name, autoscaler.ActiveAgents, autoscaler.MaxAgents))
				}
			}
		}

		if len(atCapacity) > 0 {
			msg.WriteString("⚠️ **Autoscalers at capacity:**\n")
			for _, name := range atCapacity {
				msg.WriteString(fmt.Sprintf("- %s\n", name))
			}
			msg.WriteString("\nThese autoscalers could provision agents with the required labels but are currently at maximum capacity. ")
			msg.WriteString("Wait for existing pipelines to complete or increase the autoscaler's max capacity.\n")
		} else {
			msg.WriteString("ℹ️ Autoscalers are configured but none can provision agents with the required labels.\n")
			msg.WriteString(fmt.Sprintf("%d autoscaler(s) registered\n", len(autoscalers)))
		}
	}

	return errors.New(msg.String())
}

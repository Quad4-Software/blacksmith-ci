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
	"encoding/json"
	"strconv"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pubsub"
	"codefloe.com/crowci/crow/v6/server/store"
)

// publishToTopic publishes message to UI clients.
func publishToTopic(pipeline *model.Pipeline, repo *model.Repo) {
	AnnotateAutoscalerProvisioning(server.Config.Services.Store, pipeline)

	message := pubsub.Message{
		Labels: map[string]string{
			"repo":    repo.FullName,
			"private": strconv.FormatBool(repo.IsSCMPrivate),
		},
	}
	pipelineCopy := *pipeline

	var err error
	message.Data, err = json.Marshal(model.Event{
		Repo:     *repo,
		Pipeline: pipelineCopy,
	})
	if err != nil {
		log.Error().Err(err).Msg("can't marshal JSON")
		return
	}
	server.Config.Services.Pubsub.Publish(message)
}

// AnnotateAutoscalerProvisioning sets AutoscalerProvisioning on pending workflows
// when a healthy autoscaler with pending agents can match the workflow's task labels.
func AnnotateAutoscalerProvisioning(s store.Store, pipeline *model.Pipeline) {
	if s == nil || pipeline == nil {
		return
	}

	hasPending := false
	for _, wf := range pipeline.Workflows {
		if wf.State == model.StatusPending {
			hasPending = true
			break
		}
	}
	if !hasPending {
		return
	}

	autoscalers, err := s.AutoscalerList(&model.AutoscalerListOptions{})
	if err != nil {
		log.Error().Err(err).Msg("failed to list autoscalers for provisioning annotation")
		return
	}

	// Filter to healthy autoscalers with pending agents
	var provisioning []*model.Autoscaler
	for _, a := range autoscalers {
		if a.IsHealthy() && a.PendingAgents > 0 {
			provisioning = append(provisioning, a)
		}
	}
	if len(provisioning) == 0 {
		return
	}

	tasks, err := s.TaskListPending()
	if err != nil {
		log.Error().Err(err).Msg("failed to list pending tasks for provisioning annotation")
		return
	}

	// Build a map from pipeline_id+name to task labels
	type taskKey struct {
		PipelineID int64
		Name       string
	}
	taskLabels := make(map[taskKey]map[string]string)
	for _, t := range tasks {
		taskLabels[taskKey{t.PipelineID, t.Name}] = t.Labels
	}

	for _, wf := range pipeline.Workflows {
		if wf.State != model.StatusPending {
			continue
		}
		labels := taskLabels[taskKey{pipeline.ID, wf.Name}]
		if labels == nil {
			continue
		}
		for _, a := range provisioning {
			if a.CanProvision(labels) {
				wf.AutoscalerProvisioning = true
				if a.IsProvisioningStale() {
					wf.AutoscalerProvisioningStale = true
				}
				break
			}
		}
	}
}

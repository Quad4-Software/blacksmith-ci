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

package grpc

import (
	"slices"
	"strings"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/pipeline"
	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/queue"
)

func createFilterFunc(agentFilter rpc.Filter, customLabels map[string]string) queue.FilterFn {
	return createFilterFuncWithPriority(agentFilter, customLabels, 0)
}

func createFilterFuncWithPriority(agentFilter rpc.Filter, customLabels map[string]string, priority int32) queue.FilterFn {
	return func(task *model.Task) (bool, int, int32) {
		score := 0

		logger := log.Debug().
			Str("task_id", task.ID).
			Interface("task_labels", task.Labels).
			Interface("agent_filter_labels", agentFilter.Labels).
			Interface("agent_custom_labels", customLabels).
			Int32("priority", priority)

		// Add agent identifiers if available
		if hostname, ok := agentFilter.Labels["hostname"]; ok {
			logger = logger.Str("agent_hostname", hostname)
		}
		if backend, ok := agentFilter.Labels["backend"]; ok {
			logger = logger.Str("agent_backend", backend)
		}
		if platform, ok := agentFilter.Labels["platform"]; ok {
			logger = logger.Str("agent_platform", platform)
		}

		logger.Msg("filter: checking task against agent")

		// Check all task labels
		for taskLabel, taskLabelValue := range task.Labels {
			// if a task label is empty it will be ignored
			if taskLabelValue == "" {
				continue
			}

			// Check if this is a custom label (defined on agent)
			// Custom labels: if task has a custom label, agent MUST have it with matching value.
			// The task value may list several accepted values (pipe-separated,
			// e.g. `mylabel: [a, b]` -> "a|b"); the agent matches any of them.
			if agentLabelValue, isCustom := customLabels[taskLabel]; isCustom {
				// Task has a custom label - agent must match one of the accepted values
				if !slices.Contains(strings.Split(taskLabelValue, "|"), agentLabelValue) {
					// Values don't match - task cannot be assigned to this agent
					log.Debug().
						Str("task_id", task.ID).
						Str("custom_label", taskLabel).
						Str("agent_value", agentLabelValue).
						Str("task_value", taskLabelValue).
						Msg("filter: REJECTED - custom label value mismatch")
					return false, 0, priority
				}
				// Values match - increase score
				log.Debug().
					Str("task_id", task.ID).
					Str("custom_label", taskLabel).
					Str("value", agentLabelValue).
					Msg("filter: custom label matched")
				score += 10
				continue
			}

			// Skip infrastructure labels (crowci/* prefix) - these are backend-specific metadata.
			// LabelOrgID and LabelRepoFullName are excluded: they're used for agent scoping
			// (org-/repo-scoped agents) and must be compared against the agent's filter labels.
			if strings.HasPrefix(taskLabel, "crowci/") &&
				taskLabel != pipeline.LabelOrgID &&
				taskLabel != pipeline.LabelRepoFullName {
				log.Debug().
					Str("task_id", task.ID).
					Str("label", taskLabel).
					Msg("filter: skipping infrastructure label")
				continue
			}

			// check if agent has this label
			agentLabelValue, ok := agentFilter.Labels[taskLabel]
			if !ok {
				logger := log.Debug().
					Str("task_id", task.ID).
					Str("task_label", taskLabel).
					Str("task_value", taskLabelValue)

				// Add agent identifiers to help debug which agent is rejecting
				if hostname, ok := agentFilter.Labels["hostname"]; ok {
					logger = logger.Str("agent_hostname", hostname)
				}
				if backend, ok := agentFilter.Labels["backend"]; ok {
					logger = logger.Str("agent_backend", backend)
				}

				logger.Msg("filter: REJECTED - agent missing required label")
				return false, 0, priority
			}

			// Special handling for org-id and repo labels - they support pipe-separated values
			// This allows agents to be configured to handle multiple orgs/repos
			// Support both legacy and new label names for backward compatibility
			if taskLabel == pipeline.LabelOrgID || taskLabel == "org-id" ||
				taskLabel == pipeline.LabelRepoFullName || taskLabel == "repo" {
				if agentLabelValue == "*" {
					// Wildcard match
					log.Debug().
						Str("task_id", task.ID).
						Str("label", taskLabel).
						Str("task_value", taskLabelValue).
						Msg("filter: wildcard match for org-id/repo")
					score++
					continue
				}

				// Split agent values by pipe separator
				agentValues := strings.Split(agentLabelValue, "|")

				// Check if task value matches any of the agent values
				if slices.Contains(agentValues, taskLabelValue) {
					log.Debug().
						Str("task_id", task.ID).
						Str("label", taskLabel).
						Str("task_value", taskLabelValue).
						Strs("agent_values", agentValues).
						Msg("filter: pipe-separated match for org-id/repo")
					score += 10
					continue
				}

				// No match found
				log.Debug().
					Str("task_id", task.ID).
					Str("label", taskLabel).
					Str("task_value", taskLabelValue).
					Strs("agent_values", agentValues).
					Msg("filter: REJECTED - no match in pipe-separated org-id/repo values")
				return false, 0, priority
			}

			// Standard matching for other labels.
			// The task value may list several accepted values (pipe-separated,
			// e.g. `agent: [a, b]` -> "a|b"); the agent matches if its own value
			// is any of them, or if the agent itself declares a wildcard.
			switch {
			// if agent label has a wildcard
			case agentLabelValue == "*":
				log.Debug().
					Str("task_id", task.ID).
					Str("label", taskLabel).
					Str("task_value", taskLabelValue).
					Msg("filter: wildcard match")
				score++
			// if the agent value is one of the accepted task values
			case slices.Contains(strings.Split(taskLabelValue, "|"), agentLabelValue):
				log.Debug().
					Str("task_id", task.ID).
					Str("label", taskLabel).
					Str("value", taskLabelValue).
					Msg("filter: exact match")
				score += 10
			// agent doesn't match
			default:
				log.Debug().
					Str("task_id", task.ID).
					Str("label", taskLabel).
					Str("task_value", taskLabelValue).
					Str("agent_value", agentLabelValue).
					Msg("filter: REJECTED - label value mismatch")
				return false, 0, priority
			}
		}
		// Return match status, label match score, and priority
		// Priority will be used separately for filtering
		log.Debug().
			Str("task_id", task.ID).
			Int("score", score).
			Int32("priority", priority).
			Msg("filter: ACCEPTED - task matches agent")
		return true, score, priority
	}
}

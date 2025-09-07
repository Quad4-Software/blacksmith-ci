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

	"codeberg.org/crowci/crow/v4/pipeline/rpc"
	"codeberg.org/crowci/crow/v4/server/model"
	"codeberg.org/crowci/crow/v4/server/queue"
)

func createFilterFunc(agentFilter rpc.Filter, customLabels map[string]string) queue.FilterFn {
	return createFilterFuncWithPriority(agentFilter, customLabels, 0)
}

func createFilterFuncWithPriority(agentFilter rpc.Filter, customLabels map[string]string, priority int32) queue.FilterFn {
	return func(task *model.Task) (bool, int, int32) {
		score := 0

		// First, check that the task has all required custom labels from the agent
		for customLabel, customLabelValue := range customLabels {
			taskValue, hasLabel := task.Labels[customLabel]
			// Task must have the custom label with exact match
			if !hasLabel || taskValue != customLabelValue {
				return false, 0, priority
			}
		}

		// Then check task labels as before
		for taskLabel, taskLabelValue := range task.Labels {
			// if a task label is empty it will be ignored
			if taskLabelValue == "" {
				continue
			}

			// Skip custom labels as they were already checked above
			if _, isCustom := customLabels[taskLabel]; isCustom {
				continue
			}

			// check if agent has this label
			agentLabelValue, ok := agentFilter.Labels[taskLabel]
			if !ok {
				return false, 0, priority
			}

			// Special handling for 'org-id' and 'repo' labels - partial matching
			if taskLabel == "org-id" || taskLabel == "repo" {
				// For these labels, we allow partial matching using pipe-separated values
				if agentLabelValue == "*" {
					// Wildcard match
					score++
					continue
				}

				// Split agent values by pipe separator
				agentValues := strings.Split(agentLabelValue, "|")

				// Check if task value matches any of the agent values
				matched := false
				if slices.Contains(agentValues, taskLabelValue) {
					matched = true
					score += 10
				}

				if !matched {
					return false, 0, priority
				}
			} else {
				// Standard matching for other labels
				switch agentLabelValue {
				// if agent label has a wildcard
				case "*":
					score++
				// if agent label has an exact match
				case taskLabelValue:
					score += 10
				// agent doesn't match
				default:
					return false, 0, priority
				}
			}
		}
		// Return match status, label match score, and priority
		// Priority will be used separately for filtering
		return true, score, priority
	}
}

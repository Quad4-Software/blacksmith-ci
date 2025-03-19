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

	"codeberg.org/crowci/crow/v3/pipeline/rpc"
	"codeberg.org/crowci/crow/v3/server/model"
	"codeberg.org/crowci/crow/v3/server/queue"
)

func createFilterFunc(agentFilter rpc.Filter) queue.FilterFn {
	return func(task *model.Task) (bool, int) {
		score := 0
		for taskLabel, taskLabelValue := range task.Labels {
			// if a task label is empty it will be ignored
			if taskLabelValue == "" {
				continue
			}

			// check if agent has this label
			agentLabelValue, ok := agentFilter.Labels[taskLabel]
			if !ok {
				return false, 0
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
					return false, 0
				}
			} else {
				// Standard matching for other labels
				switch {
				// if agent label has a wildcard
				case agentLabelValue == "*":
					score++
				// if agent label has an exact match
				case agentLabelValue == taskLabelValue:
					score += 10
				// agent doesn't match
				default:
					return false, 0
				}
			}
		}
		return true, score
	}
}

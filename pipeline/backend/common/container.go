// Copyright 2025 Crow Authors
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

package common

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// Container label constants for workflow discovery and adoption.
// Used by both Docker and Podman backends.
const (
	// LabelTaskUUID identifies which workflow/task a container belongs to.
	// Used by DiscoverWorkflows to find orphaned containers for adoption.
	LabelTaskUUID = "crowci/task-uuid"

	// LabelStepUUID identifies the specific step within a workflow.
	LabelStepUUID = "crowci/step-uuid"

	// LabelStepName is the human-readable step name.
	LabelStepName = "crowci/step"
)

// MaxContainerNameLength is the maximum length for container names.
const MaxContainerNameLength = 64

// ToContainerName generates a container name from step metadata.
// It uses a descriptive naming scheme by default unless explicitly set to "hash".
func ToContainerName(step *types.Step) string {
	scheme := step.ContainerNameScheme
	// Use descriptive naming by default unless explicitly set to "hash"
	if scheme != "hash" {
		owner := step.Environment["CI_REPO_OWNER"]
		repo := step.Environment["CI_REPO_NAME"]
		pipeline := step.Environment["CI_PIPELINE_NUMBER"]
		workflow := step.Environment["CI_WORKFLOW_NUMBER"]
		// Only use descriptive format if all required environment variables are present
		if owner != "" && repo != "" && pipeline != "" && step.Name != "" {
			var name string
			if workflow != "" {
				// Include workflow number (including "1" for first workflow)
				name = owner + "-" + repo + "-" + pipeline + "-" + workflow + "-" + step.Name
			} else {
				// Fallback naming when workflow number is not available
				name = owner + "-" + repo + "-" + pipeline + "-" + step.Name
			}
			return SanitizeAndTruncateName(name, MaxContainerNameLength)
		}
	}
	return "crow_" + step.UUID
}

// SanitizeAndTruncateName makes a name safe for Docker/Kubernetes/Podman and truncates to maxLen.
func SanitizeAndTruncateName(name string, maxLen int) string {
	// Lowercase, replace invalid chars with '-', collapse multiple '-'.
	name = strings.ToLower(name)
	name = regexp.MustCompile(`[^a-z0-9\-]`).ReplaceAllString(name, "-")
	name = regexp.MustCompile(`-+`).ReplaceAllString(name, "-")
	if utf8.RuneCountInString(name) > maxLen {
		runes := []rune(name)
		name = string(runes[:maxLen])
	}
	return strings.Trim(name, "-")
}

// MakeLabels creates the standard container labels for workflow discovery.
func MakeLabels(step *types.Step, taskUUID string) map[string]string {
	return map[string]string{
		// New labels for workflow discovery and adoption
		LabelTaskUUID: taskUUID,
		LabelStepUUID: step.UUID,
		LabelStepName: step.Name,
		// Legacy labels for backwards compatibility
		"wp_uuid": step.UUID,
		"wp_step": step.Name,
	}
}

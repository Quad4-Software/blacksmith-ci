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
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

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

// nameHashLength is the number of hex characters of the name hash appended
// to names that had to be truncated.
const nameHashLength = 8

var (
	invalidNameChars = regexp.MustCompile(`[^a-z0-9-]`)
	repeatedDashes   = regexp.MustCompile(`-+`)
)

// SanitizeAndTruncateName makes a name safe for Docker/Kubernetes/Podman and
// limits it to maxLen characters. The result only contains [a-z0-9-] and
// neither starts nor ends with '-'. Names that fit are returned unchanged;
// longer names are cut and suffixed with '-' plus a short hash of the full
// input, so names sharing a long prefix (e.g. two steps of a repo with a
// long owner/name) stay distinct.
func SanitizeAndTruncateName(name string, maxLen int) string {
	sanitized := strings.ToLower(name)
	sanitized = invalidNameChars.ReplaceAllString(sanitized, "-")
	sanitized = repeatedDashes.ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-")
	if len(sanitized) <= maxLen {
		return sanitized
	}

	sum := sha256.Sum256([]byte(name))
	suffix := hex.EncodeToString(sum[:])[:nameHashLength]
	if maxLen <= nameHashLength+1 {
		return suffix[:max(maxLen, 0)]
	}
	prefix := strings.TrimRight(sanitized[:maxLen-nameHashLength-1], "-")
	if prefix == "" {
		return suffix
	}
	return prefix + "-" + suffix
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

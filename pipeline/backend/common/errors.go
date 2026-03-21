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

import "strings"

// IsContainerNotFoundOrNotRunning checks if an error indicates that a container
// was not found or is not running. This handles error messages from Docker,
// Podman, and containerd.
func IsContainerNotFoundOrNotRunning(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "No such container") ||
		strings.Contains(errStr, "is not running") ||
		strings.Contains(errStr, "can only kill running containers") ||
		strings.Contains(errStr, "already in progress") ||
		strings.Contains(errStr, "no such container") ||
		strings.Contains(errStr, "container not found")
}

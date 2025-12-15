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

package backend

import (
	"codeberg.org/crowci/crow/v5/pipeline/backend/docker"
)

// ParseDockerMemory parses a memory string with optional units and returns bytes.
// This is a wrapper for docker.ParseMemory to make it accessible from the agent.
func ParseDockerMemory(s string) (int64, error) {
	return docker.ParseMemory(s)
}

// ParseDockerCPUQuota parses a CPU string and returns the quota in microseconds.
// This is a wrapper for docker.ParseCPUQuota to make it accessible from the agent.
func ParseDockerCPUQuota(s string) (int64, error) {
	return docker.ParseCPUQuota(s)
}

// Copyright 2026 Crow Authors
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

package docker

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultDockerEndpoint_PerOS(t *testing.T) {
	got := defaultDockerEndpoint()

	if runtime.GOOS == "windows" {
		assert.Equal(t, `\\.\pipe\docker_engine`, got,
			"Windows Docker daemon listens on a named pipe; the autoscaler-spawned Windows agent saw IsAvailable return false because the previous /var/run/docker.sock check has no meaning on Windows")
	} else {
		assert.Equal(t, "/var/run/docker.sock", got,
			"non-Windows agents continue using the Unix socket path that's been there forever")
	}
}

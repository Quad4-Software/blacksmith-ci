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

package podman

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/pipeline/backend/common"
)

func TestKeepaliveContainerName(t *testing.T) {
	name := keepaliveContainerName(strings.Repeat("abc123", 30))
	assert.LessOrEqual(t, len(name), common.MaxContainerNameLength)
	assert.True(t, strings.HasPrefix(name, "crow-podman-keepalive-"))
}

func TestKeepaliveIntervalSeconds(t *testing.T) {
	assert.Equal(t, 0, keepaliveIntervalSeconds(0))
	assert.Equal(t, 1, keepaliveIntervalSeconds(1*time.Millisecond))
	assert.Equal(t, 1, keepaliveIntervalSeconds(1*time.Second))
	assert.Equal(t, 90, keepaliveIntervalSeconds(90*time.Second))
	assert.Equal(t, 2, keepaliveIntervalSeconds(1500*time.Millisecond))
}

func TestKeepaliveScript(t *testing.T) {
	script := keepaliveScript("example.com", 30)
	assert.Equal(t, "while :; do nslookup example.com >/dev/null 2>&1 || true; sleep 30; done", script)
}

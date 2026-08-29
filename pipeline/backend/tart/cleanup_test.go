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

package tart

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStaleVMsIgnoresForeignVMs(t *testing.T) {
	entries := []vmEntry{
		{Source: "local", Name: "my-dev-box", State: "stopped"},
		{Source: "local", Name: "crow-abandoned", State: "stopped"},
		{Source: "oci", Name: "ghcr.io/cirruslabs/macos-tahoe-base:latest", State: "stopped"},
	}
	assert.Equal(t, []string{"crow-abandoned"}, staleVMs(entries, nil))
}

func TestStaleVMsSkipsActiveWorkflows(t *testing.T) {
	active := map[string]struct{}{"01HQZX7P8N4K2M9V6T3R5W8Y1A": {}}
	entries := []vmEntry{
		{Source: "local", Name: vmName("01HQZX7P8N4K2M9V6T3R5W8Y1A"), State: "running"},
		{Source: "local", Name: "crow-orphan", State: "running"},
	}
	// A VM belonging to a build this agent is currently running is never
	// reclaimed, however long it has been going.
	assert.Equal(t, []string{"crow-orphan"}, staleVMs(entries, active))
}

func TestStaleVMsEmpty(t *testing.T) {
	assert.Empty(t, staleVMs(nil, nil))
}

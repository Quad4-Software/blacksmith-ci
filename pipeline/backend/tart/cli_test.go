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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVMName(t *testing.T) {
	assert.Equal(t, "crow-01hqzx7p8n4k2m9v6t3r5w8y1a", vmName("01HQZX7P8N4K2M9V6T3R5W8Y1A"))
	// Anything a VM name cannot carry is replaced, so a non-ULID task id
	// still produces a usable name.
	assert.Equal(t, "crow-a-b-c", vmName("a/b c"))
}

func TestCloneArgs(t *testing.T) {
	assert.Equal(t, []string{"clone", "ghcr.io/example/img:latest", "crow-x"}, cloneArgs("ghcr.io/example/img:latest", "crow-x"))
}

func TestSetArgsOmitsUnsetValues(t *testing.T) {
	assert.Nil(t, setArgs("crow-x", 0, 0))
	assert.Equal(t, []string{"set", "crow-a", "--cpu", "4"}, setArgs("crow-a", 4, 0))
	assert.Equal(t, []string{"set", "crow-b", "--memory", "8192"}, setArgs("crow-b", 0, 8192))
	assert.Equal(t, []string{"set", "crow-c", "--cpu", "4", "--memory", "8192"}, setArgs("crow-c", 4, 8192))
}

func TestRunArgsPerNetworkMode(t *testing.T) {
	assert.Equal(t, []string{"run", "--no-graphics", "crow-x"}, runArgs("crow-x", networkShared))
	assert.Equal(t, []string{"run", "--no-graphics", "--net-softnet", "crow-x"}, runArgs("crow-x", networkSoftnet))
	assert.Equal(t, []string{"run", "--no-graphics", "--net-host", "crow-x"}, runArgs("crow-x", networkHost))
}

func TestExecArgsUsesStdin(t *testing.T) {
	// -i is what lets the step script arrive on stdin instead of argv, which
	// is what keeps step secrets out of the guest's process list.
	assert.Equal(t, []string{"exec", "-i", "crow-x", "/bin/sh", "-s"}, execArgs("crow-x", "/bin/sh"))
	assert.Equal(t, []string{"exec", "-i", "crow-x", "/bin/bash", "-s"}, execArgs("crow-x", "/bin/bash"))
}

func TestStopAndDeleteArgs(t *testing.T) {
	assert.Equal(t, []string{"stop", "crow-x", "--timeout", "30"}, stopArgs("crow-x", 30*time.Second))
	assert.Equal(t, []string{"delete", "crow-x"}, deleteArgs("crow-x"))
	assert.Equal(t, []string{"list", "--format", "json", "--source", "local"}, listArgs())
}

// The fixture is real output from tart 2.36.0, so the field names this
// depends on are pinned to what tart actually emits rather than to a guess.
func TestParseVMList(t *testing.T) {
	raw := []byte(`[
	  {
	    "Running" : false,
	    "Source" : "local",
	    "Disk" : 50,
	    "Name" : "crow-orphan",
	    "State" : "stopped",
	    "Accessed" : "2026-08-29T08:28:29Z",
	    "Size" : 33
	  },
	  {
	    "Running" : false,
	    "Disk" : 50,
	    "Name" : "my-dev-box",
	    "State" : "stopped",
	    "Accessed" : "2026-08-29T08:28:29Z",
	    "Size" : 33,
	    "Source" : "local"
	  }
	]`)
	entries, err := parseVMList(raw)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "crow-orphan", entries[0].Name)
	assert.Equal(t, "stopped", entries[0].State)
	assert.Equal(t, "local", entries[0].Source)

	// Only the crow- VM is this backend's to reclaim.
	assert.Equal(t, []string{"crow-orphan"}, staleVMs(entries, nil))
}

func TestParseVMListRejectsGarbage(t *testing.T) {
	_, err := parseVMList([]byte("not json"))
	assert.Error(t, err)
}

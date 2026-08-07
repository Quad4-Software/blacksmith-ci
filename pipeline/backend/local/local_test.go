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

package local

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

func TestSetupWorkflowRejectsTempVolumes(t *testing.T) {
	err := New().SetupWorkflow(context.Background(), &types.Config{
		TempVolumes: []string{"crow_tmp_dind"},
	}, "task-uuid")
	assert.ErrorContains(t, err, "does not support `temp_volumes`")
}

// With --backend-local-use-host-home the steps run with the account's own home,
// but nothing the backend writes on their behalf may land there. The clone step
// writes a .netrc and deletes it afterwards, so a real ~/.netrc would be
// overwritten and then destroyed.
func TestUseHostHomeKeepsWorkflowDirsPrivate(t *testing.T) {
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)

	e := &local{useHostHome: true, tempDir: t.TempDir()}
	if err := e.SetupWorkflow(context.Background(), &types.Config{}, "task-uuid"); err != nil {
		t.Fatalf("SetupWorkflow: %v", err)
	}

	state, err := e.getState("task-uuid")
	if err != nil {
		t.Fatalf("getState: %v", err)
	}

	if state.homeDir != hostHome {
		t.Errorf("homeDir = %q, want the account home %q", state.homeDir, hostHome)
	}
	if !strings.HasPrefix(state.privateDir, state.baseDir) {
		t.Errorf("privateDir = %q, want it under the workflow base %q", state.privateDir, state.baseDir)
	}
	if state.privateDir == state.homeDir {
		t.Error("privateDir must not be the account home; the clone step's netrc would overwrite a real one")
	}
	if _, err := os.Stat(state.privateDir); err != nil {
		t.Errorf("privateDir was not created: %v", err)
	}
}

// The default is unchanged: a per-workflow home, isolated from the account.
func TestDefaultHomeIsPerWorkflow(t *testing.T) {
	e := &local{tempDir: t.TempDir()}
	if err := e.SetupWorkflow(context.Background(), &types.Config{}, "task-uuid"); err != nil {
		t.Fatalf("SetupWorkflow: %v", err)
	}

	state, err := e.getState("task-uuid")
	if err != nil {
		t.Fatalf("getState: %v", err)
	}

	if !strings.HasPrefix(state.homeDir, state.baseDir) {
		t.Errorf("homeDir = %q, want it under the workflow base %q", state.homeDir, state.baseDir)
	}
	if state.homeDir != state.privateDir {
		t.Errorf("homeDir %q and privateDir %q should be the same directory by default", state.homeDir, state.privateDir)
	}
}

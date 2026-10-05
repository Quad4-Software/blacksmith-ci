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

package local

import (
	"context"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// A step inherits the agent's environment so that host toolchains keep working,
// and that environment holds the server-agent token. The server registers
// whoever presents it as a system agent and then hands it tasks from every
// repository on the instance, so a pipeline must never see it.
func TestStepEnvironDropsAgentConfiguration(t *testing.T) {
	t.Setenv("CROW_AGENT_SECRET", "super-secret-token")
	t.Setenv("CROW_SERVER", "grpc.example.com:443")
	t.Setenv("CROW_BACKEND_LOCAL_SANDBOX_LEVEL", "standard")
	t.Setenv("WOODPECKER_AGENT_SECRET", "legacy-secret-token")
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("CI_SOMETHING", "kept")

	env := stepEnviron()

	for _, entry := range env {
		assert.NotContains(t, entry, "super-secret-token", "the agent secret reached the step environment")
		assert.NotContains(t, entry, "legacy-secret-token", "the legacy agent secret reached the step environment")
		assert.False(t, strings.HasPrefix(entry, "CROW_"), "agent configuration %q reached the step environment", entry)
		assert.False(t, strings.HasPrefix(entry, "WOODPECKER_"), "agent configuration %q reached the step environment", entry)
	}

	// Everything else the host set is still there; a toolchain that cannot see
	// PATH is the reason people pick this backend in the first place.
	assert.Contains(t, env, "PATH=/usr/bin:/bin")
	assert.Contains(t, env, "CI_SOMETHING=kept")
}

func TestIsAgentEnv(t *testing.T) {
	tests := []struct {
		entry string
		want  bool
	}{
		{"CROW_AGENT_SECRET=x", true},
		{"WOODPECKER_AGENT_SECRET=x", true},
		{"CROW_SERVER=x", true},
		{"PATH=/usr/bin", false},
		{"CI_WORKSPACE=/tmp/ws", false},
		// The prefix carries the underscore, so unrelated names starting with
		// the same letters are left alone.
		{"CROWDED=x", false},
		{"MY_CROW_VAR=x", false},
		// os.Environ() is documented as "key=value" but a caller need not be.
		{"CROW_AGENT_SECRET", true},
		{"PATH", false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, isAgentEnv(tt.entry), "isAgentEnv(%q)", tt.entry)
	}
}

func TestAgentSecretFiles(t *testing.T) {
	t.Setenv("CROW_AGENT_SECRET_FILE", "/etc/crow/secret")
	t.Setenv("WOODPECKER_AGENT_SECRET_FILE", "")

	assert.Equal(t, []string{"/etc/crow/secret"}, agentSecretFiles())
}

// End to end over a real step: what the shell sees is what matters, not what
// the helper returns.
func TestStepProcessSeesNeitherSecretNorSharedTmpDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the step is driven through a POSIX shell")
	}

	t.Setenv("CROW_AGENT_SECRET", "super-secret-token")

	ctx := context.Background()
	engine := &local{tempDir: t.TempDir(), os: runtime.GOOS}

	const taskUUID = "task-uuid"
	if err := engine.SetupWorkflow(ctx, &types.Config{}, taskUUID); err != nil {
		t.Fatalf("SetupWorkflow: %v", err)
	}
	t.Cleanup(func() { _ = engine.DestroyWorkflow(ctx, &types.Config{}, taskUUID) })

	state, err := engine.getState(taskUUID)
	if err != nil {
		t.Fatalf("getState: %v", err)
	}

	step := &types.Step{
		UUID:     "step-uuid",
		Name:     "env",
		Type:     types.StepTypeCommands,
		Image:    "/bin/sh",
		Commands: []string{"env"},
	}
	if err := engine.StartStep(ctx, step, taskUUID); err != nil {
		t.Fatalf("StartStep: %v", err)
	}

	output, err := engine.TailStep(ctx, step, taskUUID)
	if err != nil {
		t.Fatalf("TailStep: %v", err)
	}
	logs, err := io.ReadAll(output)
	if err != nil {
		t.Fatalf("read step output: %v", err)
	}
	if _, err := engine.WaitStep(ctx, step, taskUUID); err != nil {
		t.Fatalf("WaitStep: %v", err)
	}

	assert.NotContains(t, string(logs), "super-secret-token", "the step printed the agent secret out of its own environment")
	// The variable being set, not the name appearing anywhere: steps also see
	// commit messages, as CI_COMMIT_MESSAGE and CI_PREV_COMMIT_MESSAGE, which
	// may well mention it.
	assert.NotRegexp(t, `(?m)^\x01?CROW_AGENT_SECRET=`, string(logs), "the step inherited the agent secret")
	assert.Contains(t, string(logs), "TMPDIR="+state.tmpDir, "the step did not get the workflow's own TMPDIR")
}

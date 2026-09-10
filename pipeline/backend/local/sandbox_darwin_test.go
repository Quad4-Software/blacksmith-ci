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

//go:build darwin

package local

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

func TestGenerateSandboxProfile(t *testing.T) {
	state := &workflowState{
		baseDir:      "/tmp/crow-test-base",
		workspaceDir: "/tmp/crow-test-base/workspace",
		homeDir:      "/tmp/crow-test-base/home",
	}

	tests := []struct {
		name         string
		level        sandboxLevel
		wantContains []string
		wantNotEmpty bool
	}{
		{
			name:         "none level returns empty profile",
			level:        sandboxLevelNone,
			wantNotEmpty: false,
		},
		{
			name:  "standard level includes network access",
			level: sandboxLevelStandard,
			wantContains: []string{
				"(version 1)",
				"(deny default)",
				"(allow network*)",
				"(allow file-read*)",
				// Allocating a pty on macOS opens the multiplexer, so a profile
				// without it fails as "can't get Master/Slave device" even
				// though it lists the pty devices. Anything driving a child
				// process through a pty hits this.
				`(literal "/dev/ptmx")`,
				state.workspaceDir,
				state.baseDir,
			},
			wantNotEmpty: true,
		},
		{
			name:  "strict level denies network",
			level: sandboxLevelStrict,
			wantContains: []string{
				"(version 1)",
				"(deny default)",
				"(deny network*)",
				state.workspaceDir,
				state.baseDir,
			},
			wantNotEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := generateSandboxProfile(tt.level, state)

			if tt.wantNotEmpty && profile == "" {
				t.Errorf("expected non-empty profile for level %s", tt.level)
			}

			if !tt.wantNotEmpty && profile != "" {
				t.Errorf("expected empty profile for level %s, got: %s", tt.level, profile)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(profile, want) {
					t.Errorf("profile does not contain expected string: %q", want)
				}
			}
		})
	}
}

func TestWrapCommandWithSandbox(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	state := &workflowState{
		baseDir:      tempDir,
		workspaceDir: filepath.Join(tempDir, "workspace"),
		homeDir:      filepath.Join(tempDir, "home"),
		stepCMDs:     make(map[string]*exec.Cmd),
	}

	tests := []struct {
		name             string
		sandboxLevel     sandboxLevel
		expectWrapped    bool
		checkSandboxExec bool
	}{
		{
			name:          "none level does not wrap",
			sandboxLevel:  sandboxLevelNone,
			expectWrapped: false,
		},
		{
			name:             "standard level wraps if sandbox-exec exists",
			sandboxLevel:     sandboxLevelStandard,
			expectWrapped:    true,
			checkSandboxExec: true,
		},
		{
			name:             "strict level wraps if sandbox-exec exists",
			sandboxLevel:     sandboxLevelStrict,
			expectWrapped:    true,
			checkSandboxExec: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check if sandbox-exec is available
			if tt.checkSandboxExec {
				if _, err := exec.LookPath("sandbox-exec"); err != nil {
					t.Skip("sandbox-exec not available on this system")
				}
			}

			e := &local{
				sandboxLevel: tt.sandboxLevel,
				os:           "darwin",
			}

			ctx := context.Background()
			originalCmd := exec.CommandContext(ctx, "/bin/echo", "test")
			originalCmd.Dir = state.workspaceDir

			wrappedCmd, err := e.wrapCommandWithSandbox(ctx, originalCmd, state)
			if err != nil {
				t.Fatalf("wrapCommandWithSandbox failed: %v", err)
			}

			if tt.expectWrapped {
				// Should be wrapped with sandbox-exec
				if wrappedCmd.Path != originalCmd.Path {
					// Command was wrapped, check if it's sandbox-exec
					if !strings.Contains(wrappedCmd.Path, "sandbox-exec") {
						t.Errorf("expected command to be wrapped with sandbox-exec, got: %s", wrappedCmd.Path)
					}
				}
			} else {
				// Should be the same command
				if wrappedCmd.Path != originalCmd.Path {
					t.Errorf("expected command to not be wrapped, but path changed from %s to %s",
						originalCmd.Path, wrappedCmd.Path)
				}
			}
		})
	}
}

// A subpath is a literal, not a regex. Escaping it with regexp.QuoteMeta, as
// this used to, silently cost a workflow access to its own directories as soon
// as a path held a "." -- an agent account at /Users/some.one was enough.
func TestSandboxProfilePathsAreLiteral(t *testing.T) {
	state := &workflowState{
		baseDir:      "/tmp/crow test [special].v2",
		workspaceDir: "/tmp/crow test [special].v2/workspace",
		homeDir:      "/tmp/crow test [special].v2/home",
	}

	profile := generateStandardProfile(state)

	for _, dir := range []string{state.baseDir, state.workspaceDir, state.homeDir} {
		if !strings.Contains(profile, `(subpath "`+dir+`")`) {
			t.Errorf("profile does not allow %q verbatim", dir)
		}
	}
	if strings.Contains(profile, `crow test \[special\].v2`) || strings.Contains(profile, `\.v2`) {
		t.Error("profile still escapes subpath literals as if they were regexes")
	}
}

// macOS reaches /tmp and /var through symlinks into /private, and the sandbox
// matches the resolved path, so a rule naming only the unresolved one never
// fires.
func TestSandboxPathsCoverThePrivateForm(t *testing.T) {
	paths := sandboxPaths("/tmp/crow-local-1")
	if !slices.Contains(paths, "/tmp/crow-local-1") {
		t.Errorf("sandboxPaths dropped the original path: %v", paths)
	}
	if !slices.Contains(paths, "/private/tmp/crow-local-1") {
		t.Errorf("sandboxPaths did not add the /private form: %v", paths)
	}

	// An existing path resolves for real, and nothing is emitted twice.
	real := t.TempDir()
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	got := sandboxPaths(real, real)
	if !slices.Contains(got, resolved) {
		t.Errorf("sandboxPaths(%q) = %v, missing the resolved form %q", real, got, resolved)
	}
	if len(got) != len(slices.Compact(slices.Sorted(slices.Values(got)))) {
		t.Errorf("sandboxPaths emitted duplicates: %v", got)
	}
}

// The standard profile used to allow file* across `/var/folders/.*/T/` and
// `^/tmp/crow-local-`, which is every concurrent workflow's base directory on
// the host, not just the caller's.
func TestStandardProfileScopesTempAccessToTheWorkflow(t *testing.T) {
	state := &workflowState{
		baseDir:      "/tmp/crow-local-mine",
		workspaceDir: "/tmp/crow-local-mine/workspace",
		homeDir:      "/tmp/crow-local-mine/home",
	}

	profile := generateStandardProfile(state)

	for _, blanket := range []string{
		`(regex #"^/var/folders/.*/T/")`,
		`(regex #"^/private/var/folders/.*/T/")`,
	} {
		if strings.Contains(profile, blanket) {
			t.Errorf("profile still grants every workflow's temp dir via %s", blanket)
		}
	}

	// The crow-local pattern survives, but only to deny the sibling workflows.
	siblingDeny := strings.Index(profile, ";; Deny reading the other workflows")
	if siblingDeny < 0 {
		t.Fatal("the sibling workflow deny is missing")
	}
	if idx := strings.Index(profile, `(regex #"^/tmp/`+workflowDirPrefix+`")`); idx >= 0 && idx < siblingDeny {
		t.Error("the crow-local pattern is still granted rather than denied")
	}

	if !strings.Contains(profile, `(subpath "/tmp/crow-local-mine")`) {
		t.Error("profile does not grant the workflow its own base directory")
	}

	// A deny is only overridden by a later rule naming the same operation
	// class, so the workflow's own directories have to come back as file-read*.
	reallow := strings.Index(profile, "(allow file-read*\n")
	if reallow < siblingDeny {
		t.Error("the workflow's own directories are not allowed back after the sibling deny")
	}
}

// The token the agent authenticates with is enough to register as a system
// agent and be handed other repositories' tasks, so a step must not be able to
// read it back off the host.
func TestStandardProfileDeniesAgentCredentials(t *testing.T) {
	state := &workflowState{
		baseDir:      "/tmp/crow-local-mine",
		workspaceDir: "/tmp/crow-local-mine/workspace",
		homeDir:      "/tmp/crow-local-mine/home",
		secretFiles:  []string{"/etc/crow/agent-secret"},
	}

	profile := generateStandardProfile(state)

	for _, want := range []string{
		`(subpath "/Library/LaunchDaemons")`,
		`(subpath "/Library/LaunchAgents")`,
		`(literal "/etc/crow/agent-secret")`,
	} {
		if !strings.Contains(profile, want) {
			t.Errorf("profile does not deny %s", want)
		}
	}

	// Writing them matters too: an installed LaunchAgent outlives the workflow.
	if !strings.Contains(profile, "(deny file*\n\t(subpath \"/Library/LaunchDaemons\")") {
		t.Error("the launchd job directories are only read-denied, so a step can still install one")
	}

	// A deny only wins if nothing below it matches the same operation.
	denyIdx := strings.Index(profile, ";; Deny reading the agent's own credentials.")
	if denyIdx < 0 {
		t.Fatal("the credential deny block is missing")
	}
	if lastAllow := strings.LastIndex(profile, "(allow "); lastAllow > denyIdx {
		t.Errorf("an allow at %d comes after the credential deny at %d and overrides it", lastAllow, denyIdx)
	}
}

// Without a secret file configured the profile must not grow an empty rule.
func TestStandardProfileWithoutSecretFileStillCompiles(t *testing.T) {
	state := &workflowState{
		baseDir:      "/tmp/crow-local-mine",
		workspaceDir: "/tmp/crow-local-mine/workspace",
		homeDir:      "/tmp/crow-local-mine/home",
	}

	profile := generateStandardProfile(state)

	if strings.Contains(profile, `(literal "")`) {
		t.Error("profile contains an empty literal rule")
	}
	assertProfileCompiles(t, profile)
}

// runSandboxed runs a shell snippet under the workflow's own sandbox profile.
func runSandboxed(t *testing.T, engine *local, state *workflowState, script string) ([]byte, error) {
	t.Helper()

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", script)
	cmd.Dir = state.workspaceDir
	cmd.Env = append(stepEnviron(), "HOME="+state.homeDir, "TMPDIR="+state.tmpDir)

	wrapped, err := engine.wrapCommandWithSandbox(ctx, cmd, state)
	if err != nil {
		t.Fatalf("wrapCommandWithSandbox: %v", err)
	}
	if wrapped == cmd {
		t.Fatal("command was not wrapped, so this proves nothing")
	}

	return wrapped.CombinedOutput()
}

// assertProfileCompiles checks that sandbox-exec accepts the profile at all: a
// malformed one is otherwise only discovered when a real step fails to start.
func assertProfileCompiles(t *testing.T, profile string) {
	t.Helper()

	if _, err := exec.LookPath("sandbox-exec"); err != nil {
		t.Skip("sandbox-exec not available on this system")
	}

	out, err := exec.Command("sandbox-exec", "-p", profile, "/usr/bin/true").CombinedOutput()
	if err != nil {
		t.Errorf("sandbox-exec rejected the profile: %v\n%s", err, out)
	}
}

// The file* deny on Documents and ~/.ssh/id_* never stopped a read, because
// the blanket "(allow file-read*)" above it names a more specific operation
// class and therefore wins. /Users/Shared is a real, writable account
// directory, so it exercises the rule without touching anybody's home.
func TestStandardSandboxDeniesReadingSensitiveUserDirs(t *testing.T) {
	if _, err := exec.LookPath("sandbox-exec"); err != nil {
		t.Skip("sandbox-exec not available on this system")
	}

	sensitive := "/Users/Shared/Documents"
	if err := os.MkdirAll(sensitive, 0o755); err != nil {
		t.Skipf("cannot use %s on this host: %v", sensitive, err)
	}
	secret := filepath.Join(sensitive, "crow-sandbox-test")
	if err := os.WriteFile(secret, []byte("private-document"), 0o600); err != nil {
		t.Skipf("cannot write to %s: %v", sensitive, err)
	}
	t.Cleanup(func() { _ = os.Remove(secret) })

	ctx := context.Background()
	engine := &local{tempDir: t.TempDir(), os: "darwin", sandboxLevel: sandboxLevelStandard}
	if err := engine.SetupWorkflow(ctx, &types.Config{}, "task"); err != nil {
		t.Fatalf("SetupWorkflow: %v", err)
	}
	t.Cleanup(func() { _ = engine.DestroyWorkflow(ctx, &types.Config{}, "task") })
	state, err := engine.getState("task")
	if err != nil {
		t.Fatalf("getState: %v", err)
	}

	out, err := runSandboxed(t, engine, state, "cat "+secret)
	if err == nil || strings.Contains(string(out), "private-document") {
		t.Errorf("read a sensitive user directory: %v\n%s", err, out)
	}
}

// With --backend-local-use-host-home the account's own home is allowed, and its
// LaunchAgents directory is a foothold that outlives the workflow installing it.
func TestStandardSandboxBlocksLaunchAgentPersistence(t *testing.T) {
	if _, err := exec.LookPath("sandbox-exec"); err != nil {
		t.Skip("sandbox-exec not available on this system")
	}

	home, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(home, "/Users/") {
		t.Skipf("no account home under /Users to test with: %v", err)
	}

	ctx := context.Background()
	engine := &local{tempDir: t.TempDir(), os: "darwin", sandboxLevel: sandboxLevelStandard, useHostHome: true}
	if err := engine.SetupWorkflow(ctx, &types.Config{}, "task"); err != nil {
		t.Fatalf("SetupWorkflow: %v", err)
	}
	t.Cleanup(func() { _ = engine.DestroyWorkflow(ctx, &types.Config{}, "task") })
	state, err := engine.getState("task")
	if err != nil {
		t.Fatalf("getState: %v", err)
	}

	planted := filepath.Join(home, "Library", "LaunchAgents", "crow-sandbox-test.plist")
	t.Cleanup(func() { _ = os.Remove(planted) })

	out, err := runSandboxed(t, engine, state, "mkdir -p ~/Library/LaunchAgents && echo plist > "+planted)
	if err == nil {
		t.Errorf("installed a launch agent in the account home: %s", out)
	}
	if _, err := os.Stat(planted); err == nil {
		t.Error("the planted launch agent exists")
	}
}

// The property the whole change is about: two workflows on one host cannot
// reach each other, and neither can read the agent's credentials.
func TestStandardSandboxIsolatesConcurrentWorkflows(t *testing.T) {
	if _, err := exec.LookPath("sandbox-exec"); err != nil {
		t.Skip("sandbox-exec not available on this system")
	}

	ctx := context.Background()
	engine := &local{tempDir: t.TempDir(), os: "darwin", sandboxLevel: sandboxLevelStandard}

	setup := func(taskUUID string) *workflowState {
		t.Helper()
		if err := engine.SetupWorkflow(ctx, &types.Config{}, taskUUID); err != nil {
			t.Fatalf("SetupWorkflow(%s): %v", taskUUID, err)
		}
		t.Cleanup(func() { _ = engine.DestroyWorkflow(ctx, &types.Config{}, taskUUID) })
		state, err := engine.getState(taskUUID)
		if err != nil {
			t.Fatalf("getState(%s): %v", taskUUID, err)
		}
		return state
	}

	attacker := setup("attacker")
	victim := setup("victim")

	victimSecret := filepath.Join(victim.workspaceDir, "deploy-key")
	if err := os.WriteFile(victimSecret, []byte("victim-only"), 0o600); err != nil {
		t.Fatalf("seed the other workflow: %v", err)
	}

	t.Run("its own workspace and tmpdir work", func(t *testing.T) {
		out, err := runSandboxed(t, engine, attacker, `echo hello > ./out.txt && cat ./out.txt && echo tmp > "$TMPDIR/t" && cat "$TMPDIR/t"`)
		if err != nil {
			t.Fatalf("a workflow cannot use its own directories: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "hello") || !strings.Contains(string(out), "tmp") {
			t.Errorf("unexpected output: %s", out)
		}
	})

	t.Run("cannot read another workflow", func(t *testing.T) {
		out, err := runSandboxed(t, engine, attacker, "cat "+victimSecret)
		if err == nil || strings.Contains(string(out), "victim-only") {
			t.Errorf("read another workflow's file: %v\n%s", err, out)
		}
	})

	t.Run("cannot write into another workflow", func(t *testing.T) {
		planted := filepath.Join(victim.workspaceDir, "planted.sh")
		out, err := runSandboxed(t, engine, attacker, "echo pwned > "+planted)
		if err == nil {
			t.Errorf("wrote into another workflow's workspace: %s", out)
		}
		if _, err := os.Stat(planted); err == nil {
			t.Error("the planted file exists in the other workflow's workspace")
		}
	})

	t.Run("cannot read the agent launchd job", func(t *testing.T) {
		out, err := runSandboxed(t, engine, attacker, "cat /Library/LaunchDaemons/*.plist")
		if err == nil {
			t.Errorf("read the launchd jobs, which carry CROW_AGENT_SECRET in the documented setup: %s", out)
		}
	})
}

// The deny for Documents, Desktop and the like has to come after every allow,
// because a sandbox profile resolves an operation with the last rule that
// matches it. With --backend-local-use-host-home the allowed home is itself
// under /Users, so an earlier deny would be overridden by it.
func TestSensitiveUserDirDenyComesLast(t *testing.T) {
	hostHome := "/Users/someone"
	state := &workflowState{
		baseDir:      "/tmp/crow-test-base",
		workspaceDir: "/tmp/crow-test-base/workspace",
		homeDir:      hostHome,
		privateDir:   "/tmp/crow-test-base/home",
		hostHome:     true,
	}

	profile := generateSandboxProfile(sandboxLevelStandard, state)

	denyIdx := strings.Index(profile, `(regex #"^/Users/[^/]+/Documents")`)
	if denyIdx < 0 {
		t.Fatal("the sensitive user directory deny is missing from the standard profile")
	}

	allowIdx := strings.LastIndex(profile, `(subpath "`+hostHome+`")`)
	if allowIdx < 0 {
		t.Fatalf("the host home %q is not allowed in the standard profile", hostHome)
	}

	if denyIdx < allowIdx {
		t.Errorf("the sensitive user directory deny is at %d, before the home allow at %d, so the allow overrides it", denyIdx, allowIdx)
	}

	// Writes matter as much as reads once the allowed home is a real one, and
	// each operation class needs its own rule: "last rule wins" holds only
	// between rules naming the same class, so a file* deny alone is overridden
	// for reads by the more specific blanket "(allow file-read*)" at the top.
	if !strings.Contains(profile, ";; Deny sensitive user directories.") {
		t.Error("expected the sensitive deny block to be present")
	}
	for _, class := range []string{"file-read*", "file*"} {
		block := "(deny " + class + "\n\t(regex #\"^/Users/[^/]+/Documents\")"
		if !strings.Contains(profile, block) {
			t.Errorf("the sensitive user directories are not denied for %s", class)
		}
	}
}

// macOS account paths need not match the launchd job's HOME and TMPDIR.
func TestStandardSandboxWithOverriddenAccountEnvironment(t *testing.T) {
	accountHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	// The probe must live in the account cache, outside the workflow temp root.
	//nolint:usetesting // t.TempDir cannot select this parent directory.
	cacheDir, err := os.MkdirTemp(filepath.Join(accountHome, "Library", "Caches"), "crow-account-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cacheDir) })
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TMPDIR", t.TempDir())
	engine := &local{tempDir: t.TempDir(), os: "darwin", sandboxLevel: sandboxLevelStandard}
	ctx := context.Background()
	if err := engine.SetupWorkflow(ctx, &types.Config{}, "task"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.DestroyWorkflow(ctx, &types.Config{}, "task") })
	state, err := engine.getState("task")
	if err != nil {
		t.Fatal(err)
	}
	for name, script := range map[string]string{
		"account temp":       `dir=$(/usr/bin/getconf DARWIN_USER_TEMP_DIR) && file=$(/usr/bin/mktemp "${dir}crow-account-test.XXXXXX") && rm "$file"`,
		"account cache":      `touch "` + filepath.Join(cacheDir, "probe") + `"`,
		"Apple git launcher": `/usr/bin/git --version`,
	} {
		t.Run(name, func(t *testing.T) {
			out, err := runSandboxed(t, engine, state, script)
			if err != nil || strings.Contains(string(out), "couldn't create cache file") {
				t.Fatalf("%v: %s", err, out)
			}
		})
	}
}

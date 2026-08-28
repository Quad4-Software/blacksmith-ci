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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// sandboxLevel defines the security level for sandbox-exec profiles.
type sandboxLevel string

const (
	sandboxLevelNone     sandboxLevel = "none"     // No sandboxing (default for backward compatibility)
	sandboxLevelStandard sandboxLevel = "standard" // Balanced security for CI/CD workloads
	sandboxLevelStrict   sandboxLevel = "strict"   // Maximum security, minimal permissions
)

// sandboxLiteral renders a path as a sandbox profile string literal.
//
// A profile string is not a regular expression. Escaping one with
// regexp.QuoteMeta, as this used to, turns a path holding a "." or a "[" into a
// literal that matches nothing, and the workflow silently loses access to its
// own directories: an agent account at /Users/some.one was enough.
func sandboxLiteral(path string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(path)
}

// sandboxPaths expands the paths a rule has to cover into every form the kernel
// may present them in, dropping empties and duplicates.
//
// The kernel resolves /tmp and /var through symlinks into /private and matches
// the resolved path against the profile's literal, so a rule naming only the
// unresolved path never fires.
func sandboxPaths(paths ...string) []string {
	var expanded []string
	seen := make(map[string]struct{})

	add := func(path string) {
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		expanded = append(expanded, path)
	}

	for _, path := range paths {
		add(path)
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			add(resolved)
			continue
		}
		// The path need not exist yet, and a rule that was dropped because of
		// that would fail later as a denied access. Resolve the two symlinks
		// macOS is known to have by hand instead.
		if strings.HasPrefix(path, "/var/") || strings.HasPrefix(path, "/tmp/") {
			add("/private" + path)
		}
	}

	return expanded
}

// sandboxRules renders one `(kind "path")` line per path, indented to sit
// inside a rule block.
func sandboxRules(kind string, paths []string) string {
	rules := make([]string, 0, len(paths))
	for _, path := range paths {
		rules = append(rules, fmt.Sprintf("\t(%s \"%s\")", kind, sandboxLiteral(path)))
	}
	return strings.Join(rules, "\n")
}

// sandboxSiblingDeny renders rules denying every workflow directory under the
// given temp roots, the caller's own included -- the block that follows it in
// the profile allows that one back.
//
// This is a regex and not a subpath because the sibling directories are not
// known when the profile is built, and because denying the temp dir wholesale
// would take away temp files that belong to the rest of the system.
func sandboxSiblingDeny(baseDir string, extraRoots ...string) string {
	roots := append([]string{filepath.Dir(baseDir)}, extraRoots...)
	rules := make([]string, 0, len(roots))

	for _, root := range sandboxPaths(roots...) {
		rules = append(rules, fmt.Sprintf("\t(regex #\"^%s/%s\")", regexp.QuoteMeta(root), workflowDirPrefix))
	}

	return strings.Join(rules, "\n")
}

// sandboxSharedTempPaths returns the account's own temp directory.
//
// Steps get a TMPDIR of their own inside the workflow, but macOS resolves the
// per-user temp dir through confstr(_CS_DARWIN_USER_TEMP_DIR) and several tools
// use that instead of the environment: xcrun writes its lookup cache there and
// "mktemp -t" creates its file there, both of which fail outright when the
// directory is not writable. The workflow directories that live inside it are
// denied separately, so this shares scratch space and not the checkouts.
func sandboxSharedTempPaths() []string {
	return []string{os.TempDir()}
}

// sandboxCachePaths returns the build tool caches shared between workflows.
//
// They sit outside the workflow deliberately: a cold cache costs a repeatedly
// built project more than the isolation is worth on a runner whose workflows
// trust each other. They are also why `standard` is not a boundary between
// workflows that do not, which the documentation says in as many words.
func sandboxCachePaths() []string {
	paths := []string{"/Library/Caches"}

	// Tools that ask the account for its home rather than reading $HOME write
	// here even when the workflow has a home of its own.
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		for _, dir := range []string{".cargo", ".rustup", ".cache", ".npm", "Library/Caches"} {
			paths = append(paths, filepath.Join(home, dir))
		}
	}

	return paths
}

// generateSandboxProfile creates a sandbox-exec profile based on the security level and workflow state.
func generateSandboxProfile(level sandboxLevel, state *workflowState) string {
	switch level {
	case sandboxLevelStrict:
		return generateStrictProfile(state)
	case sandboxLevelStandard:
		return generateStandardProfile(state)
	default:
		return ""
	}
}

// generateStandardProfile creates a balanced security profile suitable for most CI/CD workloads.
// This profile:
// - Allows network access (for package downloads, API calls, etc.)
// - Allows reading system libraries and tools
// - Restricts writing to the workflow's own directories and the shared build caches
// - Denies access to sensitive user directories and to the agent's own credentials.
func generateStandardProfile(state *workflowState) string {
	// Only this workflow's directories, never a pattern covering the temp dir
	// as a whole. `/var/folders/.*/T/` and `^/tmp/crow-local-` used to be
	// allowed outright, which let any workflow read and write every concurrent
	// workflow's checkout, home and secrets on the same host.
	workflowDirs := sandboxPaths(state.baseDir, state.workspaceDir, state.homeDir)

	// Every workflow directory sharing a temp root with this one, this one
	// included; the block after the deny allows this one back. Rendered into
	// both operation classes, like the sensitive directories below.
	siblingDeny := sandboxSiblingDeny(state.baseDir, sandboxSharedTempPaths()...)

	// Rendered into both a file-read* and a file* deny below.
	sensitiveUserDirs := strings.Join([]string{
		`	(regex #"^/Users/[^/]+/Documents")`,
		`	(regex #"^/Users/[^/]+/Desktop")`,
		`	(regex #"^/Users/[^/]+/Pictures")`,
		`	(regex #"^/Users/[^/]+/Downloads")`,
		`	(regex #"^/Users/[^/]+/\.ssh/id_")`,
	}, "\n")

	// The launchd job directories are denied unconditionally; a secret file is
	// only denied when the agent was told to read one.
	credentialPaths := []string{
		`	(subpath "/Library/LaunchDaemons")`,
		`	(subpath "/Library/LaunchAgents")`,
		`	(subpath "/System/Library/LaunchDaemons")`,
		`	(regex #"^/Users/[^/]+/Library/LaunchAgents")`,
	}
	if rules := sandboxRules("literal", sandboxPaths(state.secretFiles...)); rules != "" {
		credentialPaths = append(credentialPaths, rules)
	}
	credentialDenies := strings.Join(credentialPaths, "\n")

	return fmt.Sprintf(`(version 1)
(deny default)

;; Allow all file read operations (metadata and data)
(allow file-read*)

;; Allow access to device files (ignore dtracehelper denials)
(allow file-read* file-write* file-ioctl
	(literal "/dev/null")
	(literal "/dev/zero")
	(literal "/dev/random")
	(literal "/dev/urandom")
	(regex "^/dev/tty")
	(regex "^/dev/pty")
	;; The pty multiplexer, which is how a pty is actually allocated on macOS:
	;; posix_openpt(3) and openpty(3) both open it. "^/dev/pty" only matches the
	;; legacy BSD /dev/ptyXX devices, so without this, allocating a pty fails
	;; with "can't get Master/Slave device" while the profile looks like it
	;; permits ptys. The slave side comes back as /dev/ttysNNN, which "^/dev/tty"
	;; already covers.
	(literal "/dev/ptmx")
	(regex "^/dev/dtracehelper"))

;; Allow network access (required for package managers, API calls)
(allow network*)

;; Deny reading sensitive system files
(deny file-read*
	(literal "/etc/passwd")
	(literal "/etc/shadow")
	(literal "/etc/master.passwd")
	(literal "/etc/sudoers")
	(regex #"^/etc/sudoers\.d/")
	(literal "/private/etc/passwd")
	(literal "/private/etc/shadow")
	(literal "/private/etc/master.passwd")
	(literal "/private/etc/sudoers")
	(regex #"^/private/etc/sudoers\.d/")
	(regex #"^/var/db/dslocal/nodes/Default/users/")
	(regex #"^/private/var/db/dslocal/nodes/Default/users/"))

;; Allow all file operations in this workflow's own directories, which is where
;; TMPDIR points too (including extended attributes)
(allow file*
%s)

;; Allow extended attributes operations (needed for tar, rsync, etc.)
(allow file-read-xattr file-write-xattr
%s)

;; Allow build tool cache directories (Cargo, npm, pip, go, etc.), shared
;; between workflows
(allow file*
%s)

;; Allow the account's own temp directory, which parts of the toolchain use
;; instead of TMPDIR. The workflow directories inside it are denied further
;; down, so this shares scratch space and not anybody's checkout.
(allow file*
%s)

;; Allow process management.
;;
;; Execution is deliberately not restricted to an allowlist of directories. One
;; used to be listed here and did nothing, because the "(allow process*)" below
;; matched afterwards and a profile resolves an operation with the last rule
;; that matches. Reinstating it would not buy anything either: a step may write
;; into its workspace and execute from it, so anything it can download it can
;; run. Restricting execution is the container backend's job.
(allow process*)
(allow signal)
(allow sysctl*)
(allow system-socket)

;; Allow IPC
(allow ipc*)
(allow mach*)

;; Deny reading the other workflows that share this host's temp directory.
;;
;; The blanket "(allow file-read*)" at the top is what makes a host toolchain
;; usable, and it also handed every workflow every other workflow's checkout,
;; per-workflow home, and the .netrc the clone step writes there with the forge
;; credentials in it. Writes were confined above; reads have to be taken back
;; explicitly.
(deny file-read*
%s)

(deny file*
%s)

;; This workflow's own directories again, because the denies above cover them too.
;;
;; Twice, because a rule only overrides an earlier one naming the same operation
;; class: a single "(allow file*)" would leave the file-read* deny standing and
;; the workflow would lose access to its own checkout.
(allow file-read*
%s)

(allow file*
%s)

;; Deny sensitive user directories.
;;
;; Last, because a sandbox profile resolves an operation with the final rule
;; that matches it. These used to sit above the allow blocks, which was harmless
;; only for as long as nothing below them allowed a path under /Users. With
;; --backend-local-use-host-home the home directory allowed below IS under
;; /Users, and an earlier deny would have been overridden by it.
;;
;; Twice, once per operation class. "Last rule wins" only holds between rules
;; naming the same class, and the blanket "(allow file-read*)" at the top is the
;; more specific of the two: with only the file* deny below, as this had, reads
;; of Documents and of ~/.ssh/id_* went through and just the writes were
;; stopped.
(deny file-read*
%s)

(deny file*
%s)

;; Deny reading the agent's own credentials.
;;
;; A step no longer inherits CROW_AGENT_SECRET in its environment, but the token
;; is still on disk: in whatever CROW_AGENT_SECRET_FILE points at, and in the
;; launchd job that starts the agent, whose plist is world readable and in the
;; documented setup carries the secret inline. Anyone holding that token can
;; register as an agent and be handed other repositories' tasks, so a pipeline
;; must not be able to read it back off the host that runs it.
(deny file-read*
%s)

;; And not write them either: with --backend-local-use-host-home the account's
;; own LaunchAgents directory sits inside the home allowed above, which is a
;; job that survives the workflow that installed it.
(deny file*
%s)
`,
		sandboxRules("subpath", workflowDirs),
		sandboxRules("subpath", workflowDirs),
		sandboxRules("subpath", sandboxPaths(sandboxCachePaths()...)),
		sandboxRules("subpath", sandboxPaths(sandboxSharedTempPaths()...)),
		siblingDeny,
		siblingDeny,
		sandboxRules("subpath", workflowDirs),
		sandboxRules("subpath", workflowDirs),
		sensitiveUserDirs,
		sensitiveUserDirs,
		credentialDenies,
		credentialDenies,
	)
}

// generateStrictProfile creates a maximum security profile with minimal permissions.
// This profile:
// - Denies network access by default
// - Only allows reading necessary system files
// - Strictly limits file writes to workflow directory
// - Denies access to all user directories.
func generateStrictProfile(state *workflowState) string {
	workflowDirs := sandboxPaths(state.baseDir, state.workspaceDir, state.homeDir)

	return fmt.Sprintf(`(version 1)
(deny default)

;; Deny all network access
(deny network*)

;; Allow reading only essential system files
(allow file-read-data
	(subpath "/usr/lib")
	(subpath "/usr/share")
	(literal "/usr/bin")
	(literal "/bin")
	(regex "^/dev/(null|zero|random|urandom|stdin|stdout|stderr)$"))

;; Deny all user directory access
(deny file-read-data
	(regex "^/Users/"))

;; Allow full access only to workflow directories
(allow file-read* file-write*
%s)

;; Allow minimal process execution
(allow process-exec
	(subpath "/usr/bin")
	(subpath "/bin")
%s)

;; Allow basic process management
(allow process-fork)
(allow signal)
`,
		sandboxRules("subpath", workflowDirs),
		sandboxRules("subpath", sandboxPaths(state.baseDir)),
	)
}

// wrapCommandWithSandbox wraps a command with sandbox-exec if sandboxing is enabled.
func (e *local) wrapCommandWithSandbox(ctx context.Context, originalCmd *exec.Cmd, state *workflowState) (*exec.Cmd, error) {
	if e.sandboxLevel == sandboxLevelNone {
		log.Trace().Msg("sandbox disabled, running command without sandbox-exec")
		return originalCmd, nil
	}

	// Check if sandbox-exec is available
	sandboxExecPath, err := exec.LookPath("sandbox-exec")
	if err != nil {
		log.Warn().Err(err).Msg("sandbox-exec not found, falling back to unsandboxed execution")
		return originalCmd, nil
	}

	// Generate the sandbox profile
	profile := generateSandboxProfile(e.sandboxLevel, state)
	if profile == "" {
		return originalCmd, nil
	}

	log.Debug().
		Str("level", string(e.sandboxLevel)).
		Str("command", originalCmd.Path).
		Str("profile", profile).
		Msg("wrapping command with sandbox-exec")

	// Build the new command arguments
	// sandbox-exec -p <profile> <command> <args...>
	args := []string{"-p", profile, originalCmd.Path}
	args = append(args, originalCmd.Args[1:]...) // Skip the first arg (command path)

	// Create new command with sandbox-exec
	cmd := exec.CommandContext(ctx, sandboxExecPath, args...)
	cmd.Dir = originalCmd.Dir
	cmd.Env = originalCmd.Env
	cmd.Stdin = originalCmd.Stdin
	// Only copy Stdout/Stderr if they were explicitly set
	// (don't copy nil values, which would prevent StdoutPipe/StderrPipe from working)
	if originalCmd.Stdout != nil {
		cmd.Stdout = originalCmd.Stdout
	}
	if originalCmd.Stderr != nil {
		cmd.Stderr = originalCmd.Stderr
	}

	return cmd, nil
}

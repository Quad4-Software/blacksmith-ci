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
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"al.essio.dev/pkg/shellescape"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/pipeline/backend/common/clone"
	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// cloneBinaryName is what the clone plugin is called, on the host cache and on
// the guest's PATH alike.
const cloneBinaryName = "plugin-clone"

// cloneHeredocTag terminates the heredoc that carries the clone binary into the
// guest. It only has to be a string the base64 alphabet cannot produce.
const cloneHeredocTag = "EOF_CROW_CLONE"

// netrcHeredocTag terminates the heredoc that carries the netrc into the guest.
const netrcHeredocTag = "EOF_CROW_NETRC"

// cloneStepScript prepares the clone step and returns the script to run.
//
// The binary is fetched and cached on the host, then streamed into the guest,
// rather than downloaded inside each guest. A throwaway VM caches nothing, so a
// guest-side download would cost a fresh ~50 MB transfer every workflow and
// would make the clone step depend on guest network access.
func (e *tart) cloneStepScript(ctx context.Context, step *types.Step, state *workflowState) (string, error) {
	if !strings.Contains(step.Image, "clone") {
		log.Warn().Msgf("the clone step image '%s' does not match the default clone image. Ignoring and using the default clone image anyway.", step.Image)
	}

	clonePath, err := e.ensureGuestCloneBinary(ctx, state)
	if err != nil {
		return "", err
	}

	var netrc string
	if step.Environment["CI_NETRC_MACHINE"] != "" {
		netrc = clone.GenNetRC(step.Environment)
	}

	return cloneScript(cloneEnv(step.Environment, state.paths), state.paths, clonePath, netrc), nil
}

// cloneEnv is the clone step's environment.
//
// It is the usual step environment plus the two things only a clone needs: the
// CI_NETRC_* family, which every other step has stripped, and the prompt guards
// that stop git blocking forever on a headless agent when a credential is wrong.
func cloneEnv(stepEnvironment map[string]string, paths guestPaths) map[string]string {
	env := stepEnv(&types.Step{Environment: stepEnvironment}, paths)
	env["GIT_TERMINAL_PROMPT"] = "0"
	env["GCM_INTERACTIVE"] = "Never"
	for _, k := range []string{"CI_NETRC_MACHINE", "CI_NETRC_USERNAME", "CI_NETRC_PASSWORD"} {
		if v := stepEnvironment[k]; v != "" {
			env[k] = v
		}
	}
	return env
}

// ensureGuestCloneBinary makes the clone plugin available in the guest and
// returns its guest path. An image that already ships one is used as-is.
func (e *tart) ensureGuestCloneBinary(ctx context.Context, state *workflowState) (string, error) {
	probe := fmt.Sprintf("command -v %s\n", cloneBinaryName)
	if out, err := e.execGuest(ctx, state.vm, probe); err == nil {
		if guestPath := strings.TrimSpace(string(out)); guestPath != "" {
			log.Debug().Str("path", guestPath).Msg("using the clone plugin shipped with the VM image")
			return guestPath, nil
		}
	}

	hostPath, err := e.hostCloneBinary(ctx)
	if err != nil {
		return "", err
	}

	binary, err := os.ReadFile(hostPath) //nolint:gosec // path is the agent's own cache, not user input
	if err != nil {
		return "", fmt.Errorf("read cached clone plugin: %w", err)
	}

	guestPath := path.Join(state.paths.Base, cloneBinaryName)
	script := pushBinaryScript(guestPath, base64.StdEncoding.EncodeToString(binary))

	if out, err := e.execGuest(ctx, state.vm, script); err != nil {
		return "", fmt.Errorf("push the clone plugin into the guest: %w: %s", err, out)
	}

	return guestPath, nil
}

// hostCloneBinary returns the path of the host's cached clone plugin,
// downloading it once if it is not there yet.
//
// The cache is what makes the host-side fetch worth doing: it is shared by
// every workflow this agent runs, while each guest is discarded.
func (e *tart) hostCloneBinary(ctx context.Context) (string, error) {
	cacheDir := filepath.Join(os.TempDir(), "crow-tart-clone")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return "", err
	}
	dest := filepath.Join(cacheDir, cloneBinaryName)
	return dest, clone.EnsureBinary(ctx, clone.EnsureBinaryConfig{
		Dest: dest,
		// The guest is always macOS on the same architecture as the host,
		// because that is the only thing Virtualization.framework runs.
		OS:   "darwin",
		Arch: runtime.GOARCH,
	})
}

// pushBinaryScript writes base64-encoded bytes into dest and marks it
// executable.
//
// A heredoc carries the payload rather than an argument, because the binary is
// tens of megabytes and would not survive an argv limit.
func pushBinaryScript(dest, encoded string) string {
	quoted := shellescape.Quote(dest)
	return fmt.Sprintf("set -e\nmkdir -p %s\nbase64 -d > %s <<'%s'\n%s\n%s\nchmod +x %s\n",
		shellescape.Quote(path.Dir(dest)), quoted, cloneHeredocTag,
		encoded, cloneHeredocTag, quoted)
}

// cloneScript runs the clone plugin with a netrc that is removed afterwards,
// whatever the clone's outcome, while still reporting the clone's exit code.
func cloneScript(env map[string]string, paths guestPaths, clonePath, netrc string) string {
	var b strings.Builder
	b.WriteString("set -e\n")

	for _, k := range sortedKeys(env) {
		fmt.Fprintf(&b, "export %s=%s\n", k, shellescape.Quote(env[k]))
	}

	fmt.Fprintf(&b, "mkdir -p %s\ncd %s\n",
		shellescape.Quote(paths.Workspace), shellescape.Quote(paths.Workspace))

	netrcPath := shellescape.Quote(path.Join(paths.Home, ".netrc"))
	if netrc != "" {
		fmt.Fprintf(&b, "cat > %s <<'%s'\n%s\n%s\nchmod 600 %s\n",
			netrcPath, netrcHeredocTag, netrc, netrcHeredocTag, netrcPath)
	}

	// set -e must not end the script before the netrc is removed, so the
	// clone's status is captured rather than allowed to abort.
	fmt.Fprintf(&b, "set +e\n%s\ncode=$?\nset -e\n", shellescape.Quote(clonePath))
	if netrc != "" {
		fmt.Fprintf(&b, "rm -f %s\n", netrcPath)
	}
	b.WriteString("exit $code\n")

	return b.String()
}

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

package hyperv

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

// parseFlags runs the backend's own flag set over argv, so the tests exercise
// the defaults and env-var wiring rather than a hand-built config struct.
func parseFlags(t *testing.T, args ...string) (config, error) {
	t.Helper()

	var (
		cfg    config
		cfgErr error
	)
	cmd := &cli.Command{
		Name:  "test",
		Flags: Flags,
		Action: func(_ context.Context, c *cli.Command) error {
			cfg, cfgErr = configFromCli(c)
			return nil
		},
	}
	err := cmd.Run(t.Context(), append([]string{"test"}, args...))
	assert.NoError(t, err)
	return cfg, cfgErr
}

func TestConfigRequiresABaseDisk(t *testing.T) {
	_, err := parseFlags(t, "--backend-hyperv-guest-username", "crow", "--backend-hyperv-guest-password", "pw")
	assert.ErrorIs(t, err, ErrBaseDiskNotConfigured)
}

func TestConfigRequiresGuestCredentials(t *testing.T) {
	_, err := parseFlags(t, "--backend-hyperv-base-vhdx", `C:\g.vhdx`)
	assert.ErrorIs(t, err, ErrGuestUserNotConfigured)

	_, err = parseFlags(t, "--backend-hyperv-base-vhdx", `C:\g.vhdx`, "--backend-hyperv-guest-username", "crow")
	assert.ErrorIs(t, err, ErrGuestUserNotConfigured)
}

func TestConfigDefaults(t *testing.T) {
	cfg, err := parseFlags(t,
		"--backend-hyperv-base-vhdx", `C:\g.vhdx`,
		"--backend-hyperv-guest-username", "crow",
		"--backend-hyperv-guest-password", "pw",
	)
	assert.NoError(t, err)

	// Two is the Windows Server Standard entitlement. Defaulting to unlimited
	// would quietly put a Standard host out of compliance.
	assert.Equal(t, defaultMaxVMs, cfg.MaxVMs)
	assert.Equal(t, defaultGuestBase, cfg.GuestBase)
	assert.Equal(t, 2, cfg.Generation)
	assert.Equal(t, defaultStartupTimeout, cfg.StartupTimeout)
	// No switch by default, so a guest starts with no network adapter at all.
	assert.Empty(t, cfg.SwitchName)
	assert.Zero(t, cfg.CPU)
	assert.Zero(t, cfg.MemoryMB)
}

func TestConfigParsesMemory(t *testing.T) {
	cfg, err := parseFlags(t,
		"--backend-hyperv-base-vhdx", `C:\g.vhdx`,
		"--backend-hyperv-guest-username", "crow",
		"--backend-hyperv-guest-password", "pw",
		"--backend-hyperv-memory", "8Gi",
	)
	assert.NoError(t, err)
	assert.Equal(t, 8192, cfg.MemoryMB)
}

func TestConfigRejectsBadValues(t *testing.T) {
	base := []string{
		"--backend-hyperv-base-vhdx", `C:\g.vhdx`,
		"--backend-hyperv-guest-username", "crow",
		"--backend-hyperv-guest-password", "pw",
	}

	_, err := parseFlags(t, append(base, "--backend-hyperv-generation", "3")...)
	assert.ErrorContains(t, err, "expected 1 or 2")

	_, err = parseFlags(t, append(base, "--backend-hyperv-max-vms", "-1")...)
	assert.ErrorContains(t, err, "expected 0 or more")

	_, err = parseFlags(t, append(base, "--backend-hyperv-memory", "not-a-size")...)
	assert.ErrorContains(t, err, "backend-hyperv-memory")
}

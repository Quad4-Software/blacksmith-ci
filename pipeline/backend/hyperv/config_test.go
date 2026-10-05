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
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

// parseFlags runs the backend's own flag set over argv, so the tests exercise
// the defaults and env-var wiring rather than a hand-built config struct.
func parseFlags(t *testing.T, args ...string) (config, error) {
	t.Helper()
	clearHypervEnv(t)

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

// clearHypervEnv unsets every backend flag's environment variable for the
// duration of the test. The flags read them, so on a host configured for the
// backend, such as the one test-hyperv runs on, they would otherwise fill in
// what a test deliberately leaves out.
func clearHypervEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(strings.ToUpper(name), "CROW_BACKEND_HYPERV_") {
			// t.Setenv records the old value and restores it afterwards.
			t.Setenv(name, "")
			assert.NoError(t, os.Unsetenv(name))
		}
	}
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
	// Once there is one, the cloud metadata service is out of reach.
	assert.Contains(t, cfg.BlockedNetworks, "169.254.0.0/16")
	assert.Contains(t, cfg.BlockedNetworks, "168.63.129.16/32")
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

	_, err = parseFlags(t, append(base, "--backend-hyperv-blocked-networks", "10.0.0.0")...)
	assert.ErrorContains(t, err, "backend-hyperv-blocked-networks")
}

func TestConfigBlockedNetworksCanBeTurnedOff(t *testing.T) {
	cfg, err := parseFlags(t,
		"--backend-hyperv-base-vhdx", `C:\g.vhdx`,
		"--backend-hyperv-guest-username", "crow",
		"--backend-hyperv-guest-password", "pw",
		"--backend-hyperv-blocked-networks", "none",
	)
	assert.NoError(t, err)
	assert.Empty(t, cfg.BlockedNetworks)
}

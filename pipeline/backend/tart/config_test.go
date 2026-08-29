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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// runFlags parses argv against the backend's own flag set and hands the
// resulting command to fn, which is how the agent supplies configuration.
func runFlags(t *testing.T, args []string, fn func(*cli.Command)) {
	t.Helper()
	cmd := &cli.Command{
		Flags:  Flags,
		Action: func(_ context.Context, c *cli.Command) error { fn(c); return nil },
	}
	require.NoError(t, cmd.Run(context.Background(), append([]string{"test"}, args...)))
}

func TestConfigFromCliDefaults(t *testing.T) {
	runFlags(t, []string{"--backend-tart-image", "ghcr.io/example/img:latest"}, func(c *cli.Command) {
		cfg, err := configFromCli(c)
		require.NoError(t, err)
		assert.Equal(t, "tart", cfg.Binary)
		assert.Equal(t, "ghcr.io/example/img:latest", cfg.Image)
		assert.False(t, cfg.AllowImageFromStep)
		assert.Equal(t, 0, cfg.CPU)
		assert.Equal(t, 0, cfg.MemoryMB)
		assert.Equal(t, networkShared, cfg.Network)
		assert.Equal(t, 5*time.Minute, cfg.StartupTimeout)
		assert.Equal(t, 30*time.Second, cfg.StopTimeout)
	})
}

func TestConfigFromCliRequiresImage(t *testing.T) {
	runFlags(t, nil, func(c *cli.Command) {
		_, err := configFromCli(c)
		assert.ErrorIs(t, err, ErrImageNotConfigured)
	})
}

func TestConfigFromCliParsesMemory(t *testing.T) {
	runFlags(t, []string{
		"--backend-tart-image", "img",
		"--backend-tart-memory", "8Gi",
		"--backend-tart-cpu", "6",
	}, func(c *cli.Command) {
		cfg, err := configFromCli(c)
		require.NoError(t, err)
		assert.Equal(t, 8192, cfg.MemoryMB)
		assert.Equal(t, 6, cfg.CPU)
	})
}

func TestConfigFromCliRejectsUnknownNetwork(t *testing.T) {
	runFlags(t, []string{"--backend-tart-image", "img", "--backend-tart-network", "bridged"}, func(c *cli.Command) {
		_, err := configFromCli(c)
		assert.ErrorContains(t, err, "bridged")
	})
}

func TestConfigFromCliAcceptsKnownNetworks(t *testing.T) {
	for _, mode := range []string{"shared", "softnet", "host"} {
		runFlags(t, []string{"--backend-tart-image", "img", "--backend-tart-network", mode}, func(c *cli.Command) {
			cfg, err := configFromCli(c)
			require.NoError(t, err)
			assert.Equal(t, networkMode(mode), cfg.Network)
		})
	}
}

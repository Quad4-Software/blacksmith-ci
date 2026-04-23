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

package podman

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestConfigFromCliNetworkKeepalive(t *testing.T) {
	runConfig := func(t *testing.T, args []string) (config, error) {
		t.Helper()

		var (
			cfg config
			err error
		)

		cmd := &cli.Command{
			Flags: Flags,
			Action: func(_ context.Context, c *cli.Command) error {
				cfg, err = configFromCli(c)
				return nil
			},
		}

		argv := append([]string{"test"}, args...)
		assert.NoError(t, cmd.Run(t.Context(), argv))
		return cfg, err
	}

	t.Run("disabled by default", func(t *testing.T) {
		cfg, err := runConfig(t, nil)
		assert.NoError(t, err)
		assert.Zero(t, cfg.networkKeepaliveInterval)
		assert.Equal(t, keepaliveDefaultImage, cfg.networkKeepaliveImage)
		assert.Equal(t, keepaliveDefaultHost, cfg.networkKeepaliveHost)
	})

	t.Run("enabled with valid config", func(t *testing.T) {
		cfg, err := runConfig(t, []string{
			"--backend-podman-network-keepalive-interval", "45s",
			"--backend-podman-network-keepalive-host", "one.one.one.one",
		})
		assert.NoError(t, err)
		assert.Equal(t, 45*time.Second, cfg.networkKeepaliveInterval)
		assert.Equal(t, keepaliveDefaultImage, cfg.networkKeepaliveImage)
		assert.Equal(t, "one.one.one.one", cfg.networkKeepaliveHost)
	})

	t.Run("rejects short interval", func(t *testing.T) {
		_, err := runConfig(t, []string{
			"--backend-podman-network-keepalive-interval", "500ms",
		})
		assert.ErrorContains(t, err, "at least 1s")
	})

	t.Run("rejects invalid host", func(t *testing.T) {
		_, err := runConfig(t, []string{
			"--backend-podman-network-keepalive-interval", "30s",
			"--backend-podman-network-keepalive-host", "example.com;rm -rf /",
		})
		assert.ErrorContains(t, err, "invalid network keepalive host")
	})

	t.Run("rejects hostname-like option value", func(t *testing.T) {
		_, err := runConfig(t, []string{
			"--backend-podman-network-keepalive-interval", "30s",
			"--backend-podman-network-keepalive-host", "-debug",
		})
		assert.ErrorContains(t, err, "invalid network keepalive host")
	})
}

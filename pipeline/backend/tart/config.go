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
	"fmt"
	"time"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/pipeline/backend/common"
)

// networkMode selects how the guest reaches the network.
type networkMode string

const (
	// Tart's own default: NAT behind the host.
	networkShared networkMode = "shared"
	// Adds a userspace packet filter that stops a guest spoofing
	// addresses or reaching other guests and the local network.
	networkSoftnet networkMode = "softnet"
	// Restricts the guest to a host-only network.
	networkHost networkMode = "host"
)

// bytesPerMebibyte converts common.ParseMemory's bytes into the megabytes
// 'tart set --memory' expects.
const bytesPerMebibyte = 1024 * 1024

// config is the agent-level configuration of the backend.
type config struct {
	Binary   string
	TartHome string
	Image    string
	// AllowImageFromStep decides whether backend_options.tart.image is
	// honored. Off by default: without it any repository could make the
	// host pull and boot an arbitrary OCI image.
	AllowImageFromStep bool
	// CPU and MemoryMB are both the default given to a VM and the ceiling a
	// step may request. Zero means the image's own setting is kept.
	CPU            int
	MemoryMB       int
	Network        networkMode
	StartupTimeout time.Duration
	StopTimeout    time.Duration
}

func configFromCli(c *cli.Command) (config, error) {
	cfg := config{
		Binary:             c.String("backend-tart-binary"),
		TartHome:           c.String("backend-tart-home"),
		Image:              c.String("backend-tart-image"),
		AllowImageFromStep: c.Bool("backend-tart-image-allow-from-step"),
		CPU:                c.Int("backend-tart-cpu"),
		Network:            networkMode(c.String("backend-tart-network")),
		StartupTimeout:     c.Duration("backend-tart-startup-timeout"),
		StopTimeout:        c.Duration("backend-tart-stop-timeout"),
	}

	if cfg.Image == "" {
		return config{}, ErrImageNotConfigured
	}

	memBytes, err := common.ParseMemory(c.String("backend-tart-memory"))
	if err != nil {
		return config{}, fmt.Errorf("backend-tart-memory: %w", err)
	}
	cfg.MemoryMB = int(memBytes / bytesPerMebibyte)

	switch cfg.Network {
	case networkShared, networkSoftnet, networkHost:
	default:
		return config{}, fmt.Errorf("backend-tart-network: unknown mode %q, expected shared, softnet or host", cfg.Network)
	}

	return cfg, nil
}

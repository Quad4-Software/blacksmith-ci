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
	"fmt"
	"net/netip"
	"time"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/pipeline/backend/common"
)

// bytesPerMebibyte converts common.ParseMemory's bytes into the megabytes the
// Hyper-V cmdlets are given.
const bytesPerMebibyte = 1024 * 1024

// config is the agent-level configuration of the backend.
type config struct {
	// BaseVHDX is the golden disk every workflow's VM is a differencing
	// child of. It is never written to.
	BaseVHDX string
	// VMPath is where the per-workflow differencing disks are created.
	VMPath string
	// GuestBase is the directory inside the guest that holds everything a
	// workflow writes.
	GuestBase string
	// GuestUsername and GuestPassword authenticate PowerShell Direct to the
	// guest. They belong to a local account baked into the golden image, not
	// to anything on the host.
	GuestUsername string
	GuestPassword string
	// SwitchName is the virtual switch guests are attached to. Empty gives
	// the guest no network adapter at all.
	SwitchName string
	// BlockedNetworks are CIDR ranges a guest on the switch cannot reach,
	// enforced by port ACLs on its adapter.
	BlockedNetworks []string
	// CPU and MemoryMB are what each VM is created with. Zero keeps
	// Hyper-V's own default.
	CPU      int
	MemoryMB int
	// MaxVMs caps how many VMs this backend will have alive at once. It
	// exists because the cap is a licensing question rather than a capacity
	// one: Windows Server Standard entitles a host to two running guests.
	MaxVMs int
	// Generation is the Hyper-V VM generation. 2 is right for anything from
	// Windows Server 2012 R2 on; 1 exists for older golden images.
	Generation     int
	StartupTimeout time.Duration
	Binary         string
}

func configFromCli(c *cli.Command) (config, error) {
	cfg := config{
		BaseVHDX:       c.String("backend-hyperv-base-vhdx"),
		VMPath:         c.String("backend-hyperv-vm-path"),
		GuestBase:      c.String("backend-hyperv-guest-base"),
		GuestUsername:  c.String("backend-hyperv-guest-username"),
		GuestPassword:  c.String("backend-hyperv-guest-password"),
		SwitchName:     c.String("backend-hyperv-switch"),
		CPU:            c.Int("backend-hyperv-cpu"),
		MaxVMs:         c.Int("backend-hyperv-max-vms"),
		Generation:     c.Int("backend-hyperv-generation"),
		StartupTimeout: c.Duration("backend-hyperv-startup-timeout"),
		Binary:         c.String("backend-hyperv-powershell"),
	}

	if cfg.BaseVHDX == "" {
		return config{}, ErrBaseDiskNotConfigured
	}
	if cfg.GuestUsername == "" || cfg.GuestPassword == "" {
		return config{}, ErrGuestUserNotConfigured
	}

	memBytes, err := common.ParseMemory(c.String("backend-hyperv-memory"))
	if err != nil {
		return config{}, fmt.Errorf("backend-hyperv-memory: %w", err)
	}
	cfg.MemoryMB = int(memBytes / bytesPerMebibyte)

	switch cfg.Generation {
	case generationLegacy, defaultGeneration:
	default:
		return config{}, fmt.Errorf("backend-hyperv-generation: expected 1 or 2, got %d", cfg.Generation)
	}

	for _, network := range c.StringSlice("backend-hyperv-blocked-networks") {
		// "none" is how the default is turned off: the CLI treats an empty
		// variable as unset, which would keep the default.
		if network == "" || network == blockNoNetworks {
			continue
		}
		// Rejected here rather than by Hyper-V, which would fail every
		// workflow's VM creation instead of the agent's start.
		if _, err := netip.ParsePrefix(network); err != nil {
			return config{}, fmt.Errorf("backend-hyperv-blocked-networks: %w", err)
		}
		cfg.BlockedNetworks = append(cfg.BlockedNetworks, network)
	}

	if cfg.MaxVMs < 0 {
		return config{}, fmt.Errorf("backend-hyperv-max-vms: expected 0 or more, got %d", cfg.MaxVMs)
	}

	return cfg, nil
}

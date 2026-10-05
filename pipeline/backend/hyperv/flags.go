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
	"time"

	"github.com/urfave/cli/v3"
)

const (
	// A Windows guest booting from a differencing disk is far slower than the
	// macOS VMs the tart backend clones, and the first boot of a golden image
	// on a cold host page cache is slower again.
	defaultStartupTimeout = 10 * time.Minute
	// Two is what Windows Server Standard entitles a licensed host to run.
	// Datacenter allows unlimited guests, so a Datacenter operator raises
	// this deliberately; defaulting the other way would quietly put a
	// Standard host out of compliance.
	defaultMaxVMs = 2
	// The guest layout lives off the system drive rather than a profile
	// directory so it does not depend on which account the golden image was
	// built with.
	defaultGuestBase = `C:\crow`
	// Generation 2 is UEFI with a SCSI boot disk, which is what every Windows
	// from Server 2012 R2 on installs onto. Generation 1 exists only for a
	// golden image older than that.
	generationLegacy  = 1
	defaultGeneration = 2
)

// Flags contains the CLI flags for the Hyper-V backend.
// The value blockNoNetworks in CROW_BACKEND_HYPERV_BLOCKED_NETWORKS turns the
// default off.
const blockNoNetworks = "none"

// defaultBlockedNetworks are what a guest on a switch is kept from by default.
//
// A guest reaches the outside through the host, so without these it reaches
// what the host can. On an Azure host behind a NAT switch that was the wire
// server and every service on the host's own network address, though not the
// metadata service. Link-local still covers the metadata service of every
// major cloud in case another setup forwards it, 168.63.129.16 is Azure's wire
// server, and the two private ranges keep a guest off the cloud network or LAN
// the host sits on. 192.168.0.0/16 is left out because that is where a NAT
// switch's own subnet and DHCP server usually are.
var defaultBlockedNetworks = []string{
	"169.254.0.0/16",
	"168.63.129.16/32",
	"10.0.0.0/8",
	"172.16.0.0/12",
}

var Flags = []cli.Flag{
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_HYPERV_BASE_VHDX"),
		Name:    "backend-hyperv-base-vhdx",
		Usage:   "path to the golden VHDX each workflow's VM is a differencing child of",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_HYPERV_VM_PATH"),
		Name:    "backend-hyperv-vm-path",
		Usage:   "directory the per-workflow differencing disks are created in; empty uses the host's Hyper-V default",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_HYPERV_GUEST_USERNAME"),
		Name:    "backend-hyperv-guest-username",
		Usage:   "local account in the golden image that PowerShell Direct authenticates as",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_HYPERV_GUEST_PASSWORD"),
		Name:    "backend-hyperv-guest-password",
		Usage:   "password for the guest account; the VM is disposable and unreachable from the network, so this is not a host credential",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_HYPERV_GUEST_BASE"),
		Name:    "backend-hyperv-guest-base",
		Usage:   "directory inside the guest holding the workspace, temp and home of a workflow",
		Value:   defaultGuestBase,
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_HYPERV_SWITCH"),
		Name:    "backend-hyperv-switch",
		Usage:   "virtual switch guests are attached to; empty gives the guest no network adapter at all",
	},
	&cli.StringSliceFlag{
		Sources: cli.EnvVars("CROW_BACKEND_HYPERV_BLOCKED_NETWORKS"),
		Name:    "backend-hyperv-blocked-networks",
		Usage:   "CIDR ranges a guest on a switch cannot reach, enforced by the switch; 'none' blocks nothing",
		Value:   defaultBlockedNetworks,
	},
	&cli.IntFlag{
		Sources: cli.EnvVars("CROW_BACKEND_HYPERV_CPU"),
		Name:    "backend-hyperv-cpu",
		Usage:   "vCPUs given to each VM; 0 keeps Hyper-V's default",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_HYPERV_MEMORY"),
		Name:    "backend-hyperv-memory",
		Usage:   "memory given to each VM (e.g. 8Gi, 4096Mi); empty keeps Hyper-V's default",
	},
	&cli.IntFlag{
		Sources: cli.EnvVars("CROW_BACKEND_HYPERV_MAX_VMS"),
		Name:    "backend-hyperv-max-vms",
		Usage:   "how many VMs may be alive at once; 0 means unlimited (Windows Server Datacenter)",
		Value:   defaultMaxVMs,
	},
	&cli.IntFlag{
		Sources: cli.EnvVars("CROW_BACKEND_HYPERV_GENERATION"),
		Name:    "backend-hyperv-generation",
		Usage:   "Hyper-V VM generation, 1 or 2",
		Value:   defaultGeneration,
	},
	&cli.DurationFlag{
		Sources: cli.EnvVars("CROW_BACKEND_HYPERV_STARTUP_TIMEOUT"),
		Name:    "backend-hyperv-startup-timeout",
		Usage:   "how long to wait for a booted VM to answer PowerShell Direct",
		Value:   defaultStartupTimeout,
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_HYPERV_POWERSHELL"),
		Name:    "backend-hyperv-powershell",
		Usage:   "PowerShell executable used to drive Hyper-V on the host",
		Value:   "powershell.exe",
	},
}

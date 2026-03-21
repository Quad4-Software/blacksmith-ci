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
	"github.com/urfave/cli/v3"
)

// Flags contains the CLI flags for the Podman backend.
var Flags = []cli.Flag{
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_PODMAN_HOST"),
		Name:    "backend-podman-host",
		Usage:   "path to podman socket or url to the podman server (auto-detects rootless/rootful if not set)",
	},
	&cli.BoolFlag{
		Sources: cli.EnvVars("CROW_BACKEND_PODMAN_ENABLE_IPV6"),
		Name:    "backend-podman-ipv6",
		Usage:   "enable IPv6 in podman networks",
		Value:   false,
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_PODMAN_NETWORK"),
		Name:    "backend-podman-network",
		Usage:   "podman network attached to all pipeline steps",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_PODMAN_VOLUMES"),
		Name:    "backend-podman-volumes",
		Usage:   "additional volumes to mount (comma-separated)",
	},
	//
	// resource limit parameters
	//
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_PODMAN_LIMIT_MEM_SWAP"),
		Name:    "backend-podman-limit-mem-swap",
		Usage:   "maximum memory used for swap (e.g. 2Gi, 512Mi, 1G, 100M, or plain bytes)",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_PODMAN_LIMIT_MEM"),
		Name:    "backend-podman-limit-mem",
		Usage:   "maximum memory allowed (e.g. 2Gi, 512Mi, 1G, 100M, or plain bytes)",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_PODMAN_LIMIT_SHM_SIZE"),
		Name:    "backend-podman-limit-shm-size",
		Usage:   "podman /dev/shm size (e.g. 64Mi, 128Mi, or plain bytes)",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_PODMAN_LIMIT_CPU_QUOTA"),
		Name:    "backend-podman-limit-cpu-quota",
		Usage:   "impose a cpu quota (e.g. 2, 1000m, 0.5 for cores)",
	},
	&cli.Int64Flag{
		Sources: cli.EnvVars("CROW_BACKEND_PODMAN_LIMIT_CPU_SHARES"),
		Name:    "backend-podman-limit-cpu-shares",
		Usage:   "change the cpu shares",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_PODMAN_LIMIT_CPU_SET"),
		Name:    "backend-podman-limit-cpu-set",
		Usage:   "set the cpus allowed to execute containers",
	},
}

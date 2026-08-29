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
	"time"

	"github.com/urfave/cli/v3"
)

const (
	// Generous next to the ~26s a cold boot of the cirruslabs base image
	// takes on an M4, because a first pull of a multi-gigabyte image happens
	// inside the same window.
	defaultStartupTimeout = 5 * time.Minute
	// How long a guest gets to shut down cleanly before tart kills it. The
	// VM is discarded either way, so this only decides how gracefully its
	// filesystem is left.
	defaultStopTimeout = 30 * time.Second
)

// Flags contains the CLI flags for the tart backend.
var Flags = []cli.Flag{
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_TART_IMAGE"),
		Name:    "backend-tart-image",
		Usage:   "OCI reference of the macOS VM image cloned for each workflow",
	},
	&cli.BoolFlag{
		Sources: cli.EnvVars("CROW_BACKEND_TART_IMAGE_ALLOW_FROM_STEP"),
		Name:    "backend-tart-image-allow-from-step",
		Usage:   "allow a pipeline to select the VM image via backend_options.tart.image",
		Value:   false,
	},
	&cli.IntFlag{
		Sources: cli.EnvVars("CROW_BACKEND_TART_CPU"),
		Name:    "backend-tart-cpu",
		Usage:   "vCPUs given to each VM, and the maximum a step may request; 0 keeps the image's own setting",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_TART_MEMORY"),
		Name:    "backend-tart-memory",
		Usage:   "memory given to each VM, and the maximum a step may request (e.g. 8Gi, 512Mi); empty keeps the image's own setting",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_TART_NETWORK"),
		Name:    "backend-tart-network",
		Usage:   "guest networking mode: shared, softnet or host",
		Value:   string(networkShared),
	},
	&cli.DurationFlag{
		Sources: cli.EnvVars("CROW_BACKEND_TART_STARTUP_TIMEOUT"),
		Name:    "backend-tart-startup-timeout",
		Usage:   "how long to wait for a booted VM to answer 'tart exec'",
		Value:   defaultStartupTimeout,
	},
	&cli.DurationFlag{
		Sources: cli.EnvVars("CROW_BACKEND_TART_STOP_TIMEOUT"),
		Name:    "backend-tart-stop-timeout",
		Usage:   "how long 'tart stop' waits for a graceful shutdown before killing the VM",
		Value:   defaultStopTimeout,
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_TART_BINARY"),
		Name:    "backend-tart-binary",
		Usage:   "path to the tart binary",
		Value:   "tart",
	},
	&cli.StringFlag{
		Sources: cli.EnvVars("CROW_BACKEND_TART_HOME"),
		Name:    "backend-tart-home",
		Usage:   "TART_HOME for tart child processes; empty uses tart's own default of ~/.tart",
	},
}

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
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// hostArch maps Go's architecture name onto the one the clone plugin's
// releases use. Hyper-V runs on x86_64 and arm64 hosts only.
func hostArch() string {
	return runtime.GOARCH
}

// staleVMs returns the names of this backend's VMs that no longer belong to a
// build the agent is running.
//
// Only VMs carrying the crow- prefix are ever considered, so a VM an operator
// created by hand on the same host is never touched. A VM whose task UUID is
// active belongs to an in-flight build and is left alone regardless of state.
func staleVMs(names []string, activeTaskUUIDs map[string]struct{}) []string {
	active := make(map[string]struct{}, len(activeTaskUUIDs))
	for uuid := range activeTaskUUIDs {
		active[vmName(uuid)] = struct{}{}
	}

	var stale []string
	for _, name := range names {
		if !strings.HasPrefix(name, vmNamePrefix) {
			continue
		}
		if _, ok := active[name]; ok {
			continue
		}
		stale = append(stale, name)
	}
	return stale
}

// CleanupStaleResources removes VMs left behind by an agent that was killed
// rather than shut down.
//
// This matters more here than it does for the tart backend, whose VMs are
// child processes and die with the agent. A Hyper-V VM is owned by the
// hypervisor, so an agent that is killed leaves its guests *running*: they
// keep executing whatever the pipeline was doing, hold their differencing
// disks, and — because the cap is a license — occupy slots that no longer
// correspond to any build.
//
// The staleThreshold argument is unused. A crow- VM this agent does not have
// in flight is abandoned by definition, because the name encodes the task UUID
// of a build that is no longer running, and age would only delay reclaiming it.
func (e *hyperv) CleanupStaleResources(ctx context.Context, _ time.Duration, activeTaskUUIDs map[string]struct{}) (*types.CleanupResult, error) {
	out, err := e.run(ctx, listVMsScript())
	if err != nil {
		return nil, fmt.Errorf("list VMs: %w: %s", err, out)
	}

	stale := staleVMs(parseVMList(out), activeTaskUUIDs)
	result := &types.CleanupResult{ContainersFound: int64(len(stale))}

	for _, name := range stale {
		if err := e.destroyVM(ctx, name, diskPath(e.config.VMPath, name)); err != nil {
			log.Warn().Err(err).Str("vm", name).Msg("could not delete stale VM")
			continue
		}
		result.ContainersRemoved++
		log.Info().Str("vm", name).Msg("removed stale Hyper-V VM")
	}

	return result, nil
}

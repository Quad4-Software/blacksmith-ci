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
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// staleVMs returns the names of this backend's VMs that no longer belong to a
// build the agent is running.
//
// Only local VMs carrying the crow- prefix are ever considered, so a VM an
// operator created by hand on the same Mac is never touched. A VM whose task
// UUID is active belongs to an in-flight build and is left alone regardless of
// its age or state.
func staleVMs(entries []vmEntry, activeTaskUUIDs map[string]struct{}) []string {
	active := make(map[string]struct{}, len(activeTaskUUIDs))
	for uuid := range activeTaskUUIDs {
		active[vmName(uuid)] = struct{}{}
	}

	var stale []string
	for _, entry := range entries {
		if entry.Source != "local" || !strings.HasPrefix(entry.Name, vmNamePrefix) {
			continue
		}
		if _, ok := active[entry.Name]; ok {
			continue
		}
		stale = append(stale, entry.Name)
	}
	return stale
}

// CleanupStaleResources removes VMs left behind by an agent that was killed
// rather than shut down.
//
// Such an agent's VMs stop with it, because they are its children, but they
// stay registered with tart and on disk. Nothing else reclaims them, and each
// one holds a clone of a multi-gigabyte image.
//
// The staleThreshold argument is unused: a tart VM cannot outlive the agent
// that created it, so any crow- VM this agent does not have in flight is
// already abandoned, and there is no in-progress state that age could protect.
func (e *tart) CleanupStaleResources(ctx context.Context, _ time.Duration, activeTaskUUIDs map[string]struct{}) (*types.CleanupResult, error) {
	out, err := e.command(ctx, listArgs()...).Output()
	if err != nil {
		return nil, fmt.Errorf("tart list: %w", err)
	}

	entries, err := parseVMList(out)
	if err != nil {
		return nil, err
	}

	stale := staleVMs(entries, activeTaskUUIDs)
	result := &types.CleanupResult{ContainersFound: int64(len(stale))}

	for _, name := range stale {
		if out, err := e.command(ctx, stopArgs(name, e.config.StopTimeout)...).CombinedOutput(); err != nil {
			// Already stopped is the normal case here, so this is not a
			// failure worth reporting above debug level.
			log.Debug().Err(err).Str("vm", name).Msgf("tart stop: %s", bytes.TrimSpace(out))
		}
		if out, err := e.command(ctx, deleteArgs(name)...).CombinedOutput(); err != nil {
			log.Warn().Err(err).Str("vm", name).Msgf("could not delete stale VM: %s", bytes.TrimSpace(out))
			continue
		}
		result.ContainersRemoved++
		log.Info().Str("vm", name).Msg("removed stale tart VM")
	}

	return result, nil
}

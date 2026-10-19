// Copyright 2026 Crow CI Authors
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

// Package agentrestart rolls restarts out to agents that accept them, a few at
// a time, so they come back on the server's release without an admin asking
// each one.
package agentrestart

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/pipeline/rpc/proto"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/services/lock"
	"codefloe.com/crowci/crow/v6/server/store"
)

// Policy selects which agents are restarted without an admin asking.
type Policy string

const (
	// PolicyOff restarts only agents an admin asked or queued.
	PolicyOff Policy = "off"
	// PolicyOutdated also restarts agents on an older protocol than the server.
	PolicyOutdated Policy = "outdated"
	// PolicyRelease also restarts agents on any release other than the server's.
	PolicyRelease Policy = "release"
)

const (
	leaderLock = "agents:restart:leader"
	// How often queued restarts are promoted and the policy is applied.
	checkInterval = 30 * time.Second
	// An agent that has not reported health for this long cannot receive a
	// request, and a draining agent that went away no longer holds a slot.
	contactWindow = 2 * time.Minute
)

// ParsePolicy validates a policy name, where empty means off.
func ParsePolicy(s string) (Policy, error) {
	switch p := Policy(s); p {
	case "", PolicyOff:
		return PolicyOff, nil
	case PolicyOutdated, PolicyRelease:
		return p, nil
	default:
		return "", fmt.Errorf("invalid agent auto restart policy %q, use off, outdated or release", s)
	}
}

// Config controls the rollout.
type Config struct {
	Policy Policy
	// MaxParallel is how many agents may drain for a restart at the same time.
	MaxParallel int
	// ServerVersion is the release agents are brought onto.
	ServerVersion string
	// Kick releases the agent's waiting polls so none picks up new work.
	Kick func(agentID int64)
}

// Outdated reports whether the agent runs an older protocol than the server.
func Outdated(agent *model.Agent) bool {
	return agent.ProtocolVersion > 0 && agent.ProtocolVersion < proto.Version
}

// ReleaseDiffers reports whether the agent runs a release other than serverVersion.
func ReleaseDiffers(agent *model.Agent, serverVersion string) bool {
	return Outdated(agent) || agent.Version != serverVersion
}

// due reports whether the policy restarts the agent. It asks at most once per
// server release, so an agent whose supervisor brings back the same old
// release keeps running instead of restarting in a loop.
func (c Config) due(agent *model.Agent) bool {
	if agent.AutoRestartVersion == c.ServerVersion {
		return false
	}
	switch c.Policy {
	case PolicyOutdated:
		return Outdated(agent)
	case PolicyRelease:
		return ReleaseDiffers(agent, c.ServerVersion)
	default:
		return false
	}
}

// Reconcile requests restarts for queued agents and agents the policy selects,
// keeping at most MaxParallel agents draining. It returns how many it requested.
func Reconcile(s store.Store, cfg Config, now time.Time) (int, error) {
	agents, err := s.AgentList(&model.AgentListOptions{ListOptions: model.ListOptions{All: true}})
	if err != nil {
		return 0, err
	}

	cutoff := now.Add(-contactWindow).Unix()
	draining := 0
	var queued, due []*model.Agent
	for _, agent := range agents {
		if agent.LastContact < cutoff {
			continue
		}
		if agent.RestartRequested {
			draining++
			continue
		}
		if agent.RemoteUpdate != rpc.RemoteUpdateRestart {
			continue
		}
		switch {
		case agent.RestartQueued:
			queued = append(queued, agent)
		case cfg.due(agent):
			due = append(due, agent)
		}
	}

	// Queued agents go first: an admin asked for them explicitly.
	candidates := slices.Concat(queued, due)
	requested := 0
	for _, agent := range candidates {
		if draining >= cfg.MaxParallel {
			log.Debug().Int("waiting", len(candidates)-requested).Int("max_parallel", cfg.MaxParallel).
				Msg("agent restart limit reached, the remaining agents wait for a free slot")
			break
		}
		if err := s.AgentRequestAutoRestart(agent.ID, cfg.ServerVersion); err != nil {
			return requested, fmt.Errorf("request restart of agent %d: %w", agent.ID, err)
		}
		if cfg.Kick != nil {
			cfg.Kick(agent.ID)
		}
		draining++
		requested++
		log.Info().Int64("agent_id", agent.ID).Str("name", agent.Name).Str("agent_version", agent.Version).
			Str("server_version", cfg.ServerVersion).Bool("queued", agent.RestartQueued).
			Msg("asked agent to restart once its workflows finish")
	}
	return requested, nil
}

// Run applies Reconcile periodically on the instance holding the leader lock,
// so replicas in an HA deployment do not exceed the limit between them.
func Run(ctx context.Context, s store.Store, lockService *lock.Service, leaderLockTTL time.Duration, cfg Config) {
	for {
		lockCtx, releaseLock, err := lockService.AcquireLock(ctx, leaderLock, leaderLockTTL)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(checkInterval):
				continue
			}
		}

		runAsLeader(ctx, lockCtx, s, cfg)
		releaseLock()

		if ctx.Err() != nil {
			return
		}
	}
}

func runAsLeader(ctx, lockCtx context.Context, s store.Store, cfg Config) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-lockCtx.Done():
			return
		case <-ticker.C:
			if _, err := Reconcile(s, cfg, time.Now()); err != nil {
				log.Error().Err(err).Msg("failed to roll out agent restarts")
			}
		}
	}
}

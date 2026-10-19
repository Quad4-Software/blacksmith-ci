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

package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/pipeline/rpc"
)

// ErrRemoteRestart is the error an agent exits with after draining for a
// restart the server requested, so any supervisor restart policy, including
// on-failure, starts it again and pulls the newer release.
var ErrRemoteRestart = errors.New("restart requested by the server to update the agent")

// remoteRestartPollInterval is how often a draining agent checks whether its
// last workflow has finished. A variable so tests do not have to wait.
var remoteRestartPollInterval = 2 * time.Second

const remoteUpdateOff = "off"

// validateRemoteUpdate checks the CROW_AGENT_REMOTE_UPDATE value and returns
// the mode reported to the server, empty when restart requests are ignored.
func validateRemoteUpdate(mode string) (string, error) {
	switch mode {
	case "", remoteUpdateOff:
		return "", nil
	case rpc.RemoteUpdateRestart:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid remote update mode %q, use %q or %q", mode, remoteUpdateOff, rpc.RemoteUpdateRestart)
	}
}

// idleState is the part of agent.State that the drain needs.
type idleState interface {
	Idle() bool
}

// remoteRestart reacts to restart requests from the server. With the restart
// mode it drains once: the server already stopped handing the agent new work,
// so it waits for the running workflows and then shuts the agent down with
// ErrRemoteRestart. Otherwise it logs once that the request is ignored.
type remoteRestart struct {
	mode  string
	state idleState
	// start runs the drain in the agent's service group.
	start func(func() error)

	once sync.Once
}

func (r *remoteRestart) request(ctx context.Context) {
	r.once.Do(func() {
		if r.mode != rpc.RemoteUpdateRestart {
			log.Warn().Msg("server requested a restart to update the agent, ignoring it because CROW_AGENT_REMOTE_UPDATE is not set to restart")
			return
		}
		log.Info().Msg("server requested a restart to update the agent, finishing running workflows first")
		r.start(func() error { return drainForRestart(ctx, r.state) })
	})
}

// drainForRestart waits until no workflow is running and then stops the agent.
func drainForRestart(ctx context.Context, state idleState) error {
	for {
		if state.Idle() {
			log.Info().Msg("no workflows left, exiting so the supervisor restarts the agent")
			stopAgentFunc(ErrRemoteRestart)
			return ErrRemoteRestart
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(remoteRestartPollInterval):
		}
	}
}

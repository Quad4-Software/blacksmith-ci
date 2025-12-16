// Copyright 2024 Woodpecker Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pipeline

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

const shutdownTimeout = time.Second * 5

var (
	shutdownCtx       context.Context
	shutdownCtxCancel context.CancelFunc
	shutdownCtxLock   sync.Mutex

	// PreserveWorkflows indicates whether workflows should be preserved (not destroyed)
	// on shutdown. When true, containers/pods are left running for potential adoption
	// by a restarted agent. This is set when the agent shuts down due to server
	// disconnect rather than a normal termination.
	preserveWorkflows atomic.Bool
)

func GetShutdownCtx() context.Context {
	shutdownCtxLock.Lock()
	defer shutdownCtxLock.Unlock()
	if shutdownCtx == nil {
		shutdownCtx, shutdownCtxCancel = context.WithTimeout(context.Background(), shutdownTimeout)
	}
	return shutdownCtx
}

func CancelShutdown() {
	shutdownCtxLock.Lock()
	defer shutdownCtxLock.Unlock()
	if shutdownCtxCancel == nil {
		// we create an canceled context
		shutdownCtx, shutdownCtxCancel = context.WithCancel(context.Background()) //nolint:forbidigo
	}
	shutdownCtxCancel()
}

// SetPreserveWorkflows signals that workflows should be preserved (not destroyed)
// on shutdown. This is used when the agent shuts down due to server disconnect,
// allowing the containers/pods to be adopted by a restarted agent.
func SetPreserveWorkflows(preserve bool) {
	preserveWorkflows.Store(preserve)
}

// ShouldPreserveWorkflows returns true if workflows should be preserved on shutdown.
func ShouldPreserveWorkflows() bool {
	return preserveWorkflows.Load()
}

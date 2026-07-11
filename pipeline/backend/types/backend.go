// Copyright 2023 Woodpecker Authors
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

package types

import (
	"context"
	"io"
	"time"

	"github.com/urfave/cli/v3"
)

// Backend defines a container orchestration backend and is used
// to create and manage container resources.
type Backend interface {
	// Name returns the name of the backend.
	Name() string

	// IsAvailable check if the backend is available.
	IsAvailable(ctx context.Context) bool

	// Flags return the configuration flags of the backend.
	Flags() []cli.Flag

	// Load loads the backend engine.
	Load(ctx context.Context) (*BackendInfo, error)

	// SetupWorkflow sets up the workflow environment.
	SetupWorkflow(ctx context.Context, conf *Config, taskUUID string) error

	// StartStep starts the workflow step.
	StartStep(ctx context.Context, step *Step, taskUUID string) error

	// WaitStep waits for the workflow step to complete and returns
	// the completion results.
	WaitStep(ctx context.Context, step *Step, taskUUID string) (*State, error)

	// TailStep tails the workflow step logs.
	TailStep(ctx context.Context, step *Step, taskUUID string) (io.ReadCloser, error)

	// DestroyStep destroys the workflow step.
	DestroyStep(ctx context.Context, step *Step, taskUUID string) error

	// DestroyWorkflow destroys the workflow environment.
	DestroyWorkflow(ctx context.Context, conf *Config, taskUUID string) error
}

// BackendInfo represents the reported information of a loaded backend.
type BackendInfo struct {
	Platform string
}

// WorkflowDiscoverer is an optional interface that backends can implement
// to support discovering orphaned workflows for adoption after agent restart.
// Backends that cannot support this (like local) should not implement it.
type WorkflowDiscoverer interface {
	// DiscoverWorkflows finds orphaned workflows that can be adopted.
	// Returns a list of discovered workflows with their task UUIDs and running steps.
	DiscoverWorkflows(ctx context.Context) ([]*DiscoveredWorkflow, error)
}

// DiscoveredWorkflow represents a workflow found during discovery that may be
// eligible for adoption by a restarted agent.
type DiscoveredWorkflow struct {
	// TaskUUID is the unique identifier for the workflow/task.
	TaskUUID string
	// Steps contains information about the running steps in this workflow.
	Steps []*DiscoveredStep
}

// DiscoveredStep represents a step found during workflow discovery.
type DiscoveredStep struct {
	// UUID is the step's unique identifier.
	UUID string
	// Name is the human-readable step name.
	Name string
	// ContainerID is the backend-specific container/pod identifier.
	ContainerID string
}

// WorkflowResumer is an optional interface that backends can implement
// to support resuming monitoring of adopted workflows after agent restart.
// This is used in conjunction with WorkflowDiscoverer.
type WorkflowResumer interface {
	// ResumeStep resumes monitoring of an already-running step (for adopted workflows).
	// Unlike StartStep, this does not create a new container - it attaches to an existing one.
	// The step parameter should have its UUID set to match the running container.
	ResumeStep(ctx context.Context, step *Step, taskUUID string) error
}

// ResourceCleaner is an optional interface that backends can implement
// to support cleaning up stale resources left behind by improperly terminated pipelines.
type ResourceCleaner interface {
	// CleanupStaleResources finds and removes stale containers, networks, and volumes
	// that are older than the given threshold. Resources whose task UUID is present
	// in activeTaskUUIDs belong to a build currently running on this agent and are
	// always left alone, regardless of age or state, so an in-flight pipeline that
	// outlives the stale threshold is never reclaimed.
	CleanupStaleResources(ctx context.Context, staleThreshold time.Duration, activeTaskUUIDs map[string]struct{}) (*CleanupResult, error)
}

// Pinger is an optional interface that backends can implement to expose a
// live readiness probe against the underlying engine (Docker daemon,
// Kubernetes API, …). IsAvailable only checks static prerequisites at
// startup; Ping is intended to be called periodically while the agent is
// running so the agent can detect engines that became unreachable after
// startup (e.g. Docker socket replaced by a daemon upgrade).
type Pinger interface {
	Ping(ctx context.Context) error
}

// CleanupResult holds the results of a stale resource cleanup operation.
type CleanupResult struct {
	ContainersFound   int64
	ContainersRemoved int64
	NetworksFound     int64
	NetworksRemoved   int64
	VolumesFound      int64
	VolumesRemoved    int64
}

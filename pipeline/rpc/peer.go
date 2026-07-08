// Copyright 2021 Woodpecker Authors
// Copyright 2011 Drone.IO Inc.
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

package rpc

import (
	"context"
	"time"

	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

type (
	// DockerCleanupConfig holds cleanup configuration sent from server to agent.
	DockerCleanupConfig struct {
		Enabled               bool  `json:"enabled"`
		StaleThresholdSeconds int64 `json:"stale_threshold_seconds"`
		IntervalSeconds       int64 `json:"interval_seconds"`
		RunNow                bool  `json:"run_now"`
	}

	// DockerCleanupResult holds cleanup results sent from agent to server.
	DockerCleanupResult struct {
		ContainersFound   int64  `json:"containers_found"`
		ContainersRemoved int64  `json:"containers_removed"`
		NetworksFound     int64  `json:"networks_found"`
		NetworksRemoved   int64  `json:"networks_removed"`
		VolumesFound      int64  `json:"volumes_found"`
		VolumesRemoved    int64  `json:"volumes_removed"`
		Success           bool   `json:"success"`
		Error             string `json:"error"`
		DurationMs        int64  `json:"duration_ms"`
		Manual            bool   `json:"manual"`
	}

	// Filter defines filters for fetching items from the queue.
	Filter struct {
		Labels map[string]string `json:"labels"`
	}

	// StepState defines the step state.
	StepState struct {
		StepUUID string `json:"step_uuid"`
		Started  int64  `json:"started"`
		Finished int64  `json:"finished"`
		Exited   bool   `json:"exited"`
		ExitCode int    `json:"exit_code"`
		Error    string `json:"error"`
	}

	// WorkflowState defines the workflow state.
	WorkflowState struct {
		Started  int64  `json:"started"`
		Finished int64  `json:"finished"`
		Error    string `json:"error"`
	}

	// Workflow defines the workflow execution details.
	Workflow struct {
		ID      string          `json:"id"`
		Config  *backend.Config `json:"config"`
		Timeout int64           `json:"timeout"`
	}

	Version struct {
		GrpcVersion   int32  `json:"grpc_version,omitempty"`
		ServerVersion string `json:"server_version,omitempty"`
	}

	// AgentInfo represents all the metadata that should be known about an agent.
	AgentInfo struct {
		Version       string            `json:"version"`
		BuildDate     string            `json:"build_date"`
		Platform      string            `json:"platform"`
		Backend       string            `json:"backend"`
		Capacity      int               `json:"capacity"`
		CustomLabels  map[string]string `json:"custom_labels"`
		LimitMem      int64             `json:"limit_mem"`
		LimitCPUQuota int64             `json:"limit_cpu_quota"`
	}
)

// Peer defines a peer-to-peer connection.
type Peer interface {
	// Version returns the server- & grpc-version
	Version(c context.Context) (*Version, error)

	// Next returns the next workflow in the queue
	Next(c context.Context, f Filter) (*Workflow, error)

	// Wait blocks until the workflow is complete
	Wait(c context.Context, workflowID string) error

	// Init signals the workflow is initialized
	Init(c context.Context, workflowID string, state WorkflowState) error

	// Done signals the workflow is complete
	Done(c context.Context, workflowID string, state WorkflowState) error

	// Extend extends the workflow deadline
	Extend(c context.Context, workflowID string) error

	// Update updates the step state
	Update(c context.Context, workflowID string, state StepState) error

	// EnqueueLog queues the step log entry for delayed sending
	EnqueueLog(logEntry *LogEntry)

	// RegisterAgent register our agent to the server
	RegisterAgent(ctx context.Context, info AgentInfo) (int64, string, error)

	// UnregisterAgent unregister our agent from the server
	UnregisterAgent(ctx context.Context) error

	// ReportHealth reports health status of the agent to the server and receives config
	ReportHealth(c context.Context) (*DockerCleanupConfig, error)

	// ReportDockerCleanup sends Docker cleanup results to the server
	ReportDockerCleanup(c context.Context, result DockerCleanupResult) error

	// SetReregistrationHandler sets a callback to re-register the agent when not found on server
	SetReregistrationHandler(handler func(context.Context) error)

	// SetWorkflowPollingHealthy marks workflow polling as healthy or unhealthy
	SetWorkflowPollingHealthy(healthy bool)

	// SetMaxErrorDuration overrides how long the client retries gRPC calls
	// before returning ErrAgentRestartRequired.
	SetMaxErrorDuration(d time.Duration)

	// ClaimTask attempts to claim an orphaned workflow task for adoption after agent restart.
	// Returns the workflow if successfully claimed, nil if the task couldn't be claimed.
	ClaimTask(c context.Context, taskID string) (*Workflow, error)
}

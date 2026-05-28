// Copyright 2022 Woodpecker Authors
// Copyright 2021 Informatyka Boguslawski sp. z o.o. sp.k., http://www.ib.pl/
// Copyright 2018 Drone.IO Inc.
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

package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	grpcMetadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"codefloe.com/crowci/crow/v5/pipeline"
	"codefloe.com/crowci/crow/v5/pipeline/rpc"
	"codefloe.com/crowci/crow/v5/pipeline/rpc/proto"
	"codefloe.com/crowci/crow/v5/server"
	"codefloe.com/crowci/crow/v5/server/forge"
	"codefloe.com/crowci/crow/v5/server/logging"
	"codefloe.com/crowci/crow/v5/server/model"
	serverPipeline "codefloe.com/crowci/crow/v5/server/pipeline"
	"codefloe.com/crowci/crow/v5/server/proclog"
	"codefloe.com/crowci/crow/v5/server/pubsub"
	"codefloe.com/crowci/crow/v5/server/queue"
	"codefloe.com/crowci/crow/v5/server/store"
)

// updateAgentLastWorkDelay the delay before the LastWork info should be updated.
const updateAgentLastWorkDelay = time.Minute

type RPC struct {
	queue           queue.Queue
	pubsub          pubsub.Publisher
	logger          logging.Log
	store           store.Store
	metrics         *GRPCMetrics
	procLogReceiver *proclog.Receiver
}

// Replaces legacy filter labels with new ones.
// Only migrates labels that have actually changed names (repo -> crowci/repo-full-name, org-id -> crowci/org-id).
// Labels like platform, hostname, backend remain unchanged and should NOT be deleted.
func migrateFilterLabels(filter rpc.Filter) rpc.Filter {
	// Only migrate labels that actually need renaming
	if value, ok := filter.Labels["repo"]; ok {
		filter.Labels[pipeline.LabelRepoFullName] = value
		delete(filter.Labels, "repo")
	}

	if value, ok := filter.Labels["org-id"]; ok {
		filter.Labels[pipeline.LabelOrgID] = value
		delete(filter.Labels, "org-id")
	}

	// platform, hostname, and backend labels don't need migration as their
	// constant names (LabelPlatform, LabelHostname, LabelBackend) are identical
	// to the old names. Migrating them would just assign to the same key and delete it.

	return filter
}

// Next blocks until it provides the next workflow to execute.
func (s *RPC) Next(c context.Context, agentFilter rpc.Filter) (*rpc.Workflow, error) {
	if hostname, err := s.getHostnameFromContext(c); err == nil {
		log.Debug().Msgf("agent connected: %s: polling", hostname)
	}

	log.Debug().
		Interface("filter_labels_before_migration", agentFilter.Labels).
		Msg("received filter labels from agent")

	agentFilter = migrateFilterLabels(agentFilter)

	log.Debug().
		Interface("filter_labels_after_migration", agentFilter.Labels).
		Msg("filter labels after migration")

	agent, err := s.getAgentFromContext(c)
	if err != nil {
		return nil, err
	}

	if agent.NoSchedule {
		log.Debug().Str("agentName", agent.Name).Int64("agentID", agent.ID).Msg("Agent marked as NoSchedule, returning nil task")
		time.Sleep(1 * time.Second)
		return nil, nil
	}

	agentServerLabels, err := agent.GetServerLabels()
	if err != nil {
		return nil, err
	}

	// enforce server labels (like org-id restrictions) while preserving agent labels
	// (backend, platform, hostname, custom labels)
	for k, v := range agentServerLabels {
		agentFilter.Labels[k] = v
	}

	log.Debug().
		Str("agent_name", agent.Name).
		Int64("agent_id", agent.ID).
		Interface("agent_filter_labels", agentFilter.Labels).
		Interface("agent_custom_labels", agent.CustomLabels).
		Int32("priority", agent.Priority).
		Msg("agent polling for tasks")

	filterFn := createFilterFuncWithPriority(agentFilter, agent.CustomLabels, agent.Priority)

	for {
		// poll blocks until a task is available or the context is canceled / worker is kicked
		task, err := s.queue.Poll(c, agent.ID, filterFn)
		if err != nil || task == nil {
			return nil, err
		}

		if task.ShouldRun() {
			workflow := new(rpc.Workflow)
			err = json.Unmarshal(task.Data, workflow)
			return workflow, err
		}

		// task should not run, so mark it as done
		if err := s.Done(c, task.ID, rpc.WorkflowState{}); err != nil {
			log.Error().Err(err).Msgf("marking workflow task '%s' as done failed", task.ID)
		}
	}
}

// Wait blocks until the workflow with the given ID is done.
func (s *RPC) Wait(c context.Context, workflowID string) error {
	agent, err := s.getAgentFromContext(c)
	if err != nil {
		return err
	}

	if err := s.checkAgentPermissionByWorkflow(c, agent, workflowID, nil, nil); err != nil {
		return err
	}

	return s.queue.Wait(c, workflowID)
}

// Extend extends the lease for the workflow with the given ID.
func (s *RPC) Extend(c context.Context, workflowID string) error {
	agent, err := s.getAgentFromContext(c)
	if err != nil {
		return err
	}

	err = s.updateAgentLastWork(agent)
	if err != nil {
		return err
	}

	if err := s.checkAgentPermissionByWorkflow(c, agent, workflowID, nil, nil); err != nil {
		return err
	}

	return s.queue.Extend(c, agent.ID, workflowID)
}

// Update updates the state of a step.
func (s *RPC) Update(c context.Context, strWorkflowID string, state rpc.StepState) error {
	workflowID, err := strconv.ParseInt(strWorkflowID, 10, 64)
	if err != nil {
		return err
	}

	workflow, err := s.store.WorkflowLoad(workflowID)
	if err != nil {
		log.Error().Err(err).Msgf("rpc.update: cannot find workflow with id %d", workflowID)
		return err
	}

	currentPipeline, err := s.store.GetPipeline(workflow.PipelineID)
	if err != nil {
		log.Error().Err(err).Msgf("cannot find pipeline with id %d", workflow.PipelineID)
		return err
	}

	agent, err := s.getAgentFromContext(c)
	if err != nil {
		return err
	}

	step, err := s.store.StepByUUID(state.StepUUID)
	if err != nil {
		log.Error().Err(err).Msgf("cannot find step with uuid %s", state.StepUUID)
		return err
	}

	if step.PipelineID != currentPipeline.ID {
		msg := fmt.Sprintf("agent returned status with step uuid '%s' which does not belong to current pipeline", state.StepUUID)
		log.Error().
			Int64("stepPipelineID", step.PipelineID).
			Int64("currentPipelineID", currentPipeline.ID).
			Msg(msg)
		return errors.New(msg)
	}

	repo, err := s.store.GetRepo(currentPipeline.RepoID)
	if err != nil {
		log.Error().Err(err).Msgf("cannot find repo with id %d", currentPipeline.RepoID)
		return err
	}

	// check before agent can alter some state
	if err := s.checkAgentPermissionByWorkflow(c, agent, strWorkflowID, currentPipeline, repo); err != nil {
		return err
	}

	if err := serverPipeline.UpdateStepStatus(s.store, step, state); err != nil {
		log.Error().Err(err).Msg("rpc.update: cannot update step")
	}

	if currentPipeline.Workflows, err = s.store.WorkflowGetTree(currentPipeline); err != nil {
		log.Error().Err(err).Msg("cannot build tree from step list")
		return err
	}
	message := pubsub.Message{
		Labels: map[string]string{
			"repo":    repo.FullName,
			"private": strconv.FormatBool(repo.IsSCMPrivate),
		},
	}
	message.Data, err = json.Marshal(model.Event{
		Repo:     *repo,
		Pipeline: *currentPipeline,
	})
	if err != nil {
		return err
	}
	s.pubsub.Publish(message)

	return nil
}

// Init implements the rpc.Init function.
func (s *RPC) Init(c context.Context, strWorkflowID string, state rpc.WorkflowState) error {
	workflowID, err := strconv.ParseInt(strWorkflowID, 10, 64)
	if err != nil {
		return err
	}

	workflow, err := s.store.WorkflowLoad(workflowID)
	if err != nil {
		log.Error().Err(err).Msgf("cannot find workflow with id %d", workflowID)
		return err
	}

	agent, err := s.getAgentFromContext(c)
	if err != nil {
		return err
	}

	workflow.AgentID = agent.ID
	workflow.AgentPersistentID = agent.PersistentID
	workflow.AgentName = agent.Name

	currentPipeline, err := s.store.GetPipeline(workflow.PipelineID)
	if err != nil {
		log.Error().Err(err).Msgf("cannot find pipeline with id %d", workflow.PipelineID)
		return err
	}

	repo, err := s.store.GetRepo(currentPipeline.RepoID)
	if err != nil {
		log.Error().Err(err).Msgf("cannot find repo with id %d", currentPipeline.RepoID)
		return err
	}

	// check before agent can alter some state
	if err := s.checkAgentPermissionByWorkflow(c, agent, strWorkflowID, currentPipeline, repo); err != nil {
		return err
	}

	if currentPipeline.Status == model.StatusPending {
		if currentPipeline, err = serverPipeline.UpdateToStatusRunning(s.store, *currentPipeline, state.Started); err != nil {
			log.Error().Err(err).Msgf("init: cannot update pipeline %d state", currentPipeline.ID)
		}
	}

	s.updateForgeStatus(c, repo, currentPipeline, workflow)

	defer func() {
		currentPipeline.Workflows, _ = s.store.WorkflowGetTree(currentPipeline)
		message := pubsub.Message{
			Labels: map[string]string{
				"repo":    repo.FullName,
				"private": strconv.FormatBool(repo.IsSCMPrivate),
			},
		}
		message.Data, err = json.Marshal(model.Event{
			Repo:     *repo,
			Pipeline: *currentPipeline,
		})
		if err != nil {
			log.Error().Err(err).Msg("could not marshal JSON")
			return
		}
		s.pubsub.Publish(message)
	}()

	workflow, err = serverPipeline.UpdateWorkflowStatusToRunning(s.store, *workflow, state)
	if err != nil {
		return err
	}
	s.updateForgeStatus(c, repo, currentPipeline, workflow)

	s.publishAgentStats(c, agent.ID)
	return s.updateAgentLastWork(agent)
}

// Done marks the workflow with the given ID as done.
func (s *RPC) Done(c context.Context, strWorkflowID string, state rpc.WorkflowState) error { //nolint:contextcheck // email notification intentionally uses context.Background() to outlive the gRPC call
	workflowID, err := strconv.ParseInt(strWorkflowID, 10, 64)
	if err != nil {
		return err
	}

	workflow, err := s.store.WorkflowLoad(workflowID)
	if err != nil {
		log.Error().Err(err).Msgf("cannot find workflow with id %d - workflow may have been deleted or database is inconsistent", workflowID)
		// Try to acknowledge the workflow in the queue even if it's not found in the database
		// This prevents queue items from getting stuck
		if queueErr := s.queue.Done(c, strWorkflowID, model.StatusError); queueErr != nil {
			log.Trace().Err(queueErr).Msgf("queue.Done: cannot ack missing workflow %d - workflow may have been already cleaned up", workflowID)
		}
		return err
	}

	workflow.Children, err = s.store.StepListFromWorkflowFind(workflow)
	if err != nil {
		return err
	}

	currentPipeline, err := s.store.GetPipeline(workflow.PipelineID)
	if err != nil {
		log.Error().Err(err).Msgf("cannot find pipeline with id %d", workflow.PipelineID)
		return err
	}

	repo, err := s.store.GetRepo(currentPipeline.RepoID)
	if err != nil {
		log.Error().Err(err).Msgf("cannot find repo with id %d", currentPipeline.RepoID)
		return err
	}

	agent, err := s.getAgentFromContext(c)
	if err != nil {
		return err
	}

	// check before agent can alter some state
	if err := s.checkAgentPermissionByWorkflow(c, agent, strWorkflowID, currentPipeline, repo); err != nil {
		return err
	}

	logger := log.With().
		Str("repo_id", fmt.Sprint(repo.ID)).
		Str("pipeline_id", fmt.Sprint(currentPipeline.ID)).
		Str("workflow_id", strWorkflowID).Logger()

	logger.Trace().Msgf("gRPC Done with state: %#v", state)

	if workflow, err = serverPipeline.UpdateWorkflowStatusToDone(s.store, *workflow, state); err != nil {
		logger.Error().Err(err).Msgf("serverPipeline.UpdateWorkflowStatusToDone: cannot update workflow state: %s", err)
	}

	var queueErr error
	if workflow.Failing() {
		queueErr = s.queue.Error(c, strWorkflowID, fmt.Errorf("workflow finished with error %s", state.Error))
	} else {
		queueErr = s.queue.Done(c, strWorkflowID, workflow.State)
	}
	if queueErr != nil {
		logger.Error().Err(queueErr).Msg("queue.Done: cannot ack workflow - this may indicate database inconsistency or race condition")
	}

	currentPipeline.Workflows, err = s.store.WorkflowGetTree(currentPipeline)
	if err != nil {
		return err
	}
	s.completeChildrenIfParentCompleted(workflow)

	// When a workflow finishes non-successfully, eagerly skip any pending
	// downstream workflows whose default run-on-success semantics make them
	// unrunnable. Without this, those workflows would linger in Pending
	// state and IsThereRunningStage() below would keep the pipeline stuck
	// in "running" until an agent lazily polled and dropped them.
	if workflow.State != model.StatusSuccess {
		if cascadeErr := serverPipeline.CascadeSkipBlockedWorkflows(c, s.store, s.queue, currentPipeline); cascadeErr != nil {
			logger.Error().Err(cascadeErr).Msg("cascade skip blocked workflows failed")
		}
		if currentPipeline.Workflows, err = s.store.WorkflowGetTree(currentPipeline); err != nil {
			return err
		}
	}

	if !model.IsThereRunningStage(currentPipeline.Workflows) {
		if currentPipeline, err = serverPipeline.UpdateStatusToDone(s.store, *currentPipeline, model.PipelineStatus(currentPipeline.Workflows), workflow.Finished); err != nil {
			logger.Error().Err(err).Msgf("serverPipeline.UpdateStatusToDone: cannot update workflows final state")
		}
	}

	s.updateForgeStatus(c, repo, currentPipeline, workflow)

	// make sure writes to pubsub are non blocking (https://github.com/woodpecker-ci/woodpecker/blob/c919f32e0b6432a95e1a6d3d0ad662f591adf73f/server/logging/log.go#L9)
	go func() {
		for _, step := range workflow.Children {
			if err := s.logger.Close(c, step.ID); err != nil {
				logger.Error().Err(err).Msgf("done: cannot close log stream for step %d - stream may have been already closed or step was cleaned up", step.ID)
			}
		}
	}()

	if err := s.notify(repo, currentPipeline); err != nil {
		return err
	}

	// Email notification: only dispatch when the pipeline has reached a terminal state
	// (all workflows finished). Uses context.Background() intentionally because the
	// parent gRPC context is canceled when Done() returns.
	if notifySvc := server.Config.Services.NotificationService; notifySvc != nil && !model.IsThereRunningStage(currentPipeline.Workflows) {
		prevStatus := s.loadPreviousPipelineStatus(repo, currentPipeline)
		notifyCtx := context.Background()
		go func() {
			if err := notifySvc.Dispatch(notifyCtx, repo, currentPipeline, prevStatus); err != nil {
				logger.Error().Err(err).Msg("email notification dispatch failed")
			}
		}()
	}

	if currentPipeline.Status == model.StatusSuccess || currentPipeline.Status == model.StatusFailure {
		s.metrics.PipelineCount.WithLabelValues(repo.FullName, currentPipeline.Branch, string(currentPipeline.Status), "total").Inc()
		s.metrics.PipelineTime.WithLabelValues(repo.FullName, currentPipeline.Branch, string(currentPipeline.Status), "total").Set(float64(currentPipeline.Finished - currentPipeline.Started))
		duration := float64(currentPipeline.Finished - currentPipeline.Started)
		s.metrics.PipelineDuration.WithLabelValues(repo.FullName, currentPipeline.Branch, string(currentPipeline.Status)).Observe(duration)
	}
	if currentPipeline.IsMultiPipeline() {
		s.metrics.PipelineTime.WithLabelValues(repo.FullName, currentPipeline.Branch, string(workflow.State), workflow.Name).Set(float64(workflow.Finished - workflow.Started))
	}

	s.publishAgentStats(c, agent.ID)
	return s.updateAgentLastWork(agent)
}

// Log writes a log entry to the database and publishes it to the pubsub.
// An explicit stepUUID makes it obvious that all entries must come from the same step.
func (s *RPC) Log(c context.Context, stepUUID string, rpcLogEntries []*rpc.LogEntry) error {
	step, err := s.store.StepByUUID(stepUUID)
	if err != nil {
		return fmt.Errorf("could not find step with uuid %s in store: %w", stepUUID, err)
	}

	log.Debug().Str("stepUUID", stepUUID).Int64("stepID", step.ID).Int("logCount", len(rpcLogEntries)).Msg("external agent sending logs for step")

	agent, err := s.getAgentFromContext(c)
	if err != nil {
		return err
	}

	currentPipeline, err := s.store.GetPipeline(step.PipelineID)
	if err != nil {
		log.Error().Err(err).Msgf("cannot find pipeline with id %d", step.PipelineID)
		return err
	}

	// check before agent can alter some state
	if err := s.checkAgentPermissionByWorkflow(c, agent, "", currentPipeline, nil); err != nil {
		return err
	}

	err = s.updateAgentLastWork(agent)
	if err != nil {
		return err
	}

	var logEntries []*model.LogEntry

	for _, rpcLogEntry := range rpcLogEntries {
		if rpcLogEntry.StepUUID != stepUUID {
			return fmt.Errorf("expected step UUID %s, got %s", stepUUID, rpcLogEntry.StepUUID)
		}
		logEntries = append(logEntries, &model.LogEntry{
			StepID: step.ID,
			Time:   rpcLogEntry.Time,
			Line:   rpcLogEntry.Line,
			Data:   rpcLogEntry.Data,
			Type:   model.LogEntryType(rpcLogEntry.Type),
		})
	}

	// make sure writes to pubsub are non blocking (https://github.com/woodpecker-ci/woodpecker/blob/c919f32e0b6432a95e1a6d3d0ad662f591adf73f/server/logging/log.go#L9)
	go func() {
		// write line to listening web clients
		if err := s.logger.Write(c, step.ID, logEntries); err != nil {
			log.Error().Err(err).Msgf("could not write to logger")
		}
	}()

	if err = server.Config.Services.LogStore.LogAppend(step, logEntries); err != nil {
		log.Error().Err(err).Msg("could not store log entries to database")
	}

	return nil
}

// StreamProcLogs receives client-streamed batches of process log entries and forwards them to the
// proclog receiver for storage. Returns Unimplemented when the receiver is not configured.
func (s *RPC) StreamProcLogs(stream proto.Crow_StreamProcLogsServer) error {
	if s.procLogReceiver == nil {
		return status.Error(codes.Unimplemented, "process log ingest is disabled on this server")
	}

	agent, err := s.getAgentFromContext(stream.Context())
	if err != nil {
		return err
	}

	var received int64
	for {
		batch, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return stream.SendAndClose(&proto.ProcLogAck{Received: received})
		}
		if err != nil {
			return err
		}

		entries := make([]*model.ProcLogEntry, 0, len(batch.GetEntries()))
		for _, e := range batch.GetEntries() {
			entries = append(entries, &model.ProcLogEntry{
				Time:  e.GetTime(),
				Level: e.GetLevel(),
				Line:  e.GetLine(),
			})
		}

		if err := s.procLogReceiver.Ingest(stream.Context(), proclog.SourceTypeAgent, agent.ID, entries); err != nil {
			log.Error().Err(err).Msg("proclog: failed to ingest batch")
			return err
		}
		received += int64(len(entries))
	}
}

func (s *RPC) RegisterAgent(ctx context.Context, info rpc.AgentInfo) (int64, string, error) {
	agent, err := s.getAgentFromContext(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to get agent from context during registration")
		return -1, "", err
	}

	log.Debug().
		Int64("agent_id", agent.ID).
		Str("backend", info.Backend).
		Str("platform", info.Platform).
		Interface("custom_labels", info.CustomLabels).
		Msg("registering agent")

	// Always store the hostname from the agent for label filtering
	if hostname, err := s.getHostnameFromContext(ctx); err == nil {
		agent.Hostname = hostname
		// Use hostname as default name if not set
		if agent.Name == "" {
			agent.Name = hostname
		}
	}

	agent.Backend = info.Backend
	agent.Platform = info.Platform
	agent.Capacity = int32(info.Capacity)
	agent.Version = info.Version
	agent.BuildDate = info.BuildDate
	agent.CustomLabels = info.CustomLabels
	agent.Priority = info.Priority
	agent.LimitMem = info.LimitMem
	agent.LimitCPUQuota = info.LimitCPUQuota
	// Reset NoSchedule when agent registers - allows previously disabled agents to receive tasks again
	agent.NoSchedule = false

	// Generate or update persistent ID for stable identity across ID changes
	// This allows metrics to track agents even after cleanup/reconnection
	agent.PersistentID = model.GeneratePersistentID(agent.Token, agent.Name, agent.Platform, agent.Backend)

	err = s.store.AgentUpdate(agent)
	if err != nil {
		log.Error().Err(err).Int64("agent_id", agent.ID).Msg("failed to update agent in database")
		return -1, "", err
	}

	s.metrics.AgentRegistrations.Inc()

	log.Info().
		Int64("agent_id", agent.ID).
		Str("name", agent.Name).
		Interface("custom_labels", agent.CustomLabels).
		Msg("agent registered successfully")

	// Mark Docker cleanup as available when a Docker agent registers
	if info.Backend == "docker" && server.Config.Services.MaintenanceService != nil {
		server.Config.Services.MaintenanceService.SetDockerCleanupAvailable(true)
	}

	return agent.ID, agent.Name, nil
}

// UnregisterAgent removes the agent from the database.
func (s *RPC) UnregisterAgent(ctx context.Context) error {
	agent, err := s.getAgentFromContext(ctx)
	if err != nil {
		return err
	}
	if !agent.IsSystemAgent() {
		// registered with individual agent token -> do not unregister
		return nil
	}
	log.Debug().Msgf("un-registering agent with ID %d", agent.ID)

	if err = s.store.AgentDelete(agent); err != nil {
		return err
	}

	s.metrics.AgentUnregistrations.Inc()

	return nil
}

func (s *RPC) ReportHealth(ctx context.Context, status string, workflowPollingHealthy bool) (*rpc.DockerCleanupConfig, error) {
	agent, err := s.getAgentFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if status != "I am alive!" {
		return nil, errors.New("are you alive?")
	}

	agent.LastContact = time.Now().Unix()
	agent.WorkflowPollingHealthy = workflowPollingHealthy

	if err := s.store.AgentUpdate(agent); err != nil {
		return nil, err
	}

	// Return Docker cleanup config for Docker agents
	if agent.Backend == "docker" && server.Config.Services.MaintenanceService != nil {
		// Ensure availability flag is set (handles agents that registered before this feature)
		if !server.Config.Services.MaintenanceService.IsDockerCleanupAvailable() {
			server.Config.Services.MaintenanceService.SetDockerCleanupAvailable(true)
		}
		cleanupConfig := server.Config.Services.MaintenanceService.GetDockerCleanupConfigForAgent()
		return cleanupConfig, nil
	}

	return nil, nil
}

func (s *RPC) ReportDockerCleanup(ctx context.Context, result rpc.DockerCleanupResult) error {
	agent, err := s.getAgentFromContext(ctx)
	if err != nil {
		return err
	}

	if server.Config.Services.MaintenanceService != nil {
		return server.Config.Services.MaintenanceService.RecordDockerCleanupResult(agent.ID, agent.Name, result)
	}

	return nil
}

// ClaimTask allows a restarted agent to claim an orphaned workflow task.
// This is HA-safe: the task is only returned if it was previously assigned to this specific agent.
func (s *RPC) ClaimTask(ctx context.Context, taskID string) (*rpc.Workflow, error) {
	agent, err := s.getAgentFromContext(ctx)
	if err != nil {
		return nil, err
	}

	log.Debug().
		Int64("agent_id", agent.ID).
		Str("task_id", taskID).
		Msg("agent attempting to claim orphaned task")

	// Atomically claim the task - only succeeds if the task was previously assigned to this agent
	task, err := s.store.TaskAtomicClaim(taskID, agent.ID)
	if err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("failed to claim task")
		return nil, err
	}

	if task == nil {
		log.Debug().
			Int64("agent_id", agent.ID).
			Str("task_id", taskID).
			Msg("task not found or belongs to different agent")
		return nil, nil
	}

	// Deserialize the workflow data
	workflow := new(rpc.Workflow)
	if err := json.Unmarshal(task.Data, workflow); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("failed to unmarshal workflow data")
		return nil, err
	}

	// Validate agent still has permission to run this workflow (labels/repo access may have changed)
	if err := s.checkAgentPermissionByWorkflow(ctx, agent, workflow.ID, nil, nil); err != nil {
		log.Warn().
			Err(err).
			Int64("agent_id", agent.ID).
			Str("task_id", taskID).
			Str("workflow_id", workflow.ID).
			Msg("agent no longer has permission to run this workflow, rejecting claim")
		return nil, nil // Return nil to indicate task can't be claimed by this agent
	}

	// Re-add task to queue's running state so Wait() and Extend() work correctly
	if err := s.queue.Reclaim(ctx, task); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("failed to reclaim task in queue")
		return nil, err
	}

	log.Info().
		Int64("agent_id", agent.ID).
		Str("task_id", taskID).
		Str("workflow_id", workflow.ID).
		Msg("agent successfully claimed orphaned task")

	return workflow, nil
}

func (s *RPC) checkAgentPermissionByWorkflow(_ context.Context, agent *model.Agent, strWorkflowID string, pipeline *model.Pipeline, repo *model.Repo) error {
	var err error
	if repo == nil && pipeline == nil {
		workflowID, err := strconv.ParseInt(strWorkflowID, 10, 64)
		if err != nil {
			return err
		}

		workflow, err := s.store.WorkflowLoad(workflowID)
		if err != nil {
			log.Error().Err(err).Msgf("cannot find workflow with id %d", workflowID)
			return err
		}

		pipeline, err = s.store.GetPipeline(workflow.PipelineID)
		if err != nil {
			log.Error().Err(err).Msgf("cannot find pipeline with id %d", workflow.PipelineID)
			return err
		}
	}

	if repo == nil {
		repo, err = s.store.GetRepo(pipeline.RepoID)
		if err != nil {
			log.Error().Err(err).Msgf("cannot find repo with id %d", pipeline.RepoID)
			return err
		}
	}

	if agent.CanAccessRepo(repo) {
		return nil
	}

	msg := fmt.Sprintf("agent '%d' is not allowed to interact with repo[%d] '%s'", agent.ID, repo.ID, repo.FullName)
	log.Error().Int64("repoId", repo.ID).Msg(msg)
	return errors.New(msg)
}

func (s *RPC) completeChildrenIfParentCompleted(completedWorkflow *model.Workflow) {
	for _, c := range completedWorkflow.Children {
		if c.Running() {
			if _, err := serverPipeline.UpdateStepToStatusSkipped(s.store, *c, completedWorkflow.Finished); err != nil {
				log.Error().Err(err).Msgf("done: cannot update step_id %d child state", c.ID)
			}
		}
	}
}

func (s *RPC) updateForgeStatus(ctx context.Context, repo *model.Repo, pipeline *model.Pipeline, workflow *model.Workflow) {
	user, err := s.store.GetUser(repo.UserID)
	if err != nil {
		log.Error().Err(err).Msgf("cannot get user with id '%d'", repo.UserID)
		return
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	// This ensures we use the correct OAuth tokens for the forge the repo belongs to.
	if userForge, ufErr := s.store.UserForgeGet(user.ID, repo.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}

	_forge, err := server.Config.Services.Manager.ForgeFromRepo(repo)
	if err != nil {
		log.Error().Err(err).Msgf("can not get forge for repo '%s'", repo.FullName)
		return
	}

	if refreshErr := forge.RefreshUserForge(ctx, _forge, s.store, user, repo.ForgeID); refreshErr != nil {
		log.Error().Err(refreshErr).Msg("Failed to refresh user auth token")
		return
	}

	// only do status updates for parent steps
	if workflow != nil {
		err = _forge.Status(ctx, user, repo, pipeline, workflow)
		if err != nil {
			log.Error().Err(err).Msgf("error setting commit status for %s/%d", repo.FullName, pipeline.Number)
		}
	}
}

func (s *RPC) loadPreviousPipelineStatus(repo *model.Repo, pipeline *model.Pipeline) model.StatusValue {
	prev, err := s.store.GetPipelineLastBefore(repo, pipeline.Branch, pipeline.Number)
	if err != nil || prev == nil {
		return ""
	}
	return prev.Status
}

func (s *RPC) notify(repo *model.Repo, pipeline *model.Pipeline) (err error) {
	message := pubsub.Message{
		Labels: map[string]string{
			"repo":    repo.FullName,
			"private": strconv.FormatBool(repo.IsSCMPrivate),
		},
	}
	message.Data, err = json.Marshal(model.Event{
		Repo:     *repo,
		Pipeline: *pipeline,
	})
	if err != nil {
		return err
	}
	s.pubsub.Publish(message)
	return nil
}

func (s *RPC) getAgentFromContext(ctx context.Context) (*model.Agent, error) {
	md, ok := grpcMetadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("metadata is not provided")
	}

	values := md["agent_id"]
	if len(values) == 0 {
		return nil, errors.New("agent_id is not provided")
	}

	_agentID := values[0]
	agentID, err := strconv.ParseInt(_agentID, 10, 64)
	if err != nil {
		return nil, errors.New("agent_id is not a valid integer")
	}

	return s.store.AgentFind(agentID)
}

func (s *RPC) getHostnameFromContext(ctx context.Context) (string, error) {
	metadata, ok := grpcMetadata.FromIncomingContext(ctx)
	if ok {
		hostname, ok := metadata["hostname"]
		if ok && len(hostname) != 0 {
			return hostname[0], nil
		}
	}
	return "", errors.New("no hostname in metadata")
}

func (s *RPC) updateAgentLastWork(agent *model.Agent) error {
	// only update agent.LastWork if not recently updated
	if time.Unix(agent.LastWork, 0).Add(updateAgentLastWorkDelay).After(time.Now()) {
		return nil
	}

	agent.LastWork = time.Now().Unix()
	if err := s.store.AgentUpdate(agent); err != nil {
		return err
	}

	return nil
}

// Helper to publish agent stats update.
func (s *RPC) publishAgentStats(ctx context.Context, agentID int64) {
	// Get agent info
	agent, err := s.store.AgentFind(agentID)
	if err != nil || agent == nil {
		return
	}
	// Count running workflows for this agent
	info := s.queue.Info(ctx)
	running := 0
	for _, task := range info.Running {
		if task.AgentID == agentID {
			running++
		}
	}
	agentWithStats := model.AgentWithStats{
		Agent:            *agent,
		RunningWorkflows: running,
	}
	data, err := json.Marshal(map[string]any{"agent": agentWithStats})
	if err != nil {
		return
	}
	s.pubsub.Publish(pubsub.Message{Data: data})
	log.Debug().Interface("agent_event", agentWithStats).Msg("Publishing agent stats event")
}

// Copyright 2022 crow Authors
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

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/pipeline/rpc"
	"codefloe.com/crowci/crow/v5/pipeline/rpc/proto"
	"codefloe.com/crowci/crow/v5/server/logging"
	"codefloe.com/crowci/crow/v5/server/proclog"
	"codefloe.com/crowci/crow/v5/server/pubsub"
	"codefloe.com/crowci/crow/v5/server/queue"
	"codefloe.com/crowci/crow/v5/server/store"
	"codefloe.com/crowci/crow/v5/version"
)

// CrowServer is a grpc server implementation.
type CrowServer struct {
	proto.UnimplementedCrowServer
	peer RPC
}

func NewCrowServer(queue queue.Queue, logger logging.Log, pubsub pubsub.Publisher, store store.Store, procLogReceiver *proclog.Receiver) proto.CrowServer {
	peer := RPC{
		store:           store,
		queue:           queue,
		pubsub:          pubsub,
		logger:          logger,
		metrics:         NewGRPCMetrics(),
		procLogReceiver: procLogReceiver,
	}
	return &CrowServer{peer: peer}
}

func (s *CrowServer) Version(_ context.Context, _ *proto.Empty) (*proto.VersionResponse, error) {
	return &proto.VersionResponse{
		GrpcVersion:   proto.Version,
		ServerVersion: version.String(),
	}, nil
}

func (s *CrowServer) Next(c context.Context, req *proto.NextRequest) (*proto.NextResponse, error) {
	filter := rpc.Filter{
		Labels: req.GetFilter().GetLabels(),
	}

	res := new(proto.NextResponse)
	pipeline, err := s.peer.Next(c, filter)
	if err != nil || pipeline == nil {
		return res, err
	}

	res.Workflow = new(proto.Workflow)
	res.Workflow.Id = pipeline.ID
	res.Workflow.Timeout = pipeline.Timeout
	res.Workflow.Payload, err = json.Marshal(pipeline.Config)

	return res, err
}

func (s *CrowServer) Init(c context.Context, req *proto.InitRequest) (*proto.Empty, error) {
	state := rpc.WorkflowState{
		Started:  req.GetState().GetStarted(),
		Finished: req.GetState().GetFinished(),
		Error:    req.GetState().GetError(),
	}
	res := new(proto.Empty)
	err := s.peer.Init(c, req.GetId(), state)
	return res, err
}

func (s *CrowServer) Update(c context.Context, req *proto.UpdateRequest) (*proto.Empty, error) {
	state := rpc.StepState{
		StepUUID: req.GetState().GetStepUuid(),
		Started:  req.GetState().GetStarted(),
		Finished: req.GetState().GetFinished(),
		Exited:   req.GetState().GetExited(),
		Error:    req.GetState().GetError(),
		ExitCode: int(req.GetState().GetExitCode()),
	}
	res := new(proto.Empty)
	err := s.peer.Update(c, req.GetId(), state)
	return res, err
}

func (s *CrowServer) Done(c context.Context, req *proto.DoneRequest) (*proto.Empty, error) {
	state := rpc.WorkflowState{
		Started:  req.GetState().GetStarted(),
		Finished: req.GetState().GetFinished(),
		Error:    req.GetState().GetError(),
	}
	res := new(proto.Empty)
	err := s.peer.Done(c, req.GetId(), state)
	return res, err
}

func (s *CrowServer) Wait(c context.Context, req *proto.WaitRequest) (*proto.Empty, error) {
	res := new(proto.Empty)
	err := s.peer.Wait(c, req.GetId())
	return res, err
}

func (s *CrowServer) Extend(c context.Context, req *proto.ExtendRequest) (*proto.Empty, error) {
	res := new(proto.Empty)
	err := s.peer.Extend(c, req.GetId())
	return res, err
}

func (s *CrowServer) Log(c context.Context, req *proto.LogRequest) (*proto.Empty, error) {
	var (
		entries  []*rpc.LogEntry
		stepUUID string
	)

	write := func() error {
		if len(entries) > 0 {
			if err := s.peer.Log(c, stepUUID, entries); err != nil {
				log.Error().Err(err).Msg("could not write log entries")
				return err
			}
		}
		return nil
	}

	for _, reqEntry := range req.GetLogEntries() {
		entry := &rpc.LogEntry{
			Data:     reqEntry.GetData(),
			Line:     int(reqEntry.GetLine()),
			Time:     reqEntry.GetTime(),
			StepUUID: reqEntry.GetStepUuid(),
			Type:     int(reqEntry.GetType()),
		}
		if entry.StepUUID != stepUUID {
			_ = write()
			stepUUID = entry.StepUUID
			entries = entries[:0]
		}
		entries = append(entries, entry)
	}

	res := new(proto.Empty)
	err := write()
	return res, err
}

func (s *CrowServer) RegisterAgent(c context.Context, req *proto.RegisterAgentRequest) (*proto.RegisterAgentResponse, error) {
	res := new(proto.RegisterAgentResponse)
	agentInfo := req.GetInfo()
	agentID, agentName, err := s.peer.RegisterAgent(c, rpc.AgentInfo{
		Version:       agentInfo.GetVersion(),
		BuildDate:     agentInfo.GetBuildDate(),
		Platform:      agentInfo.GetPlatform(),
		Backend:       agentInfo.GetBackend(),
		Capacity:      int(agentInfo.GetCapacity()),
		CustomLabels:  agentInfo.GetCustomLabels(),
		Priority:      agentInfo.GetPriority(),
		LimitMem:      agentInfo.GetLimitMem(),
		LimitCPUQuota: agentInfo.GetLimitCpuQuota(),
	})
	res.AgentId = agentID
	res.AgentName = agentName
	return res, err
}

func (s *CrowServer) UnregisterAgent(ctx context.Context, _ *proto.Empty) (*proto.Empty, error) {
	err := s.peer.UnregisterAgent(ctx)
	return new(proto.Empty), err
}

func (s *CrowServer) ReportHealth(c context.Context, req *proto.ReportHealthRequest) (*proto.ReportHealthResponse, error) {
	res := new(proto.ReportHealthResponse)
	cleanupConfig, err := s.peer.ReportHealth(c, req.GetStatus(), req.GetWorkflowPollingHealthy())
	if err != nil {
		return res, err
	}
	if cleanupConfig != nil {
		res.DockerCleanupConfig = &proto.DockerCleanupConfig{
			Enabled:               cleanupConfig.Enabled,
			StaleThresholdSeconds: cleanupConfig.StaleThresholdSeconds,
			IntervalSeconds:       cleanupConfig.IntervalSeconds,
			RunNow:                cleanupConfig.RunNow,
		}
	}
	return res, nil
}

func (s *CrowServer) ReportDockerCleanup(c context.Context, req *proto.DockerCleanupResultRequest) (*proto.Empty, error) {
	res := new(proto.Empty)
	err := s.peer.ReportDockerCleanup(c, rpc.DockerCleanupResult{
		ContainersFound:   req.GetContainersFound(),
		ContainersRemoved: req.GetContainersRemoved(),
		NetworksFound:     req.GetNetworksFound(),
		NetworksRemoved:   req.GetNetworksRemoved(),
		VolumesFound:      req.GetVolumesFound(),
		VolumesRemoved:    req.GetVolumesRemoved(),
		Success:           req.GetSuccess(),
		Error:             req.GetError(),
		DurationMs:        req.GetDurationMs(),
		Manual:            req.GetManual(),
	})
	return res, err
}

func (s *CrowServer) StreamProcLogs(stream proto.Crow_StreamProcLogsServer) error {
	return s.peer.StreamProcLogs(stream)
}

func (s *CrowServer) ClaimTask(c context.Context, req *proto.ClaimTaskRequest) (*proto.ClaimTaskResponse, error) {
	res := new(proto.ClaimTaskResponse)
	workflow, err := s.peer.ClaimTask(c, req.GetTaskId())
	if err != nil || workflow == nil {
		return res, err
	}

	res.Workflow = new(proto.Workflow)
	res.Workflow.Id = workflow.ID
	res.Workflow.Timeout = workflow.Timeout
	res.Workflow.Payload, err = json.Marshal(workflow.Config)

	return res, err
}

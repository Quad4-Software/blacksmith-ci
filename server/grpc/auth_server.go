// Copyright 2023 Woodpecker Authors
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
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	grpcMetadata "google.golang.org/grpc/metadata"

	"codefloe.com/crowci/crow/v6/pipeline/rpc/proto"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

type CrowAuthServer struct {
	proto.UnimplementedCrowAuthServer
	jwtManager       *JWTManager
	agentMasterToken string
	store            store.Store
}

func NewCrowAuthServer(jwtManager *JWTManager, agentMasterToken string, store store.Store) *CrowAuthServer {
	return &CrowAuthServer{jwtManager: jwtManager, agentMasterToken: agentMasterToken, store: store}
}

func (s *CrowAuthServer) Auth(ctx context.Context, req *proto.AuthRequest) (*proto.AuthResponse, error) {
	hostname := hostnameFromContext(ctx)

	log.Trace().
		Int64("agent-id", req.AgentId).
		Bool("has-agent-token", req.AgentToken != "").
		Str("hostname", hostname).
		Msg("Server received Auth request")

	// Determine if this is an agent or autoscaler by checking which table has the token
	entity, entityType, err := s.authenticateByToken(req.AgentId, req.AgentToken, hostname)
	if err != nil {
		log.Trace().
			Int64("id", req.AgentId).
			Str("type", entityType).
			Err(err).
			Msg("Authentication failed")
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	log.Trace().
		Int64("id", entity.ID).
		Str("name", entity.Name).
		Str("type", entityType).
		Msg("Authenticated successfully, generating JWT")

	accessToken, err := s.jwtManager.Generate(entity.ID)
	if err != nil {
		log.Trace().
			Int64("id", entity.ID).
			Str("type", entityType).
			Err(err).
			Msg("Failed to generate JWT")
		return nil, err
	}

	response := &proto.AuthResponse{
		Status:      "ok",
		AgentId:     entity.ID,
		AccessToken: accessToken,
	}

	log.Trace().
		Int64("id", entity.ID).
		Str("type", entityType).
		Msg("Auth response sent successfully")

	return response, nil
}

// AuthEntity represents either an agent or autoscaler for authentication.
type AuthEntity struct {
	ID      int64
	Name    string
	Token   string
	OwnerID int64
	OrgIDs  model.JSONSliceInt64
}

// authenticateByToken tries to authenticate as either an agent or autoscaler
// Returns the entity, entity type ("agent" or "autoscaler"), and any error.
func (s *CrowAuthServer) authenticateByToken(id int64, token, hostname string) (*AuthEntity, string, error) {
	// Try global agent secret first
	if s.agentMasterToken != "" && token == s.agentMasterToken {
		return s.authenticateWithAgentMasterToken(id, hostname)
	}

	// Try individual agent token
	if agent, err := s.authenticateAsAgent(token); err == nil {
		return agent, "agent", nil
	}

	// Try autoscaler token
	if autoscaler, err := s.authenticateAsAutoscaler(token); err == nil {
		return autoscaler, "autoscaler", nil
	}

	return nil, "", fmt.Errorf("token not found in agent or autoscaler tables")
}

func (s *CrowAuthServer) authenticateWithAgentMasterToken(id int64, hostname string) (*AuthEntity, string, error) {
	if id == -1 {
		// Re-bind to an existing system agent for this hostname if one exists.
		// Master-token agents share a single token, so without a persisted agent
		// ID they would otherwise create a fresh orphaned row on every reconnect
		// (e.g. container restarts without persistent config). Binding by hostname
		// keeps the agent visible under a single, stable row.
		if hostname != "" {
			agent, err := s.store.AgentFindSystemByHostname(hostname)
			if err == nil {
				log.Debug().Int64("agent_id", agent.ID).Str("hostname", hostname).
					Msg("re-binding system agent by hostname")
				return authEntityFromAgent(agent), "agent", nil
			}
			if !errors.Is(err, types.RecordNotExist) {
				return nil, "", err
			}
		}

		// Create new system agent
		agent := &model.Agent{
			OwnerID:  model.IDNotSet,
			Token:    s.agentMasterToken,
			Capacity: -1,
		}
		if err := s.store.AgentCreate(agent); err != nil {
			log.Error().Err(err).Msg("error creating system agent")
			return nil, "", err
		}
		return authEntityFromAgent(agent), "agent", nil
	}

	// Find existing agent by ID
	agent, err := s.store.AgentFind(id)
	if err != nil {
		if errors.Is(err, types.RecordNotExist) {
			// Agent ID no longer exists (e.g. after a server upgrade or database reset).
			// Re-register as a new agent instead of failing.
			log.Warn().Int64("agent_id", id).Msg("agent ID not found in database, re-registering as new agent")
			return s.authenticateWithAgentMasterToken(-1, hostname)
		}
		return nil, "", err
	}

	return authEntityFromAgent(agent), "agent", nil
}

func authEntityFromAgent(agent *model.Agent) *AuthEntity {
	return &AuthEntity{
		ID:      agent.ID,
		Name:    agent.Name,
		Token:   agent.Token,
		OwnerID: agent.OwnerID,
		OrgIDs:  agent.OrgIDs,
	}
}

// hostnameFromContext extracts the agent hostname from the gRPC request metadata.
// It returns an empty string when no hostname was provided.
func hostnameFromContext(ctx context.Context) string {
	md, ok := grpcMetadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if values := md.Get("hostname"); len(values) > 0 {
		return values[0]
	}
	return ""
}

func (s *CrowAuthServer) authenticateAsAgent(token string) (*AuthEntity, error) {
	agent, err := s.store.AgentFindByToken(token)
	if err != nil {
		return nil, err
	}

	return &AuthEntity{
		ID:      agent.ID,
		Name:    agent.Name,
		Token:   agent.Token,
		OwnerID: agent.OwnerID,
		OrgIDs:  agent.OrgIDs,
	}, nil
}

func (s *CrowAuthServer) authenticateAsAutoscaler(token string) (*AuthEntity, error) {
	autoscaler, err := s.store.AutoscalerFindByToken(token)
	if err != nil {
		return nil, err
	}

	var orgIDs model.JSONSliceInt64
	if autoscaler.OrgID != model.IDNotSet {
		orgIDs = model.JSONSliceInt64{autoscaler.OrgID}
	}
	return &AuthEntity{
		ID:      autoscaler.ID,
		Name:    autoscaler.Name,
		Token:   autoscaler.Token,
		OwnerID: autoscaler.OwnerID,
		OrgIDs:  orgIDs,
	}, nil
}

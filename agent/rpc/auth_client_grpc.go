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

package rpc

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"codeberg.org/crowci/crow/v4/pipeline/rpc/proto"
)

const (
	authClientTimeout     = time.Second * 5
	connectionWaitTimeout = 2 * time.Second
)

type AuthClient struct {
	client     proto.CrowAuthClient
	conn       *grpc.ClientConn
	agentToken string
	agentID    int64
}

func NewAuthGrpcClient(conn *grpc.ClientConn, agentToken string, agentID int64) *AuthClient {
	client := new(AuthClient)
	client.client = proto.NewCrowAuthClient(conn)
	client.conn = conn
	client.agentToken = agentToken
	client.agentID = agentID
	return client
}

// UpdateAgentID updates the agent ID used for authentication.
// This is called after re-registration when the agent gets a new ID from the server.
func (c *AuthClient) UpdateAgentID(agentID int64) {
	c.agentID = agentID
	log.Debug().Int64("new_agent_id", agentID).Msg("updated auth client with new agent ID")
}

func (c *AuthClient) Auth(ctx context.Context) (string, int64, error) {
	log.Trace().
		Int64("agent-id", c.agentID).
		Dur("timeout", authClientTimeout).
		Msg("Starting authentication request to server")

	// Add connection state logging for debugging
	if c.conn != nil {
		state := c.conn.GetState()
		log.Trace().
			Str("connection-state", state.String()).
			Str("target", c.conn.Target()).
			Msg("gRPC connection state before Auth RPC")

		// If not connected, try to wait briefly for connection
		if state != connectivity.Ready {
			log.Trace().Msg("Connection not ready, waiting for state change...")
			waitCtx, waitCancel := context.WithTimeout(ctx, connectionWaitTimeout)
			c.conn.WaitForStateChange(waitCtx, state)
			waitCancel()
			newState := c.conn.GetState()
			log.Trace().
				Str("old-state", state.String()).
				Str("new-state", newState.String()).
				Msg("Connection state after wait")
		}
	}

	ctx, cancel := context.WithTimeout(ctx, authClientTimeout)
	defer cancel()

	req := &proto.AuthRequest{
		AgentToken: c.agentToken,
		AgentId:    c.agentID,
	}

	log.Trace().Msg("Sending Auth RPC to server")
	res, err := c.client.Auth(ctx, req)
	if err != nil {
		log.Trace().
			Err(err).
			Int64("agent-id", c.agentID).
			Msg("Auth RPC failed")
		return "", -1, err
	}

	c.agentID = res.GetAgentId()

	log.Trace().
		Int64("agent-id", c.agentID).
		Bool("has-access-token", res.GetAccessToken() != "").
		Msg("Auth RPC succeeded")

	return res.GetAccessToken(), c.agentID, nil
}

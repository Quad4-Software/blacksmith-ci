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

package proto

import "fmt"

// Version is the protocol version of the crow.proto file.
// IMPORTANT: increase by 1 each time crow.proto changes.
// Bumping it no longer disconnects older agents: it only lets each side tell
// which fields and RPCs the other one understands.
const Version int32 = 20

// MinAgentVersion is the oldest agent protocol version the server accepts.
// Raise it only when a change breaks older agents, such as removing or
// renaming a field or RPC, or changing what an existing field means.
const MinAgentVersion int32 = 19

// MinServerVersion is the oldest server protocol version the agent accepts.
// Raise it only when the agent starts to depend on a server feature that
// older servers lack.
const MinServerVersion int32 = 19

// LegacyVersion is reported in VersionResponse.grpc_version forever.
// Agents built before protocol negotiation refuse to start unless that field
// equals their own version exactly, so changing it would disconnect them.
const LegacyVersion int32 = 19

// ServerProtocol returns the protocol version of the server and the oldest
// agent protocol it accepts.
// Servers predating negotiation only report grpc_version and require an exact match.
func ServerProtocol(res *VersionResponse) (version, minAgent int32) {
	if res.GetProtocolVersion() == 0 {
		return res.GetGrpcVersion(), res.GetGrpcVersion()
	}
	return res.GetProtocolVersion(), res.GetMinAgentProtocolVersion()
}

// AgentProtocol returns the protocol version of an agent.
// Agents predating negotiation do not send it, but they only got past their
// exact version check if they spoke LegacyVersion.
func AgentProtocol(reported int32) int32 {
	if reported != 0 {
		return reported
	}
	return LegacyVersion
}

// CheckServerCompatibility reports whether an agent speaking agentVersion and
// accepting servers from minServer onwards can talk to a server speaking
// serverVersion and accepting agents from minAgent onwards.
func CheckServerCompatibility(serverVersion, minAgent, agentVersion, minServer int32) error {
	if agentVersion < minAgent {
		return fmt.Errorf("agent speaks protocol %d but the server requires at least %d, update the agent",
			agentVersion, minAgent)
	}
	if serverVersion < minServer {
		return fmt.Errorf("server speaks protocol %d but the agent requires at least %d, update the server",
			serverVersion, minServer)
	}
	return nil
}

// CheckAgentCompatibility reports whether the server accepts an agent that
// reported the protocol version reported, where 0 means a legacy agent.
func CheckAgentCompatibility(reported, minAgent int32) error {
	agentVersion := AgentProtocol(reported)
	if agentVersion < minAgent {
		return fmt.Errorf("agent speaks protocol %d but the server requires at least %d, update the agent",
			agentVersion, minAgent)
	}
	return nil
}

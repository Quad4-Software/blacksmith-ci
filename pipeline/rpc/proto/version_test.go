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

package proto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLegacyVersionIsFrozen(t *testing.T) {
	// Agents built before protocol negotiation compare grpc_version against
	// their own version for an exact match. Changing LegacyVersion disconnects them.
	assert.EqualValues(t, 19, LegacyVersion)
	assert.LessOrEqual(t, MinAgentVersion, Version)
	assert.LessOrEqual(t, MinServerVersion, Version)
	assert.LessOrEqual(t, LegacyVersion, Version)
}

func TestServerProtocol(t *testing.T) {
	tests := []struct {
		name         string
		res          *VersionResponse
		wantVersion  int32
		wantMinAgent int32
	}{
		{
			name:         "legacy server requires an exact match",
			res:          &VersionResponse{GrpcVersion: 18},
			wantVersion:  18,
			wantMinAgent: 18,
		},
		{
			name:         "negotiating server reports its range",
			res:          &VersionResponse{GrpcVersion: 19, ProtocolVersion: 23, MinAgentProtocolVersion: 21},
			wantVersion:  23,
			wantMinAgent: 21,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, minAgent := ServerProtocol(tt.res)
			assert.Equal(t, tt.wantVersion, version)
			assert.Equal(t, tt.wantMinAgent, minAgent)
		})
	}
}

func TestCheckServerCompatibility(t *testing.T) {
	tests := []struct {
		name          string
		serverVersion int32
		minAgent      int32
		agentVersion  int32
		minServer     int32
		wantErr       string
	}{
		{name: "same version", serverVersion: 20, minAgent: 19, agentVersion: 20, minServer: 19},
		{name: "older agent within range", serverVersion: 23, minAgent: 19, agentVersion: 20, minServer: 19},
		{name: "newer agent on older server", serverVersion: 20, minAgent: 19, agentVersion: 23, minServer: 19},
		{name: "legacy server at the frozen version", serverVersion: 19, minAgent: 19, agentVersion: 20, minServer: 19},
		{
			name: "agent below the server minimum", serverVersion: 25, minAgent: 22, agentVersion: 21, minServer: 19,
			wantErr: "update the agent",
		},
		{
			name: "server below the agent minimum", serverVersion: 18, minAgent: 18, agentVersion: 20, minServer: 19,
			wantErr: "update the server",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckServerCompatibility(tt.serverVersion, tt.minAgent, tt.agentVersion, tt.minServer)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestCheckAgentCompatibility(t *testing.T) {
	assert.NoError(t, CheckAgentCompatibility(0, LegacyVersion), "legacy agents speak LegacyVersion")
	assert.Error(t, CheckAgentCompatibility(0, LegacyVersion+1), "legacy agents fall below a raised minimum")
	assert.NoError(t, CheckAgentCompatibility(21, 20))
	assert.ErrorContains(t, CheckAgentCompatibility(20, 21), "update the agent")
}

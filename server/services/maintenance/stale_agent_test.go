// Copyright 2025 Crow Authors
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

package maintenance

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codeberg.org/crowci/crow/v4/server/model"
	mock_store "codeberg.org/crowci/crow/v4/server/store/mocks"
)

func TestRunStaleAgentCleanup(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully deletes stale agents", func(t *testing.T) {
		// Setup
		mockStore := mock_store.NewMockStore(t)
		svc := &service{
			store: mockStore,
		}

		now := time.Now()
		staleDuration := 24 * time.Hour
		cutoffTime := now.Add(-staleDuration)

		// Create test agents
		staleAgent := &model.Agent{
			ID:          1,
			Name:        "stale-agent",
			LastContact: cutoffTime.Add(-1 * time.Hour).Unix(), // 25 hours ago
		}

		activeAgent := &model.Agent{
			ID:          2,
			Name:        "active-agent",
			LastContact: now.Add(-1 * time.Hour).Unix(), // 1 hour ago
		}

		agents := []*model.Agent{staleAgent, activeAgent}

		// Mock expectations
		mockStore.On("AgentList", &model.AgentListOptions{ListOptions: model.ListOptions{All: true}}).Return(agents, nil)
		mockStore.On("AgentDelete", staleAgent).Return(nil)

		// Mock maintenance config operations
		mockStore.On("MaintenanceConfigGetByActionType", "stale_agent_cleanup").Return(&model.MaintenanceConfig{
			ID:         1,
			ActionType: "stale_agent_cleanup",
			Enabled:    true,
			Schedule:   "0 */6 * * *",
		}, nil).Times(2)

		mockStore.On("MaintenanceConfigUpdateByActionType", "stale_agent_cleanup", mock.AnythingOfType("*model.MaintenanceConfig")).Return(nil).Times(2)

		// Mock maintenance log creation
		mockStore.On("MaintenanceLogCreate", mock.AnythingOfType("*model.MaintenanceLog")).Return(nil)

		// Execute
		err := svc.RunStaleAgentCleanup(ctx, staleDuration)

		// Assert
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
	})

	t.Run("handles deletion errors gracefully", func(t *testing.T) {
		// Setup
		mockStore := mock_store.NewMockStore(t)
		svc := &service{
			store: mockStore,
		}

		now := time.Now()
		staleDuration := 24 * time.Hour
		cutoffTime := now.Add(-staleDuration)

		// Create test agents
		staleAgent1 := &model.Agent{
			ID:          1,
			Name:        "stale-agent-1",
			LastContact: cutoffTime.Add(-1 * time.Hour).Unix(),
		}

		staleAgent2 := &model.Agent{
			ID:          2,
			Name:        "stale-agent-2",
			LastContact: cutoffTime.Add(-2 * time.Hour).Unix(),
		}

		agents := []*model.Agent{staleAgent1, staleAgent2}

		// Mock expectations
		mockStore.On("AgentList", &model.AgentListOptions{ListOptions: model.ListOptions{All: true}}).Return(agents, nil)
		mockStore.On("AgentDelete", staleAgent1).Return(assert.AnError) // First deletion fails
		mockStore.On("AgentDelete", staleAgent2).Return(nil)            // Second deletion succeeds

		// Mock maintenance config operations
		mockStore.On("MaintenanceConfigGetByActionType", "stale_agent_cleanup").Return(&model.MaintenanceConfig{
			ID:         1,
			ActionType: "stale_agent_cleanup",
			Enabled:    true,
			Schedule:   "0 */6 * * *",
		}, nil).Times(2)

		mockStore.On("MaintenanceConfigUpdateByActionType", "stale_agent_cleanup", mock.AnythingOfType("*model.MaintenanceConfig")).Return(nil).Times(2)

		// Mock maintenance log creation
		mockStore.On("MaintenanceLogCreate", mock.AnythingOfType("*model.MaintenanceLog")).Return(nil)

		// Execute
		err := svc.RunStaleAgentCleanup(ctx, staleDuration)

		// Assert - should return error but still process all agents
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "1 errors")
		mockStore.AssertExpectations(t)
	})

	t.Run("deletes agents that never connected and are old", func(t *testing.T) {
		// Setup
		mockStore := mock_store.NewMockStore(t)
		svc := &service{
			store: mockStore,
		}

		now := time.Now()
		staleDuration := 24 * time.Hour
		cutoffTime := now.Add(-staleDuration)

		// Create test agent that never connected but was created 25 hours ago
		oldNeverConnectedAgent := &model.Agent{
			ID:          1,
			Name:        "old-never-connected-agent",
			LastContact: 0,                                     // Never connected
			Created:     cutoffTime.Add(-1 * time.Hour).Unix(), // Created 25 hours ago
		}

		// Create test agent that never connected but was created recently
		newNeverConnectedAgent := &model.Agent{
			ID:          2,
			Name:        "new-never-connected-agent",
			LastContact: 0,                              // Never connected
			Created:     now.Add(-1 * time.Hour).Unix(), // Created 1 hour ago
		}

		agents := []*model.Agent{oldNeverConnectedAgent, newNeverConnectedAgent}

		// Mock expectations
		mockStore.On("AgentList", &model.AgentListOptions{ListOptions: model.ListOptions{All: true}}).Return(agents, nil)
		// Should delete the old agent that never connected
		mockStore.On("AgentDelete", oldNeverConnectedAgent).Return(nil)
		// Should NOT delete the recently created agent

		// Mock maintenance config operations
		mockStore.On("MaintenanceConfigGetByActionType", "stale_agent_cleanup").Return(&model.MaintenanceConfig{
			ID:         1,
			ActionType: "stale_agent_cleanup",
			Enabled:    true,
			Schedule:   "0 */6 * * *",
		}, nil).Times(2)

		mockStore.On("MaintenanceConfigUpdateByActionType", "stale_agent_cleanup", mock.AnythingOfType("*model.MaintenanceConfig")).Return(nil).Times(2)

		// Mock maintenance log creation
		mockStore.On("MaintenanceLogCreate", mock.AnythingOfType("*model.MaintenanceLog")).Return(nil)

		// Execute
		err := svc.RunStaleAgentCleanup(ctx, staleDuration)

		// Assert
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)
	})

	t.Run("deletes agents from real database scenario - 3 day old never connected agents", func(t *testing.T) {
		// This reproduces the exact scenario from the user's database
		// where agents were created 3+ days ago but never connected
		mockStore := mock_store.NewMockStore(t)
		svc := &service{
			store: mockStore,
		}

		// Use 24 hour stale duration (default)
		staleDuration := 24 * time.Hour

		// Use real current time since the production code uses time.Now()
		now := time.Now()

		// Create agents that are 3+ days old and should be deleted
		agent1 := &model.Agent{
			ID:          45,
			Name:        "",
			LastContact: 0,                               // Never connected
			Created:     now.Add(-72 * time.Hour).Unix(), // 3 days ago
		}

		agent2 := &model.Agent{
			ID:          36,
			Name:        "",
			LastContact: 0,
			Created:     now.Add(-73 * time.Hour).Unix(), // 3+ days ago
		}

		agent3 := &model.Agent{
			ID:          49,
			Name:        "",
			LastContact: 0,
			Created:     now.Add(-74 * time.Hour).Unix(), // 3+ days ago
		}

		// Mix in some agents that should NOT be deleted
		recentAgent := &model.Agent{
			ID:          100,
			Name:        "recent-agent",
			LastContact: 0,
			Created:     now.Add(-1 * time.Hour).Unix(), // Created 1 hour ago
		}

		activeAgent := &model.Agent{
			ID:          101,
			Name:        "active-agent",
			LastContact: now.Add(-30 * time.Minute).Unix(), // Contacted 30 min ago
			Created:     now.Add(-48 * time.Hour).Unix(),
		}

		agents := []*model.Agent{agent1, agent2, agent3, recentAgent, activeAgent}

		// Mock expectations
		mockStore.On("AgentList", &model.AgentListOptions{ListOptions: model.ListOptions{All: true}}).Return(agents, nil)

		// Should delete the 3 old agents that never connected
		// Use MatchedBy to match on agent ID instead of exact struct equality
		mockStore.On("AgentDelete", mock.MatchedBy(func(a *model.Agent) bool {
			return a.ID == 45 || a.ID == 36 || a.ID == 49
		})).Return(nil).Times(3)
		// Should NOT delete recentAgent (100) or activeAgent (101)

		// Mock maintenance config operations
		mockStore.On("MaintenanceConfigGetByActionType", "stale_agent_cleanup").Return(&model.MaintenanceConfig{
			ID:         1,
			ActionType: "stale_agent_cleanup",
			Enabled:    true,
			Schedule:   "0 */6 * * *",
		}, nil).Times(2)

		mockStore.On("MaintenanceConfigUpdateByActionType", "stale_agent_cleanup", mock.AnythingOfType("*model.MaintenanceConfig")).Return(nil).Times(2)

		// Mock maintenance log creation
		mockStore.On("MaintenanceLogCreate", mock.AnythingOfType("*model.MaintenanceLog")).Return(nil)

		// Execute with context that has the simulated "now" time
		err := svc.RunStaleAgentCleanup(ctx, staleDuration)

		// Assert
		assert.NoError(t, err)
		mockStore.AssertExpectations(t)

		// Verify that exactly 3 agents were attempted to be deleted
		mockStore.AssertNumberOfCalls(t, "AgentDelete", 3)
	})
}

func TestGetStaleAgentDuration(t *testing.T) {
	t.Run("returns default duration when env var not set", func(t *testing.T) {
		t.Setenv("CROW_MAINTENANCE_STALE_AGENT_DURATION", "")

		duration := GetStaleAgentDuration()

		assert.Equal(t, 24*time.Hour, duration)
	})

	t.Run("parses valid duration from env var", func(t *testing.T) {
		t.Setenv("CROW_MAINTENANCE_STALE_AGENT_DURATION", "48h")

		duration := GetStaleAgentDuration()

		assert.Equal(t, 48*time.Hour, duration)
	})

	t.Run("returns default on invalid duration", func(t *testing.T) {
		t.Setenv("CROW_MAINTENANCE_STALE_AGENT_DURATION", "invalid")

		duration := GetStaleAgentDuration()

		assert.Equal(t, 24*time.Hour, duration)
	})
}

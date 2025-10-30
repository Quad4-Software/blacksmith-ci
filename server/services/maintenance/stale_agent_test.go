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

	t.Run("skips agents with no last contact", func(t *testing.T) {
		// Setup
		mockStore := mock_store.NewMockStore(t)
		svc := &service{
			store: mockStore,
		}

		staleDuration := 24 * time.Hour

		// Create test agent with no last contact
		newAgent := &model.Agent{
			ID:          1,
			Name:        "new-agent",
			LastContact: 0, // Never connected
		}

		agents := []*model.Agent{newAgent}

		// Mock expectations
		mockStore.On("AgentList", &model.AgentListOptions{ListOptions: model.ListOptions{All: true}}).Return(agents, nil)
		// Should not attempt to delete agent with LastContact = 0

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
		mockStore.AssertNotCalled(t, "AgentDelete", mock.Anything)
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

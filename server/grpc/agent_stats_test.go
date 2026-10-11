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

package grpc

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pubsub"
	pubsub_mocks "codefloe.com/crowci/crow/v6/server/pubsub/mocks"
	"codefloe.com/crowci/crow/v6/server/queue"
	queue_mocks "codefloe.com/crowci/crow/v6/server/queue/mocks"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

// TestPublishAgentStatsOmitsToken is not parallel because it swaps the global logger.
func TestPublishAgentStatsOmitsToken(t *testing.T) {
	const token = "SECRET-AGENT-TOKEN-DO-NOT-LEAK"

	var logs bytes.Buffer
	prevLogger := log.Logger
	log.Logger = zerolog.New(&logs).Level(zerolog.DebugLevel)
	t.Cleanup(func() { log.Logger = prevLogger })

	mockStore := store_mocks.NewMockStore(t)
	mockStore.On("AgentFind", int64(7)).Return(&model.Agent{ID: 7, Name: "stats-agent", Token: token}, nil)

	mockQueue := queue_mocks.NewMockQueue(t)
	mockQueue.On("Info", mock.Anything).Return(queue.InfoT{
		Running: []*model.Task{{AgentID: 7}, {AgentID: 8}},
	})

	var published []pubsub.Message
	mockPubsub := pubsub_mocks.NewMockPublisher(t)
	mockPubsub.On("Publish", mock.Anything).Run(func(args mock.Arguments) {
		msg, ok := args.Get(0).(pubsub.Message)
		require.True(t, ok)
		published = append(published, msg)
	}).Return()

	s := &RPC{store: mockStore, queue: mockQueue, pubsub: mockPubsub}
	s.publishAgentStats(t.Context(), 7)

	require.Len(t, published, 1)
	data := string(published[0].Data)
	assert.NotContains(t, data, token)
	assert.NotContains(t, data, `"token"`)

	var event struct {
		Agent map[string]any `json:"agent"`
	}
	require.NoError(t, json.Unmarshal(published[0].Data, &event))
	assert.Equal(t, "stats-agent", event.Agent["name"])
	assert.InDelta(t, 1, event.Agent["running_workflows"], 0)

	assert.Contains(t, logs.String(), "Publishing agent stats event")
	assert.NotContains(t, logs.String(), token)
}

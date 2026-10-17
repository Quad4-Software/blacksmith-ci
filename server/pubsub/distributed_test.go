// Copyright 2026 Crow CI Authors
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

package pubsub

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestDistributedPollDeliversSameSecondMessagesOnce(t *testing.T) {
	const start int64 = 1_000

	mockStore := store_mocks.NewMockStore(t)
	p := &DistributedPublisher{
		store:      mockStore,
		instanceID: "self",
		subs:       make(map[*Receiver]context.Context),
	}
	p.resetPollState(start)

	var (
		mu        sync.Mutex
		delivered = map[string]int{}
	)
	receiver := Receiver(func(m Message) {
		mu.Lock()
		delivered[m.ID]++
		mu.Unlock()
	})
	p.subs[&receiver] = t.Context()

	first := &model.DistributedMessage{ID: "first", CreatedAt: start, InstanceID: "other"}
	// Inserted later within the same second as first, after the first poll.
	sameSecond := &model.DistributedMessage{ID: "same-second", CreatedAt: start, InstanceID: "other"}
	own := &model.DistributedMessage{ID: "own", CreatedAt: start, InstanceID: "self"}
	later := &model.DistributedMessage{ID: "later", CreatedAt: start + 10, InstanceID: "other"}

	mockStore.On("PubSubMessageListSince", start).Return([]*model.DistributedMessage{first, own}, nil).Once()
	mockStore.On("PubSubMessageListSince", start).Return([]*model.DistributedMessage{first, own, sameSecond}, nil).Once()
	mockStore.On("PubSubMessageListSince", start).Return([]*model.DistributedMessage{first, own, sameSecond, later}, nil).Once()
	// The cursor follows the newest created_at seen, minus the lookback window.
	mockStore.On("PubSubMessageListSince", start+10-pollLookback).Return([]*model.DistributedMessage{later}, nil).Twice()
	// An empty result must not move the cursor backwards.
	mockStore.On("PubSubMessageListSince", start+10-pollLookback).Return([]*model.DistributedMessage{}, nil).Once()

	for range 6 {
		p.pollMessages()
	}

	assert.Equal(t, start+10, p.lastPolled)
	// IDs older than the poll window are evicted from the seen-set.
	assert.Equal(t, map[string]int64{"later": start + 10}, p.seen)

	expected := map[string]int{"first": 1, "same-second": 1, "later": 1}
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(delivered) == len(expected)
	}, time.Second, 10*time.Millisecond)
	// Give any duplicate deliveries a chance to show up.
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, expected, delivered)
}

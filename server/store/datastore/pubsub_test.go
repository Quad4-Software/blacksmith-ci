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

package datastore

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
)

func TestPubSubMessageListSinceIsInclusive(t *testing.T) {
	store, closer := newTestStore(t, new(model.DistributedMessage))
	defer closer()

	for _, msg := range []*model.DistributedMessage{
		{ID: "before", CreatedAt: 99},
		{ID: "at-b", CreatedAt: 100},
		{ID: "at-a", CreatedAt: 100},
		{ID: "after", CreatedAt: 101},
	} {
		assert.NoError(t, store.PubSubMessageInsert(msg))
	}

	messages, err := store.PubSubMessageListSince(100)
	assert.NoError(t, err)
	ids := make([]string, 0, len(messages))
	for _, msg := range messages {
		ids = append(ids, msg.ID)
	}
	assert.ElementsMatch(t, []string{"at-a", "at-b", "after"}, ids)
}

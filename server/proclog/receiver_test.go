// Copyright 2021 Woodpecker Authors
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

package proclog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v5/server/model"
	store_mocks "codefloe.com/crowci/crow/v5/server/store/mocks"
)

func TestReceiverIngest(t *testing.T) {
	storeMock := store_mocks.NewMockStore(t)
	r := NewReceiver(storeMock)

	storeMock.On("ProcLogInsert", mock.Anything, mock.MatchedBy(func(entries []*model.ProcLogEntry) bool {
		return len(entries) == 2 &&
			entries[0].SourceType == SourceTypeAgent &&
			entries[0].SourceID == 42 &&
			entries[0].Line == "line one"
	})).Return(nil).Once()

	err := r.Ingest(context.Background(), SourceTypeAgent, 42, []*model.ProcLogEntry{
		{Time: 1, Level: "info", Line: "line one"},
		{Time: 2, Level: "info", Line: "line two"},
	})
	require.NoError(t, err)
}

func TestReceiverIngestSetsSourceFields(t *testing.T) {
	storeMock := store_mocks.NewMockStore(t)
	r := NewReceiver(storeMock)

	var captured []*model.ProcLogEntry
	storeMock.On("ProcLogInsert", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			entries, ok := args.Get(1).([]*model.ProcLogEntry)
			require.True(t, ok, "unexpected argument type")
			captured = entries
		}).
		Return(nil)

	err := r.Ingest(context.Background(), SourceTypeAutoscaler, 7,
		[]*model.ProcLogEntry{{Time: 10, Level: "warn", Line: "x"}})
	require.NoError(t, err)
	require.Len(t, captured, 1)
	assert.Equal(t, SourceTypeAutoscaler, captured[0].SourceType)
	assert.Equal(t, int64(7), captured[0].SourceID)
}

func TestReceiverIngestEmpty(t *testing.T) {
	storeMock := store_mocks.NewMockStore(t)
	r := NewReceiver(storeMock)
	// No store call expected.
	require.NoError(t, r.Ingest(context.Background(), SourceTypeAgent, 1, nil))
}

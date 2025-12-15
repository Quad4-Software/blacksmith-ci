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

package logging

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codeberg.org/crowci/crow/v5/server/model"
)

func TestLogging(t *testing.T) {
	var (
		wg sync.WaitGroup

		testStepID = int64(123)
		testEntry  = &model.LogEntry{
			Data: []byte("test"),
		}
	)

	ctx, cancel := context.WithCancelCause(
		t.Context(),
	)

	receiver := make(LogChan, 10)
	defer close(receiver)

	go func() {
		for range receiver {
			wg.Done()
		}
	}()

	logger := New()
	assert.NoError(t, logger.Open(ctx, testStepID))
	go func() {
		assert.NoError(t, logger.Tail(ctx, testStepID, receiver))
	}()
	go func() {
		assert.NoError(t, logger.Tail(ctx, testStepID, receiver))
	}()

	<-time.After(500 * time.Millisecond)

	wg.Add(4)
	go func() {
		assert.NoError(t, logger.Write(ctx, testStepID, []*model.LogEntry{testEntry}))
		assert.NoError(t, logger.Write(ctx, testStepID, []*model.LogEntry{testEntry}))
	}()

	wg.Wait()

	wg.Add(1)
	go func() {
		assert.NoError(t, logger.Tail(ctx, testStepID, receiver))
	}()

	<-time.After(500 * time.Millisecond)

	wg.Wait()
	cancel(nil)
}

func TestLog_Close_ExistingStream(t *testing.T) {
	t.Parallel()

	// Setup
	logger := New()
	stepID := int64(123)

	// Open a stream first
	err := logger.Open(t.Context(), stepID)
	assert.NoError(t, err)

	// Close the existing stream
	err = logger.Close(t.Context(), stepID)
	assert.NoError(t, err)

	// Verify stream was removed
	err = logger.Close(t.Context(), stepID)
	assert.NoError(t, err) // Should not return error for already closed stream
}

func TestLog_Close_NonExistentStream(t *testing.T) {
	t.Parallel()

	// Setup
	logger := New()
	stepID := int64(123)

	// Try to close a stream that doesn't exist
	err := logger.Close(t.Context(), stepID)
	assert.NoError(t, err) // Should return nil, not ErrNotFound
}

func TestLog_Close_MultipleCalls(t *testing.T) {
	t.Parallel()

	// Setup
	logger := New()
	stepID := int64(123)

	// Open a stream
	err := logger.Open(t.Context(), stepID)
	assert.NoError(t, err)

	// Close it multiple times
	err = logger.Close(t.Context(), stepID)
	assert.NoError(t, err)

	err = logger.Close(t.Context(), stepID)
	assert.NoError(t, err)

	err = logger.Close(t.Context(), stepID)
	assert.NoError(t, err)
}

func TestLog_Write_ToNonExistentStream(t *testing.T) {
	t.Parallel()

	// Setup
	logger := New()
	stepID := int64(123)

	// Write to a stream that doesn't exist (should auto-create)
	entries := []*model.LogEntry{
		{
			StepID: stepID,
			Line:   1,
			Data:   []byte("test log entry"),
		},
	}

	err := logger.Write(t.Context(), stepID, entries)
	assert.NoError(t, err)

	// Verify stream was created and can be closed
	err = logger.Close(t.Context(), stepID)
	assert.NoError(t, err)
}

func TestLog_Tail_NonExistentStream(t *testing.T) {
	t.Parallel()

	// Setup
	logger := New()
	stepID := int64(123)

	// Try to tail a stream that doesn't exist
	receiver := make(LogChan, 1)
	err := logger.Tail(t.Context(), stepID, receiver)
	assert.Error(t, err)
	assert.Equal(t, ErrNotFound, err)
}

func TestLog_Integration_OpenWriteClose(t *testing.T) {
	t.Parallel()

	// Setup
	logger := New()
	stepID := int64(123)
	ctx := t.Context()

	// Open stream
	err := logger.Open(ctx, stepID)
	assert.NoError(t, err)

	// Write some entries
	entries := []*model.LogEntry{
		{
			StepID: stepID,
			Line:   1,
			Data:   []byte("first log entry"),
		},
		{
			StepID: stepID,
			Line:   2,
			Data:   []byte("second log entry"),
		},
	}

	err = logger.Write(ctx, stepID, entries)
	assert.NoError(t, err)

	// Close stream
	err = logger.Close(ctx, stepID)
	assert.NoError(t, err)

	// Try to close again (should not error)
	err = logger.Close(ctx, stepID)
	assert.NoError(t, err)
}

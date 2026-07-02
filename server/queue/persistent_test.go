// Copyright 2024 Woodpecker Authors
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

package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/store"
)

// fakeInnerQueue is a minimal Queue used to drive persistentQueue tests. Only
// PushAtOnce is exercised; the embedded nil Queue would panic on any other call,
// which keeps the test honest about what it touches.
type fakeInnerQueue struct {
	Queue
	pushAtOnceErr error
	pushed        [][]*model.Task
}

func (f *fakeInnerQueue) PushAtOnce(_ context.Context, tasks []*model.Task) error {
	f.pushed = append(f.pushed, tasks)
	return f.pushAtOnceErr
}

// recordingStore records TaskInsert/TaskDelete calls and can be configured to
// fail the Nth insert to simulate a mid-batch failure.
type recordingStore struct {
	store.Store
	failInsertOnID string
	inserted       []string
	deleted        []string
}

func (s *recordingStore) TaskInsert(task *model.Task) error {
	if task.ID == s.failInsertOnID {
		return errors.New("insert failed")
	}
	s.inserted = append(s.inserted, task.ID)
	return nil
}

func (s *recordingStore) TaskDelete(id string) error {
	s.deleted = append(s.deleted, id)
	return nil
}

func tasks(ids ...string) []*model.Task {
	out := make([]*model.Task, 0, len(ids))
	for _, id := range ids {
		out = append(out, &model.Task{ID: id})
	}
	return out
}

func TestPushAtOnceRollsBackOnMidInsertFailure(t *testing.T) {
	inner := &fakeInnerQueue{}
	st := &recordingStore{failInsertOnID: "b"}
	q := &persistentQueue{Queue: inner, store: st}

	err := q.PushAtOnce(t.Context(), tasks("a", "b", "c"))

	assert.Error(t, err)
	// Only "a" was inserted before "b" failed; it must be rolled back.
	assert.Equal(t, []string{"a"}, st.inserted)
	assert.Equal(t, []string{"a"}, st.deleted)
	// The batch must never reach the underlying queue when persistence failed.
	assert.Empty(t, inner.pushed)
}

func TestPushAtOnceRollsBackWhenInnerPushFails(t *testing.T) {
	inner := &fakeInnerQueue{pushAtOnceErr: errors.New("push failed")}
	st := &recordingStore{}
	q := &persistentQueue{Queue: inner, store: st}

	err := q.PushAtOnce(t.Context(), tasks("a", "b", "c"))

	assert.Error(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, st.inserted)
	// All persisted rows are removed again when the in-memory push fails.
	assert.Equal(t, []string{"a", "b", "c"}, st.deleted)
}

func TestPushAtOnceSuccessKeepsRows(t *testing.T) {
	inner := &fakeInnerQueue{}
	st := &recordingStore{}
	q := &persistentQueue{Queue: inner, store: st}

	err := q.PushAtOnce(t.Context(), tasks("a", "b"))

	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, st.inserted)
	assert.Empty(t, st.deleted)
	assert.Len(t, inner.pushed, 1)
}

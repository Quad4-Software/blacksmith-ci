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

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

// fakeInnerQueue is a minimal Queue used to drive persistentQueue tests. Only
// PushAtOnce is exercised; the embedded nil Queue would panic on any other call,
// which keeps the test honest about what it touches.
type fakeInnerQueue struct {
	Queue
	pushAtOnceErr error
	pushed        [][]*model.Task
	doneErr       error
	done          []string
}

func (f *fakeInnerQueue) PushAtOnce(_ context.Context, tasks []*model.Task) error {
	f.pushed = append(f.pushed, tasks)
	return f.pushAtOnceErr
}

func (f *fakeInnerQueue) Done(_ context.Context, id string, _ model.StatusValue) error {
	f.done = append(f.done, id)
	return f.doneErr
}

// recordingStore records TaskInsert/TaskDelete calls and can be configured to
// fail the Nth insert to simulate a mid-batch failure.
type recordingStore struct {
	store.Store
	failInsertOnID string
	inserted       []string
	deleted        []string
	staleDeleted   int64
	staleSweeps    int
	pending        []*model.Task
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

func (s *recordingStore) TaskDeleteStale() (int64, error) {
	s.staleSweeps++
	return s.staleDeleted, nil
}

func (s *recordingStore) TaskListPending() ([]*model.Task, error) {
	return s.pending, nil
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

func TestDoneDeletesPersistedRow(t *testing.T) {
	inner := &fakeInnerQueue{}
	st := &recordingStore{}
	q := &persistentQueue{Queue: inner, store: st}

	err := q.Done(t.Context(), "a", model.StatusSuccess)

	assert.NoError(t, err)
	assert.Equal(t, []string{"a"}, inner.done)
	// The success path must remove the persisted row, otherwise every
	// completed workflow leaks a tasks-table row until the next restart.
	assert.Equal(t, []string{"a"}, st.deleted)
}

func TestDoneKeepsRowWhenInnerDoneFails(t *testing.T) {
	inner := &fakeInnerQueue{doneErr: errors.New("done failed")}
	st := &recordingStore{}
	q := &persistentQueue{Queue: inner, store: st}

	err := q.Done(t.Context(), "a", model.StatusSuccess)

	assert.Error(t, err)
	// The row must survive so the task can be recovered after a restart.
	assert.Empty(t, st.deleted)
}

func TestWithTaskStoreReconcilesStaleRowsAndLoadsPending(t *testing.T) {
	inner := &fakeInnerQueue{}
	st := &recordingStore{staleDeleted: 3, pending: tasks("a", "b")}

	q := WithTaskStore(t.Context(), inner, st)

	assert.NotNil(t, q)
	// Stale rows (workflows already completed) are swept before loading.
	assert.Equal(t, 1, st.staleSweeps)
	// Pending tasks are pushed back into the queue on startup.
	assert.Len(t, inner.pushed, 1)
	assert.Equal(t, []string{"a", "b"}, []string{inner.pushed[0][0].ID, inner.pushed[0][1].ID})
}

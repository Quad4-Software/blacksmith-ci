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

package datastore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/server/model"
)

func TestWorkflowApprovalEventCreateAndList(t *testing.T) {
	store, closer := newTestStore(t, new(model.Workflow), new(model.WorkflowApprovalEvent), new(model.Pipeline), new(model.Repo))
	defer closer()

	wf := &model.Workflow{Name: "deploy", PID: 1, PipelineID: 42}
	assert.NoError(t, store.WorkflowsCreate([]*model.Workflow{wf}))

	ev := &model.WorkflowApprovalEvent{
		WorkflowID: wf.ID,
		UserID:     7,
		Action:     model.ApprovalActionApproved,
		Reason:     "lgtm",
		CreatedAt:  time.Now().Unix(),
	}
	assert.NoError(t, store.WorkflowApprovalEventCreate(ev))

	events, err := store.WorkflowApprovalEventsForWorkflow(wf.ID)
	assert.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, model.ApprovalActionApproved, events[0].Action)
	assert.Equal(t, "lgtm", events[0].Reason)
}

func TestWorkflowsAwaitingApprovalBefore(t *testing.T) {
	store, closer := newTestStore(t, new(model.Workflow))
	defer closer()

	now := time.Now().Unix()

	expired := &model.Workflow{
		Name: "x", PID: 1, PipelineID: 1,
		RequiresApproval: true, ApprovalState: model.ApprovalPending, ApprovalDeadline: now - 60,
	}
	fresh := &model.Workflow{
		Name: "y", PID: 2, PipelineID: 1,
		RequiresApproval: true, ApprovalState: model.ApprovalPending, ApprovalDeadline: now + 600,
	}
	done := &model.Workflow{
		Name: "z", PID: 3, PipelineID: 1,
		RequiresApproval: true, ApprovalState: model.ApprovalApproved, ApprovalDeadline: now - 60,
	}
	assert.NoError(t, store.WorkflowsCreate([]*model.Workflow{expired, fresh, done}))

	wfs, err := store.WorkflowsAwaitingApprovalBefore(now)
	assert.NoError(t, err)
	assert.Len(t, wfs, 1)
	assert.Equal(t, expired.ID, wfs[0].ID)
}

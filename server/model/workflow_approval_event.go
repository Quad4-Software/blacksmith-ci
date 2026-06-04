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

package model

import "github.com/uptrace/bun"

// ApprovalAction is the verb recorded in a WorkflowApprovalEvent.
type ApprovalAction string //	@name	ApprovalAction

const (
	ApprovalActionApproved ApprovalAction = "approved"
	ApprovalActionDeclined ApprovalAction = "declined"
	ApprovalActionExpired  ApprovalAction = "expired"
)

// WorkflowApprovalEvent is an append-only audit row recording a single
// decision (or auto-expiry) against a workflow approval gate.
type WorkflowApprovalEvent struct {
	bun.BaseModel `bun:"table:workflow_approval_events"`

	ID         int64          `json:"id"          bun:"id,pk,autoincrement"`
	WorkflowID int64          `json:"workflow_id" bun:"workflow_id,notnull"`
	UserID     int64          `json:"user_id"     bun:"user_id"` // 0 if system (timeout)
	Action     ApprovalAction `json:"action"      bun:"action,notnull"`
	Reason     string         `json:"reason"      bun:"reason,type:text"`
	CreatedAt  int64          `json:"created_at"  bun:"created_at,notnull"`
} //	@name	WorkflowApprovalEvent

func (WorkflowApprovalEvent) TableName() string { return "workflow_approval_events" }

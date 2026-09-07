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

import (
	"errors"
	"fmt"
)

// ApprovalState describes the lifecycle of a workflow-level approval gate.
type ApprovalState string //	@name	ApprovalState

const (
	ApprovalNone     ApprovalState = ""         // gate not configured on this workflow
	ApprovalPending  ApprovalState = "pending"  // waiting for a human decision
	ApprovalApproved ApprovalState = "approved" // approved; workflow may proceed
	ApprovalDeclined ApprovalState = "declined" // explicitly declined by an approver
	ApprovalExpired  ApprovalState = "expired"  // auto-declined by the timeout sweeper
)

var ErrInvalidApprovalState = errors.New("invalid approval state")

func (s ApprovalState) Validate() error {
	switch s {
	case ApprovalNone, ApprovalPending, ApprovalApproved, ApprovalDeclined, ApprovalExpired:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidApprovalState, s)
	}
}

// Terminal reports whether the state is final (no further transitions).
func (s ApprovalState) Terminal() bool {
	return s == ApprovalApproved || s == ApprovalDeclined || s == ApprovalExpired
}

// Granted reports whether the gate lets the workflow proceed.
func (s ApprovalState) Granted() bool {
	return s == ApprovalApproved
}

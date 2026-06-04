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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApprovalStateValidate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      ApprovalState
		wantErr bool
	}{
		{ApprovalNone, false},
		{ApprovalPending, false},
		{ApprovalApproved, false},
		{ApprovalDeclined, false},
		{ApprovalExpired, false},
		{ApprovalState("garbage"), true},
	}
	for _, c := range cases {
		err := c.in.Validate()
		if c.wantErr {
			assert.Error(t, err, "for %q", c.in)
		} else {
			assert.NoError(t, err, "for %q", c.in)
		}
	}
}

func TestApprovalStateTerminal(t *testing.T) {
	t.Parallel()
	assert.False(t, ApprovalNone.Terminal())
	assert.False(t, ApprovalPending.Terminal())
	assert.True(t, ApprovalApproved.Terminal())
	assert.True(t, ApprovalDeclined.Terminal())
	assert.True(t, ApprovalExpired.Terminal())
}

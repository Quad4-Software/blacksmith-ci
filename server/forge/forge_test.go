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

package forge

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTokenExpiryToUnix(t *testing.T) {
	tests := []struct {
		name     string
		expiry   time.Time
		expected int64
	}{
		{
			name:     "zero time returns 0",
			expiry:   time.Time{},
			expected: 0,
		},
		{
			name:     "non-zero time returns unix timestamp",
			expiry:   time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
			expected: 1736942400,
		},
		{
			name:     "future time returns correct timestamp",
			expiry:   time.Date(2030, 6, 15, 0, 0, 0, 0, time.UTC),
			expected: 1907712000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := TokenExpiryToUnix(tc.expiry)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTokenExpiryToUnix_ZeroTimeDoesNotReturnNegative(t *testing.T) {
	// This is the critical test case that verifies the fix.
	// time.Time{}.Unix() returns -62135596800 (year 1 AD),
	// but we need 0 to indicate "no expiry".
	zeroTime := time.Time{}
	result := TokenExpiryToUnix(zeroTime)

	assert.Equal(t, int64(0), result, "zero time should return 0, not a negative number")
	assert.GreaterOrEqual(t, result, int64(0), "result should never be negative")
}

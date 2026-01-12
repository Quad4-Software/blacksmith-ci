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

package matrix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatrix(t *testing.T) {
	axis, _ := ParseString(fakeMatrix)
	assert.Len(t, axis, 24)

	set := map[string]bool{}
	for _, perm := range axis {
		set[perm.String()] = true
	}
	assert.Len(t, set, 24)
}

func TestMatrixEmpty(t *testing.T) {
	axis, err := ParseString("")
	assert.NoError(t, err)
	assert.Empty(t, axis)
}

func TestMatrixIncluded(t *testing.T) {
	axis, err := ParseString(fakeMatrixInclude)
	assert.NoError(t, err)
	assert.Len(t, axis, 2)
	assert.Equal(t, "1.5", axis[0]["go_version"])
	assert.Equal(t, "1.6", axis[1]["go_version"])
	assert.Equal(t, "3.4", axis[0]["python_version"])
	assert.Equal(t, "3.4", axis[1]["python_version"])
}

var fakeMatrix = `
matrix:
  go_version:
    - go1
    - go1.2
  python_version:
    - 3.2
    - 3.3
  django_version:
    - 1.7
    - 1.7.1
    - 1.7.2
  redis_version:
    - 2.6
    - 2.8
`

var fakeMatrixInclude = `
matrix:
  include:
    - go_version: 1.5
      python_version: 3.4
    - go_version: 1.6
      python_version: 3.4
`

func TestMatrixLimitExceeded(t *testing.T) {
	// This matrix produces 24 permutations (2*2*3*2=24)
	// With a limit of 10, it should fail
	_, err := ParseStringWithLimit(fakeMatrix, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "matrix permutation limit exceeded")
	assert.Contains(t, err.Error(), "24 permutations requested")
	assert.Contains(t, err.Error(), "maximum is 10")
}

func TestMatrixLimitNotExceeded(t *testing.T) {
	// With a limit of 30, 24 permutations should pass
	axis, err := ParseStringWithLimit(fakeMatrix, 30)
	assert.NoError(t, err)
	assert.Len(t, axis, 24)
}

func TestMatrixUnlimited(t *testing.T) {
	// With a limit of 0, no limit is enforced
	axis, err := ParseStringWithLimit(fakeMatrix, 0)
	assert.NoError(t, err)
	assert.Len(t, axis, 24)
}

func TestMatrixIncludeLimitExceeded(t *testing.T) {
	// The include matrix has 2 items, limit to 1
	_, err := ParseStringWithLimit(fakeMatrixInclude, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "matrix permutation limit exceeded")
}

// Copyright 2023 Woodpecker Authors
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
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/server/store/types"
)

func TestWrapGet(t *testing.T) {
	err := wrapGet(sql.ErrNoRows)
	assert.ErrorIs(t, err, types.RecordNotExist)

	err = wrapGet(errors.New("test err"))
	assert.Equal(t, "TestWrapGet: test err", err.Error())

	err = wrapGet(nil)
	assert.NoError(t, err)
}

// mockResult implements sql.Result for testing.
type mockResult struct{ n int64 }

func (m mockResult) LastInsertId() (int64, error) { return 0, nil }
func (m mockResult) RowsAffected() (int64, error) { return m.n, nil }

func TestWrapDelete(t *testing.T) {
	err := wrapDelete(mockResult{0}, nil)
	assert.ErrorIs(t, err, types.RecordNotExist)

	err = wrapDelete(mockResult{1}, errors.New("test err"))
	assert.Equal(t, "TestWrapDelete: test err", err.Error())

	err = wrapDelete(mockResult{1}, nil)
	assert.NoError(t, err)
}

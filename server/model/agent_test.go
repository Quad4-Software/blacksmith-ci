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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/pipeline"
)

func TestGenerateNewAgentToken(t *testing.T) {
	token1 := GenerateNewAgentToken()
	token2 := GenerateNewAgentToken()

	assert.NotEmpty(t, token1)
	assert.NotEmpty(t, token2)
	assert.NotEqual(t, token1, token2)
	assert.Len(t, token1, 56)
}

func TestAgentIsGlobal(t *testing.T) {
	assert.True(t, (&Agent{OrgIDs: nil}).IsGlobal())
	assert.True(t, (&Agent{OrgIDs: JSONSliceInt64{}}).IsGlobal())
	assert.False(t, (&Agent{OrgIDs: JSONSliceInt64{5}}).IsGlobal())
}

func TestAgentGetServerLabels(t *testing.T) {
	global, err := (&Agent{OrgIDs: nil}).GetServerLabels()
	assert.NoError(t, err)
	assert.Equal(t, "*", global[pipeline.LabelOrgID])

	one, err := (&Agent{OrgIDs: JSONSliceInt64{5}}).GetServerLabels()
	assert.NoError(t, err)
	assert.Equal(t, "5", one[pipeline.LabelOrgID])

	multi, err := (&Agent{OrgIDs: JSONSliceInt64{5, 7}}).GetServerLabels()
	assert.NoError(t, err)
	assert.Equal(t, "5|7", multi[pipeline.LabelOrgID])
}

func TestAgentCanAccessRepo(t *testing.T) {
	repo := &Repo{OrgID: 7}
	assert.True(t, (&Agent{OrgIDs: nil}).CanAccessRepo(repo))
	assert.True(t, (&Agent{OrgIDs: JSONSliceInt64{5, 7}}).CanAccessRepo(repo))
	assert.False(t, (&Agent{OrgIDs: JSONSliceInt64{5}}).CanAccessRepo(repo))
}

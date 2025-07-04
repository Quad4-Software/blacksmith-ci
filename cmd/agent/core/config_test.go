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

package core

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadAgentIDFileNotExists(t *testing.T) {
	assert.EqualValues(t, -1, readAgentConfig("foobar.conf").AgentID)
}

func TestReadAgentIDFileExists(t *testing.T) {
	tmpF, errTmpF := os.CreateTemp(t.TempDir(), "tmp_")
	if !assert.NoError(t, errTmpF) {
		return
	}
	defer os.Remove(tmpF.Name())

	// there is an existing config
	errWrite := os.WriteFile(tmpF.Name(), []byte(`{"agent_id":3}`), 0o644)
	if !assert.NoError(t, errWrite) {
		return
	}

	// read existing config
	actual := readAgentConfig(tmpF.Name())
	assert.EqualValues(t, AgentConfig{3}, actual)

	// update existing config and check
	actual.AgentID = 33
	_ = writeAgentConfig(actual, tmpF.Name())
	actual = readAgentConfig(tmpF.Name())
	assert.EqualValues(t, 33, actual.AgentID)

	tmpF2, errTmpF := os.CreateTemp(t.TempDir(), "tmp_")
	if !assert.NoError(t, errTmpF) {
		return
	}
	defer os.Remove(tmpF2.Name())

	// write new config
	_ = writeAgentConfig(actual, tmpF2.Name())
	actual = readAgentConfig(tmpF2.Name())
	assert.EqualValues(t, 33, actual.AgentID)
}

func TestReadAgentConfigFallback(t *testing.T) {
	dir := t.TempDir()
	crowPath := dir + "/agent.crow.conf"
	woodpeckerPath := dir + "/agent.woodpecker.conf"

	// Neither file exists
	conf := readAgentConfig(crowPath)
	assert.EqualValues(t, -1, conf.AgentID)

	// Only woodpecker exists
	err := os.WriteFile(woodpeckerPath, []byte(`{"agent_id":42}`), 0o644)
	assert.NoError(t, err)
	conf = readAgentConfig(woodpeckerPath)
	assert.EqualValues(t, 42, conf.AgentID)

	// Only crow exists
	_ = os.Remove(woodpeckerPath)
	err = os.WriteFile(crowPath, []byte(`{"agent_id":99}`), 0o644)
	assert.NoError(t, err)
	conf = readAgentConfig(crowPath)
	assert.EqualValues(t, 99, conf.AgentID)
}

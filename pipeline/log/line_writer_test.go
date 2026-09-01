// Copyright 2019 Woodpecker Authors
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

package log_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v6/pipeline/log"
	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	rpc_mocks "codefloe.com/crowci/crow/v6/pipeline/rpc/mocks"
)

func TestLineWriter(t *testing.T) {
	peer := new(rpc_mocks.MockPeer)
	peer.On("EnqueueLog", mock.Anything)

	secrets := []string{"world"}
	lw := log.NewLineWriter(peer, "e9ea76a5-44a1-4059-9c4a-6956c478b26d", secrets...)

	_, err := lw.Write([]byte("hello world\n"))
	assert.NoError(t, err)
	_, err = lw.Write([]byte("the previous line had no newline at the end"))
	assert.NoError(t, err)

	peer.AssertCalled(t, "EnqueueLog", &rpc.LogEntry{
		StepUUID: "e9ea76a5-44a1-4059-9c4a-6956c478b26d",
		Time:     0,
		Type:     rpc.LogEntryStdout,
		Line:     0,
		Data:     []byte("hello ********"),
	})

	peer.AssertCalled(t, "EnqueueLog", &rpc.LogEntry{
		StepUUID: "e9ea76a5-44a1-4059-9c4a-6956c478b26d",
		Time:     0,
		Type:     rpc.LogEntryStdout,
		Line:     1,
		Data:     []byte("the previous line had no newline at the end"),
	})

	peer.AssertExpectations(t)
}

func TestLineWriterWritesPreparationProgress(t *testing.T) {
	peer := rpc_mocks.NewMockPeer(t)
	peer.On("EnqueueLog", &rpc.LogEntry{
		Data:     []byte("Pulling image example/clone:latest"),
		StepUUID: "step-uuid",
		Type:     rpc.LogEntryProgress,
		Line:     0,
		Time:     int64(0),
	}).Once()

	writer := log.NewLineWriterWithType(peer, "step-uuid", rpc.LogEntryStdout)
	_, err := writer.WriteProgress([]byte("Pulling image example/clone:latest"))

	assert.NoError(t, err)
}

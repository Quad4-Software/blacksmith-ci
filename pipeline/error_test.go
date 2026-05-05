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

package pipeline

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExitError(t *testing.T) {
	err := ExitError{
		UUID: "14534321",
		Code: 255,
	}
	assert.Equal(t, "uuid=14534321: exit code 255", err.Error())
}

func TestOomError(t *testing.T) {
	err := OomError{
		UUID: "14534321",
		Code: 137,
	}
	msg := err.Error()
	assert.Contains(t, msg, "uuid=14534321")
	assert.Contains(t, msg, "OOM-killed")
	assert.Contains(t, msg, "exit code 137")
	assert.Contains(t, msg, "mem_limit")
	assert.Contains(t, msg, "troubleshooting")
}

func TestDaemonUnreachableError(t *testing.T) {
	cause := errors.New("error during connect: Get \"http://%2Fvar%2Frun%2Fdocker.sock/v1.51/containers/foo/json\": received signal: 0x29c016660d90")
	err := &DaemonUnreachableError{
		Operation: "inspect",
		Container: "foo",
		Cause:     cause,
	}
	msg := err.Error()
	assert.Contains(t, msg, "lost connection to the container backend")
	assert.Contains(t, msg, "inspect")
	assert.Contains(t, msg, "foo")
	assert.Contains(t, msg, "troubleshooting")
	// We must NOT speculate about causes (e.g. agent OOM) in the error
	// message — many distinct failures produce the same transport-level
	// error and the agent cannot tell them apart. Causes belong in the
	// troubleshooting docs, not the error string.
	assert.NotContains(t, msg, "OOM")
	// Unwrap should return the original cause for errors.Is / errors.As.
	assert.ErrorIs(t, err, cause)
}

func TestDaemonUnreachableErrorEmptyContainer(t *testing.T) {
	err := &DaemonUnreachableError{
		Operation: "create volume",
		Cause:     errors.New("connection refused"),
	}
	assert.Contains(t, err.Error(), "create volume on container")
}

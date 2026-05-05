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

package docker

import (
	"errors"
	"io"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/pipeline"
)

func TestIsDaemonUnreachable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{
			"reported bug: received signal pointer",
			errors.New(`error during connect: Get "http://%2Fvar%2Frun%2Fdocker.sock/v1.51/containers/foo/json": received signal: 0x29c016660d90`),
			true,
		},
		{
			"docker daemon not running",
			errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?"),
			true,
		},
		{
			"connection refused on docker.sock",
			errors.New("Get http://%2Fvar%2Frun%2Fdocker.sock/v1.41/info: dial unix /var/run/docker.sock: connect: connection refused"),
			true,
		},
		{
			"broken pipe on docker.sock",
			errors.New("Post http://%2Fvar%2Frun%2Fdocker.sock/v1.41/containers/create: write unix @->/var/run/docker.sock: write: broken pipe"),
			true,
		},
		{"io.EOF wrapped", errors.Join(errors.New("read body"), io.EOF), true},
		{"ECONNREFUSED wrapped", errors.Join(errors.New("dial"), syscall.ECONNREFUSED), true},
		{"EPIPE wrapped", errors.Join(errors.New("write"), syscall.EPIPE), true},

		// Negative cases — these are normal API responses, not transport failures.
		{"no such container", errors.New("Error: No such container: foo"), false},
		{"already exists", errors.New(`Error response from daemon: Conflict. The container name "/foo" is already in use`), false},
		{"image not found", errors.New("Error: No such image: golang:1.99"), false},
		{"unrelated error", errors.New("something went wrong"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isDaemonUnreachable(tc.err))
		})
	}
}

func TestWrapDaemonError(t *testing.T) {
	// Daemon-unreachable errors get wrapped.
	cause := errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?")
	wrapped := wrapDaemonError("inspect", "foo", cause)
	var typed *pipeline.DaemonUnreachableError
	if !assert.ErrorAs(t, wrapped, &typed) {
		return
	}
	assert.Equal(t, "inspect", typed.Operation)
	assert.Equal(t, "foo", typed.Container)
	assert.ErrorIs(t, wrapped, cause)

	// Normal API errors pass through unchanged so existing detection
	// (e.g. isErrContainerNotFoundOrNotRunning) keeps working.
	api := errors.New("Error: No such container: foo")
	assert.Same(t, api, wrapDaemonError("inspect", "foo", api))

	// nil stays nil.
	assert.NoError(t, wrapDaemonError("inspect", "foo", nil))
}

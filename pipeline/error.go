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
	"fmt"
	"strings"
)

var (
	// ErrSkip is used as a return value when container execution should be
	// skipped at runtime. It is not returned as an error by any function.
	ErrSkip = errors.New("Skipped")

	// ErrCancel is used as a return value when the container execution receives
	// a cancellation signal from the context.
	ErrCancel = errors.New("Canceled")
)

// An ExitError reports an unsuccessful exit.
type ExitError struct {
	UUID string
	Code int
}

// Error returns the error message in string format.
func (e *ExitError) Error() string {
	return fmt.Sprintf("uuid=%s: exit code %d", e.UUID, e.Code)
}

// An OomError reports the process received an OOMKill from the kernel.
type OomError struct {
	UUID string
	Code int
}

// Error returns the error message in string format.
func (e *OomError) Error() string {
	return fmt.Sprintf("uuid=%s: step container was OOM-killed (exit code %d). "+
		"The step exceeded its memory limit. Increase the step's memory limit via "+
		"backend_options.docker.mem_limit (or remove it). "+
		"See https://docs.crowci.com/dev/troubleshooting/#step-container-oom-killed",
		e.UUID, e.Code)
}

// A DaemonUnreachableError reports that a call from the agent to the container
// backend (Docker / containerd) failed at the transport layer rather than
// returning a normal API response. This identifies the *class* of failure (the
// connection between the agent and the daemon was severed mid-call) but
// intentionally does not speculate about why — many distinct causes produce
// the same transport-level error, and the agent cannot tell them apart from
// the SDK error alone. See the troubleshooting docs for diagnosis.
//
// Operation is the backend call that failed (e.g. "inspect", "wait", "logs"),
// Container is the container the call was targeting (may be empty), and Cause
// is the original error from the SDK.
type DaemonUnreachableError struct {
	Operation string
	Container string
	Cause     error
}

// Error returns the error message in string format.
func (e *DaemonUnreachableError) Error() string {
	target := e.Container
	if target == "" {
		target = "container"
	}
	return fmt.Sprintf(
		"lost connection to the container backend while running %s on %s: %v. "+
			"See https://docs.crowci.com/dev/troubleshooting/#agent-lost-backend-connection "+
			"for diagnostic steps.",
		e.Operation, target, e.Cause,
	)
}

// Unwrap returns the underlying cause so callers can use errors.Is / errors.As.
func (e *DaemonUnreachableError) Unwrap() error {
	return e.Cause
}

// A StuckStepGraphError reports that the step scheduler ran out of runnable
// steps while some were still unstarted. The scheduler blocks on step results,
// so without this the workflow would hang until its timeout instead of
// reporting a problem. Reaching it means the dependency graph handed to the
// runtime was not schedulable, which is a bug rather than a pipeline error.
type StuckStepGraphError struct {
	Steps []string
}

// Error returns the error message in string format.
func (e *StuckStepGraphError) Error() string {
	return fmt.Sprintf(
		"workflow step dependencies are not schedulable, the following steps never became runnable: %s",
		strings.Join(e.Steps, ", "),
	)
}

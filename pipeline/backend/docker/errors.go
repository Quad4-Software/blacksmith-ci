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
	"net"
	"strings"
	"syscall"

	"codefloe.com/crowci/crow/v5/pipeline"
)

// isDaemonUnreachable reports whether err indicates that the Docker daemon
// connection broke (rather than the daemon returning a normal API error like
// "no such container"). This is intentionally conservative: false positives
// would hide real bugs, so we only match on errors that clearly point at the
// transport layer (socket / TCP / pipe) rather than the daemon's response.
func isDaemonUnreachable(err error) bool {
	if err == nil {
		return false
	}

	// Typed network / syscall errors. These are the clearest signal.
	if errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}

	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return true
	}

	// String-based fallbacks for errors the Docker SDK wraps without exposing
	// the underlying type. Match phrases that can only come from a broken
	// connection to the daemon, not from the daemon's API responses.
	msg := err.Error()
	switch {
	case strings.Contains(msg, "Cannot connect to the Docker daemon"):
		return true
	case strings.Contains(msg, "error during connect:"):
		// The Docker SDK prefixes transport-level failures with
		// "error during connect: <method> <url>: <cause>" — including the
		// "received signal: 0x..." shape we have observed when the calling
		// goroutine is torn down mid-request.
		return true
	case strings.Contains(msg, "received signal: 0x"):
		return true
	case strings.Contains(msg, "docker.sock") && strings.Contains(msg, "connection refused"):
		return true
	case strings.Contains(msg, "docker.sock") && strings.Contains(msg, "broken pipe"):
		return true
	}

	return false
}

// wrapDaemonError converts a raw Docker SDK error into a
// pipeline.DaemonUnreachableError when it indicates a lost connection to the
// daemon, so users see a self-explanatory message in the workflow status
// instead of a cryptic transport-level dump. Other errors are returned
// unchanged.
//
// The operation argument should be a short verb describing the failed call
// (e.g. "inspect", "wait", "tail logs"). The container argument is the target
// container name (may be empty if not applicable).
func wrapDaemonError(operation, container string, err error) error {
	if err == nil || !isDaemonUnreachable(err) {
		return err
	}
	return &pipeline.DaemonUnreachableError{
		Operation: operation,
		Container: container,
		Cause:     err,
	}
}

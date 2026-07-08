// Copyright 2022 Woodpecker Authors
// Copyright 2011 Drone.IO Inc.
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

package log

import (
	"io"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/pipeline/rpc"
	"codefloe.com/crowci/crow/v6/pipeline/shared"
)

// LineWriter sends logs to the client.
type LineWriter struct {
	sync.Mutex

	peer      rpc.Peer
	stepUUID  string
	num       int
	startTime time.Time
	replacer  *strings.Replacer
	logType   int // rpc.LogEntryStdout or rpc.LogEntryStderr
}

// NewLineWriter returns a new line writer for stdout.
func NewLineWriter(peer rpc.Peer, stepUUID string, secret ...string) io.Writer {
	lw := &LineWriter{
		peer:      peer,
		stepUUID:  stepUUID,
		num:       0,
		startTime: time.Now().UTC(),
		replacer:  shared.NewSecretsReplacer(secret),
		logType:   rpc.LogEntryStdout,
	}
	return lw
}

// NewLineWriterWithType returns a new line writer with a specific log type.
func NewLineWriterWithType(peer rpc.Peer, stepUUID string, logType int, secret ...string) *LineWriter {
	lw := &LineWriter{
		peer:      peer,
		stepUUID:  stepUUID,
		num:       0,
		startTime: time.Now().UTC(),
		replacer:  shared.NewSecretsReplacer(secret),
		logType:   logType,
	}
	return lw
}

func (w *LineWriter) Write(p []byte) (n int, err error) {
	w.Lock()
	defer w.Unlock()

	// Check for stream tag byte (added by Docker backend's taggedWriter)
	// 0x01 = stdout, 0x02 = stderr
	logType := w.logType
	data := p
	if len(p) > 0 && (p[0] == '\x01' || p[0] == '\x02') {
		switch p[0] {
		case '\x01':
			logType = rpc.LogEntryStdout
		case '\x02':
			logType = rpc.LogEntryStderr
		}
		data = p[1:] // Strip the tag byte
	}

	dataStr := string(data)
	if w.replacer != nil {
		dataStr = w.replacer.Replace(dataStr)
	}
	log.Trace().Str("step-uuid", w.stepUUID).Msgf("grpc write line: %s", dataStr)

	line := &rpc.LogEntry{
		Data:     []byte(strings.TrimSuffix(dataStr, "\n")), // remove trailing newline
		StepUUID: w.stepUUID,
		Time:     int64(time.Since(w.startTime).Seconds()),
		Type:     logType,
		Line:     w.num,
	}

	w.num++

	w.peer.EnqueueLog(line)
	return len(p), nil // Return original length including tag
}

// GetLineNumber returns the current line number (for synchronization).
func (w *LineWriter) GetLineNumber() int {
	w.Lock()
	defer w.Unlock()
	return w.num
}

// SetLineNumber sets the line number (for synchronization between stdout and stderr).
func (w *LineWriter) SetLineNumber(num int) {
	w.Lock()
	defer w.Unlock()
	w.num = num
}

// MultiplexedWriter wraps stdout and stderr writers and keeps their line numbers synchronized.
type MultiplexedWriter struct {
	stdout *LineWriter
	stderr *LineWriter
	mu     sync.Mutex
}

// NewMultiplexedWriter creates stdout and stderr writers that share line numbering.
func NewMultiplexedWriter(peer rpc.Peer, stepUUID string, secret ...string) *MultiplexedWriter {
	return &MultiplexedWriter{
		stdout: NewLineWriterWithType(peer, stepUUID, rpc.LogEntryStdout, secret...),
		stderr: NewLineWriterWithType(peer, stepUUID, rpc.LogEntryStderr, secret...),
	}
}

// Stdout returns the stdout writer.
func (m *MultiplexedWriter) Stdout() io.Writer {
	return &syncWriter{mw: m, w: m.stdout}
}

// Stderr returns the stderr writer.
func (m *MultiplexedWriter) Stderr() io.Writer {
	return &syncWriter{mw: m, w: m.stderr}
}

// syncWriter synchronizes line numbers after each write.
type syncWriter struct {
	mw *MultiplexedWriter
	w  *LineWriter
}

func (s *syncWriter) Write(p []byte) (n int, err error) {
	s.mw.mu.Lock()
	defer s.mw.mu.Unlock()

	// Sync line number before write
	maxLine := s.mw.stdout.GetLineNumber()
	if s.mw.stderr.GetLineNumber() > maxLine {
		maxLine = s.mw.stderr.GetLineNumber()
	}
	s.w.SetLineNumber(maxLine)

	// Perform write
	n, err = s.w.Write(p)

	// Sync line numbers after write
	newLine := s.w.GetLineNumber()
	s.mw.stdout.SetLineNumber(newLine)
	s.mw.stderr.SetLineNumber(newLine)

	return n, err
}

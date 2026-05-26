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

package clone

import (
	"io"
	"sync"
)

// StreamBuffer is a thread-safe append-only buffer. Multiple readers can
// observe the same content independently; each reader sees all writes from
// the start. Reads block until new data is written or Close is called.
//
// Used by the docker backend's native clone path: the clone process writes
// stdout/stderr into a StreamBuffer; TailStep returns a Reader so the agent
// can stream logs to the server while the clone is still running.
type StreamBuffer struct {
	mu     sync.Mutex
	cond   *sync.Cond
	data   []byte
	closed bool
}

// NewStreamBuffer returns a ready-to-use buffer.
func NewStreamBuffer() *StreamBuffer {
	b := &StreamBuffer{}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Write appends p to the buffer and wakes any blocked readers.
// Safe for concurrent use.
func (b *StreamBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return 0, io.ErrClosedPipe
	}
	b.data = append(b.data, p...)
	b.cond.Broadcast()
	return len(p), nil
}

// Close marks the buffer finished. Subsequent reads will drain any
// remaining content and then return io.EOF.
func (b *StreamBuffer) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.cond.Broadcast()
	return nil
}

// Reader returns an independent io.ReadCloser that reads from offset 0.
// Calling Reader multiple times yields independent readers.
func (b *StreamBuffer) Reader() io.ReadCloser {
	return &streamReader{buf: b}
}

type streamReader struct {
	buf *StreamBuffer
	pos int
}

func (r *streamReader) Read(p []byte) (int, error) {
	r.buf.mu.Lock()
	defer r.buf.mu.Unlock()
	for r.pos >= len(r.buf.data) && !r.buf.closed {
		r.buf.cond.Wait()
	}
	if r.pos >= len(r.buf.data) {
		return 0, io.EOF
	}
	n := copy(p, r.buf.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *streamReader) Close() error { return nil }

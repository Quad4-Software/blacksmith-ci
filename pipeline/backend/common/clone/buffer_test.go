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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamBuffer_ReaderBlocksUntilWrite(t *testing.T) {
	b := NewStreamBuffer()
	r := b.Reader()
	done := make(chan []byte, 1)

	go func() {
		buf := make([]byte, 5)
		n, _ := r.Read(buf)
		done <- buf[:n]
	}()

	select {
	case <-done:
		t.Fatal("reader should block until something is written")
	case <-time.After(50 * time.Millisecond):
	}

	_, _ = b.Write([]byte("hello"))

	select {
	case got := <-done:
		assert.Equal(t, []byte("hello"), got)
	case <-time.After(time.Second):
		t.Fatal("reader did not unblock after write")
	}
}

func TestStreamBuffer_ReaderSeesAllWrites(t *testing.T) {
	b := NewStreamBuffer()
	_, _ = b.Write([]byte("first "))
	_, _ = b.Write([]byte("second"))
	b.Close()

	r := b.Reader()
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, []byte("first second"), got)
}

func TestStreamBuffer_MultipleReadersIndependent(t *testing.T) {
	b := NewStreamBuffer()
	_, _ = b.Write([]byte("payload"))
	b.Close()

	r1, r2 := b.Reader(), b.Reader()

	g1, err := io.ReadAll(r1)
	require.NoError(t, err)
	g2, err := io.ReadAll(r2)
	require.NoError(t, err)

	assert.Equal(t, []byte("payload"), g1, "r1 must see the full buffer")
	assert.Equal(t, []byte("payload"), g2, "r2 must also see the full buffer independently")
}

func TestStreamBuffer_CloseSignalsEOF(t *testing.T) {
	b := NewStreamBuffer()
	r := b.Reader()
	done := make(chan error, 1)

	go func() {
		_, err := io.ReadAll(r)
		done <- err
	}()

	b.Close()

	select {
	case err := <-done:
		assert.NoError(t, err, "ReadAll returns nil on clean EOF")
	case <-time.After(time.Second):
		t.Fatal("Close did not propagate EOF to reader")
	}
}

func TestStreamBuffer_ConcurrentWrites(t *testing.T) {
	b := NewStreamBuffer()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = b.Write([]byte("x"))
		}()
	}
	wg.Wait()
	b.Close()

	r := b.Reader()
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Len(t, got, 10, "all concurrent writes must be captured")
}

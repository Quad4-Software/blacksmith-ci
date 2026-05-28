// Copyright 2021 Woodpecker Authors
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

package proclog

import "sync"

// Ring is a fixed-capacity FIFO of SinkLine. On push when full, oldest is dropped.
// Safe for concurrent use.
type Ring struct {
	mu   sync.Mutex
	buf  []SinkLine
	head int // index of the next read
	size int
	cap  int
}

func NewRing(capacity int) *Ring {
	if capacity <= 0 {
		capacity = defaultCapacity
	}
	return &Ring{buf: make([]SinkLine, capacity), cap: capacity}
}

// Push adds an item, dropping the oldest if at capacity.
func (r *Ring) Push(item SinkLine) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size < r.cap {
		idx := (r.head + r.size) % r.cap
		r.buf[idx] = item
		r.size++
		return
	}
	// Full: overwrite oldest (head), advance head.
	r.buf[r.head] = item
	r.head = (r.head + 1) % r.cap
}

// Len returns the current number of stored items.
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.size
}

// DrainAll returns all items in FIFO order and empties the ring.
func (r *Ring) DrainAll() []SinkLine {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.drainLocked(r.size)
}

// DrainN returns up to n items in FIFO order, removing them from the ring.
func (r *Ring) DrainN(n int) []SinkLine {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n > r.size {
		n = r.size
	}
	return r.drainLocked(n)
}

func (r *Ring) drainLocked(n int) []SinkLine {
	out := make([]SinkLine, n)
	for i := range n {
		out[i] = r.buf[(r.head+i)%r.cap]
	}
	r.head = (r.head + n) % r.cap
	r.size -= n
	return out
}

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

import (
	"bytes"
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

const defaultCapacity = 500

// SinkLine is one parsed zerolog line ready for shipping.
type SinkLine struct {
	Time  int64  // unix nanos
	Level string // "debug"|"info"|"warn"|"error"|"trace"|"fatal"
	Line  string // raw original JSON line (preserves structured fields)
}

// Sink is a zerolog io.Writer that captures each JSON line into a bounded channel.
// On overflow, the oldest queued line is dropped to make room (newest wins).
type Sink struct {
	out       chan SinkLine
	dropCount atomic.Int64
}

// NewSink constructs a sink with a channel capacity of `capacity`.
func NewSink(capacity int) *Sink {
	if capacity <= 0 {
		capacity = defaultCapacity
	}
	return &Sink{out: make(chan SinkLine, capacity)}
}

// Out returns the read end of the channel.
func (s *Sink) Out() <-chan SinkLine { return s.out }

// DropCount returns and zeroes the overflow drop counter.
func (s *Sink) DropCount() int64 { return s.dropCount.Swap(0) }

// Write implements io.Writer. Called by zerolog with one full JSON line per Write.
func (s *Sink) Write(p []byte) (int, error) {
	n := len(p)
	trimmed := bytes.TrimSpace(p)
	if len(trimmed) == 0 {
		return n, nil
	}

	line := SinkLine{Line: string(trimmed)}

	// Best-effort parse for level/time.
	var head struct {
		Level   string `json:"level"`
		TimeStr string `json:"time"`
	}
	if err := json.Unmarshal(trimmed, &head); err == nil {
		line.Level = head.Level
		if head.TimeStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, head.TimeStr); err == nil {
				line.Time = t.UnixNano()
			}
		}
	}
	if line.Time == 0 {
		line.Time = time.Now().UnixNano()
	}
	if line.Level == "" {
		line.Level = zerolog.LevelInfoValue
	}

	select {
	case s.out <- line:
	default:
		// Channel full: drop the oldest queued item to make room for the newest.
		select {
		case <-s.out:
			s.dropCount.Add(1)
		default:
		}
		select {
		case s.out <- line:
		default:
			s.dropCount.Add(1)
		}
	}
	return n, nil
}

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

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
)

var (
	// A running archive or transfer reports how far it got this often, so a
	// large cache does not look like a hung step.
	progressInterval = 10 * time.Second
	// A transfer attempt that moved no data for this long is aborted, so a
	// hung connection is retried instead of holding the step until the step
	// timeout.
	stallTimeout = time.Minute
)

// errStalled ends a transfer attempt that stopped moving data.
var errStalled = errors.New("transfer stalled")

// counter counts the bytes and files an operation has processed so far.
type counter struct {
	bytes atomic.Int64
	files atomic.Int64
}

// reader counts what is read through r.
func (c *counter) reader(r io.Reader) io.Reader { return countingReader{r, c} }

type countingReader struct {
	r io.Reader
	c *counter
}

func (r countingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.c.bytes.Add(int64(n))
	return n, err
}

// watch prints line every progressInterval until stop is called, unless line
// is nil. When stall
// is set, the returned context is canceled with errStalled once c stops
// advancing for stallTimeout; stop then reports errStalled for any error the
// cancellation caused.
func watch(ctx context.Context, out io.Writer, c *counter, stall bool, line func(done int64, elapsed time.Duration) string) (context.Context, func(error) error) {
	ctx, cancel := context.WithCancelCause(ctx)
	done := make(chan struct{})
	finished := make(chan struct{})
	start := time.Now()

	go func() {
		defer close(finished)
		ticker := time.NewTicker(min(progressInterval, stallTimeout))
		defer ticker.Stop()
		lastReport, lastChange, last := start, start, c.bytes.Load()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				current := c.bytes.Load()
				if current != last {
					lastChange, last = now, current
				}
				if stall && now.Sub(lastChange) >= stallTimeout {
					cancel(errStalled)
					return
				}
				if line != nil && now.Sub(lastReport) >= progressInterval {
					fmt.Fprintln(out, line(current, now.Sub(start)))
					lastReport = now
				}
			}
		}
	}()

	return ctx, func(err error) error {
		close(done)
		<-finished
		stalled := errors.Is(context.Cause(ctx), errStalled)
		cancel(nil)
		if err != nil && stalled {
			return fmt.Errorf("%w: no data moved for %s", errStalled, stallTimeout)
		}
		return err
	}
}

// transferLine formats the progress of a transfer of total bytes.
func transferLine(what string, total int64) func(int64, time.Duration) string {
	return func(done int64, elapsed time.Duration) string {
		line := fmt.Sprintf("%s: %s", what, humanize.IBytes(uint64(max(done, 0))))
		if total > 0 {
			line += fmt.Sprintf(" of %s (%d%%)", humanize.IBytes(uint64(total)), done*100/total)
		}
		return line + ", " + rate(done, elapsed)
	}
}

// rate formats bytes moved in elapsed as a per-second rate.
func rate(bytes int64, elapsed time.Duration) string {
	if elapsed <= 0 {
		return "-/s"
	}
	return humanize.IBytes(uint64(float64(max(bytes, 0))/elapsed.Seconds())) + "/s"
}

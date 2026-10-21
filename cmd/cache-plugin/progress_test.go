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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fastProgress shortens the progress and stall intervals for one test.
func fastProgress(t *testing.T) {
	t.Helper()
	interval, stall, delays := progressInterval, stallTimeout, retryDelays
	progressInterval, stallTimeout, retryDelays = 5*time.Millisecond, 50*time.Millisecond, []time.Duration{time.Millisecond}
	t.Cleanup(func() { progressInterval, stallTimeout, retryDelays = interval, stall, delays })
}

func TestWatchReportsProgress(t *testing.T) {
	fastProgress(t)
	var out strings.Builder
	c := new(counter)
	_, stop := watch(context.Background(), &out, c, true, transferLine("uploaded", 100))
	for range 10 {
		c.bytes.Add(5)
		time.Sleep(10 * time.Millisecond)
	}
	require.NoError(t, stop(nil))
	assert.Contains(t, out.String(), "uploaded: ")
	assert.Contains(t, out.String(), " of 100 B (")
}

func TestWatchCancelsAStalledTransfer(t *testing.T) {
	fastProgress(t)
	ctx, stop := watch(context.Background(), new(strings.Builder), new(counter), true, transferLine("downloaded", 0))
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("a transfer that moves no data is not canceled")
	}
	err := stop(context.Canceled)
	require.ErrorIs(t, err, errStalled)
	assert.True(t, retryable(err, false), "a stalled transfer is retried")
}

func TestWatchWithoutStallNeverCancels(t *testing.T) {
	fastProgress(t)
	ctx, stop := watch(context.Background(), new(strings.Builder), new(counter), false, transferLine("archived", 0))
	time.Sleep(4 * stallTimeout)
	require.NoError(t, ctx.Err())
	require.NoError(t, stop(nil))
}

func TestStalledUploadIsRetried(t *testing.T) {
	fastProgress(t)
	attempts := make(chan struct{}, 10)
	release := make(chan struct{})
	// The store takes the body but never answers.
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		attempts <- struct{}{}
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	archive, err := os.CreateTemp(t.TempDir(), "archive")
	require.NoError(t, err)
	t.Cleanup(func() { _ = archive.Close() })
	_, err = archive.WriteString("archive")
	require.NoError(t, err)

	var out strings.Builder
	c := &client{http: http.DefaultClient, out: &out}
	err = c.uploadWhole(context.Background(), srv.URL+"/blob", archive, int64(len("archive")))
	require.ErrorIs(t, err, errStalled)
	assert.Len(t, attempts, 2, "the stalled attempt is retried once per retry delay")
	assert.Contains(t, out.String(), "upload failed, retrying")
	assert.Contains(t, out.String(), "transfer stalled")
}

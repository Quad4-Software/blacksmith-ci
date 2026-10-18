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
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryable(t *testing.T) {
	t.Parallel()

	reset := &url.Error{Op: "Get", URL: "https://s3", Err: errors.New("connection reset by peer")}
	tests := []struct {
		name                string
		err                 error
		notFoundIsTransient bool
		want                bool
	}{
		{"failed request", reset, false, true},
		{"broken-off body", fmt.Errorf("extract: %w", &readError{errors.New("connection reset by peer")}), false, true},
		{"server error", &statusError{code: http.StatusInternalServerError}, false, true},
		{"throttled", &statusError{code: http.StatusTooManyRequests}, false, true},
		{"storage not caught up", &statusError{code: http.StatusNotFound}, true, true},
		{"api not found", &statusError{code: http.StatusNotFound}, false, false},
		{"forbidden", &statusError{code: http.StatusForbidden}, true, false},
		{"cache miss", errMiss, false, false},
		{"refused archive entry", errors.New("refusing to extract through symlink"), true, false},
		{"canceled", &url.Error{Op: "Get", URL: "https://s3", Err: context.Canceled}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, retryable(tt.err, tt.notFoundIsTransient))
		})
	}
}

// flakyTransport fails the first request of each kind the way an object store
// does now and then: a download breaks off halfway, an upload gets a 503 and a
// commit a 500.
type flakyTransport struct {
	mu     sync.Mutex
	failed map[string]bool
}

func (f *flakyTransport) once(kind string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failed[kind] {
		return false
	}
	f.failed[kind] = true
	return true
}

func (f *flakyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/blobs/") && f.once("download"):
		resp, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		resp.Body = &breakingBody{ReadCloser: resp.Body, left: resp.ContentLength / 2}
		return resp, nil
	case req.Method == http.MethodPut && f.once("upload"):
		if req.Body != nil {
			_, _ = io.Copy(io.Discard, req.Body)
		}
		return failure(req, http.StatusServiceUnavailable), nil
	case strings.HasSuffix(req.URL.Path, "/commit") && f.once("commit"):
		return failure(req, http.StatusInternalServerError), nil
	}
	return http.DefaultTransport.RoundTrip(req)
}

func failure(req *http.Request, code int) *http.Response {
	return &http.Response{
		StatusCode: code,
		Status:     http.StatusText(code),
		Body:       io.NopCloser(strings.NewReader("try again")),
		Request:    req,
	}
}

// breakingBody returns left bytes and then fails like a reset connection.
type breakingBody struct {
	io.ReadCloser
	left int64
}

func (b *breakingBody) Read(p []byte) (int, error) {
	if b.left <= 0 {
		return 0, errors.New("read tcp: connection reset by peer")
	}
	if int64(len(p)) > b.left {
		p = p[:b.left]
	}
	n, err := b.ReadCloser.Read(p)
	b.left -= int64(n)
	return n, err
}

func TestTransientFailuresAreRetried(t *testing.T) {
	h := newHarness(t)
	previousDelays, previousTransport := retryDelays, http.DefaultClient.Transport
	retryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	http.DefaultClient.Transport = &flakyTransport{failed: map[string]bool{}}
	t.Cleanup(func() { retryDelays, http.DefaultClient.Transport = previousDelays, previousTransport })

	spec := cacheSpec{Key: "big", Paths: []string{"cache"}}
	first := t.TempDir()
	random := make([]byte, 1<<20)
	f, err := os.Open("/dev/urandom")
	require.NoError(t, err)
	_, err = io.ReadFull(f, random)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	writeFile(t, first, "cache/blob", string(random))

	out := h.run(t, "save", first, spec)
	assert.Contains(t, out, "upload failed, retrying")
	assert.Contains(t, out, "/commit failed, retrying")
	assert.Contains(t, out, "saved big")

	second := t.TempDir()
	out = h.run(t, "restore", second, spec)
	assert.Contains(t, out, "restore of big failed, retrying")
	assert.Contains(t, out, "connection reset by peer")
	assert.Contains(t, out, "restored big (exact match")
	assert.Equal(t, string(random), readFile(t, second, "cache/blob"))
}

func TestTransferErrorsHideTheSignature(t *testing.T) {
	previous := retryDelays
	retryDelays = []time.Duration{time.Millisecond}
	t.Cleanup(func() { retryDelays = previous })

	// Nothing listens on port 1, so every attempt fails in transport.
	signed := "http://127.0.0.1:1/bucket/cache/1.tar.zst?X-Amz-Credential=key&X-Amz-Signature=secret"
	var out strings.Builder
	c := &client{http: http.DefaultClient, out: &out}

	_, err := c.download(context.Background(), signed)
	require.Error(t, err)
	archive, err := os.CreateTemp(t.TempDir(), "archive")
	require.NoError(t, err)
	t.Cleanup(func() { _ = archive.Close() })
	err = c.upload(context.Background(), signed, archive, 0)
	require.Error(t, err)

	logged := out.String() + err.Error()
	assert.Contains(t, logged, "http://127.0.0.1:1/bucket/cache/1.tar.zst")
	assert.NotContains(t, logged, "X-Amz-Signature")
	assert.NotContains(t, logged, "secret")
}

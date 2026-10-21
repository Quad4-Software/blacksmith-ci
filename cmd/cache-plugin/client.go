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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	// Metadata calls time out after this; transfers are bounded by
	// stallTimeout and the step timeout.
	apiTimeout = 30 * time.Second
	// This many parts of an archive are uploaded at once. Object stores take
	// a few MB/s per connection, so the throughput grows with the number of
	// parallel parts.
	uploadConcurrency = 32
)

var (
	errMiss     = errors.New("no cache found")
	errExists   = errors.New("already cached")
	errReadOnly = errors.New("this pipeline may only restore caches (pull request from a fork)")
	errQuota    = errors.New("cache quota exceeded")
)

// client talks to the server's build cache API with the workflow's cache token.
type client struct {
	baseURL string
	token   string
	http    *http.Client
	// out receives retry notices. Parts upload in parallel, so it must be
	// safe for concurrent use, see syncWriter.
	out io.Writer
}

// syncWriter serializes writes to w.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

type restoreResponse struct {
	ID    int64  `json:"id"`
	Key   string `json:"key"`
	Scope string `json:"scope"`
	Size  int64  `json:"size"`
	Exact bool   `json:"exact"`
	URL   string `json:"url"`
}

// reserveResponse is where to upload. Servers that can take the archive in
// parts set PartURLs, and part i holds the bytes from i*PartSize.
type reserveResponse struct {
	ID       int64    `json:"id"`
	URL      string   `json:"url"`
	UploadID string   `json:"upload_id"`
	PartSize int64    `json:"part_size"`
	PartURLs []string `json:"part_urls"`
}

// call posts body to an API endpoint and decodes the answer into out. Failed
// requests and server errors are retried; the API's own answers, such as a
// miss, are returned as the matching sentinel error.
func (c *client) call(ctx context.Context, endpoint string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return withRetry(ctx, c.out, endpoint, false, func() error {
		return c.callOnce(ctx, endpoint, payload, out)
	})
}

func (c *client) callOnce(ctx context.Context, endpoint string, payload []byte, out any) error {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		return json.NewDecoder(resp.Body).Decode(out)
	case http.StatusNotFound:
		return errMiss
	case http.StatusConflict:
		return errExists
	case http.StatusForbidden:
		return errReadOnly
	case http.StatusInsufficientStorage:
		return errQuota
	default:
		return responseError(resp)
	}
}

func responseError(resp *http.Response) error {
	var body struct {
		Error string `json:"error"`
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096)) //nolint:mnd
	msg := strings.TrimSpace(string(raw))
	if json.Unmarshal(raw, &body) == nil && body.Error != "" {
		msg = body.Error
	}
	return &statusError{code: resp.StatusCode, msg: fmt.Sprintf("server returned %s: %s", resp.Status, msg)}
}

func (c *client) restore(ctx context.Context, key string, restoreKeys []string) (*restoreResponse, error) {
	out := new(restoreResponse)
	err := c.call(ctx, "/restore", map[string]any{"key": key, "restore_keys": restoreKeys}, out)
	return out, err
}

func (c *client) reserve(ctx context.Context, key string, size int64) (*reserveResponse, error) {
	out := new(reserveResponse)
	err := c.call(ctx, "/reserve", map[string]any{"key": key, "size": size, "multipart": true}, out)
	return out, err
}

func (c *client) commit(ctx context.Context, id int64, uploadID string) error {
	var out map[string]any
	return c.call(ctx, "/commit", map[string]any{"id": id, "upload_id": uploadID}, &out)
}

// authorize adds the cache token to transfers to the server's own blob
// endpoint. Presigned storage URLs carry their own signature and are rejected
// when they get a second authorization.
func (c *client) authorize(req *http.Request) {
	if strings.HasPrefix(req.URL.String(), c.baseURL+"/blobs/") {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

// redactURL drops the query of the URL in a request error. Storage URLs are
// presigned, and their signature must not end up in the step log, where
// everyone who can read the pipeline could use it until it expires.
func redactURL(err error) error {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}
	if u, parseErr := url.Parse(urlErr.URL); parseErr == nil {
		u.RawQuery, u.Fragment, u.User = "", "", nil
		urlErr.URL = u.String()
	} else {
		urlErr.URL = "<storage URL>"
	}
	return err
}

// download opens the archive at url. Read errors of the returned body are
// tagged, so a connection that breaks off mid-transfer is retried.
func (c *client) download(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, redactURL(err)
	}
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, redactURL(err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, responseError(resp)
	}
	return bodyReader{resp.Body}, nil
}

// upload sends the archive file to where the reservation points, in parallel
// parts if the server offers them.
func (c *client) upload(ctx context.Context, res *reserveResponse, f *os.File, size int64) error {
	if len(res.PartURLs) > 0 {
		return c.uploadParts(ctx, res.PartURLs, res.PartSize, f, size)
	}
	return c.uploadWhole(ctx, res.URL, f, size)
}

// uploadParts PUTs part i of the archive file to urls[i], uploadConcurrency
// parts at a time. Each part is retried on its own, and an attempt that stops
// sending data for stallTimeout is aborted and retried.
func (c *client) uploadParts(ctx context.Context, urls []string, partSize int64, f *os.File, size int64) error {
	fmt.Fprintf(c.out, "uploading in %d parts, %d at a time\n", len(urls), min(len(urls), uploadConcurrency))
	sent := new(counter)
	_, stop := watch(ctx, c.out, sent, false, transferLine("uploaded", size))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(uploadConcurrency)
	for i, url := range urls {
		offset := int64(i) * partSize
		length := min(partSize, size-offset)
		group.Go(func() error {
			return withRetry(groupCtx, c.out, fmt.Sprintf("upload of part %d", i+1), false, func() error {
				part := new(counter)
				attemptCtx, stopPart := watch(groupCtx, c.out, part, true, nil)
				err := stopPart(c.uploadOnce(attemptCtx, url, sent.reader(part.reader(io.NewSectionReader(f, offset, length))), length))
				if err != nil {
					// The retry sends the part again.
					sent.bytes.Add(-part.bytes.Load())
				}
				return err
			})
		})
	}
	return stop(group.Wait())
}

// uploadWhole PUTs the archive file to url, retrying failed attempts from the
// start. An attempt that stops sending data for stallTimeout is aborted and
// retried.
func (c *client) uploadWhole(ctx context.Context, url string, f *os.File, size int64) error {
	return withRetry(ctx, c.out, "upload", false, func() error {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		sent := new(counter)
		attemptCtx, stop := watch(ctx, c.out, sent, true, transferLine("uploaded", size))
		return stop(c.uploadOnce(attemptCtx, url, sent.reader(f), size))
	})
}

func (c *client) uploadOnce(ctx context.Context, url string, r io.Reader, size int64) error {
	// r is no Closer, so the client cannot close the file between attempts.
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, r)
	if err != nil {
		return redactURL(err)
	}
	// Presigned PUTs need the length up front, so the archive is not chunked.
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/zstd")
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return redactURL(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return responseError(resp)
	}
	return nil
}

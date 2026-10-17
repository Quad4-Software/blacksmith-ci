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
	"os"
	"strings"
	"time"
)

// apiTimeout bounds the metadata calls; transfers are bounded by the step timeout.
const apiTimeout = 30 * time.Second

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
}

type restoreResponse struct {
	ID    int64  `json:"id"`
	Key   string `json:"key"`
	Scope string `json:"scope"`
	Size  int64  `json:"size"`
	Exact bool   `json:"exact"`
	URL   string `json:"url"`
}

type reserveResponse struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}

func (c *client) call(ctx context.Context, endpoint string, body, out any) error {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
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
	if json.Unmarshal(raw, &body) == nil && body.Error != "" {
		return fmt.Errorf("server returned %s: %s", resp.Status, body.Error)
	}
	return fmt.Errorf("server returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
}

func (c *client) restore(ctx context.Context, key string, restoreKeys []string) (*restoreResponse, error) {
	out := new(restoreResponse)
	err := c.call(ctx, "/restore", map[string]any{"key": key, "restore_keys": restoreKeys}, out)
	return out, err
}

func (c *client) reserve(ctx context.Context, key string, size int64) (*reserveResponse, error) {
	out := new(reserveResponse)
	err := c.call(ctx, "/reserve", map[string]any{"key": key, "size": size}, out)
	return out, err
}

func (c *client) commit(ctx context.Context, id int64) error {
	var out map[string]any
	return c.call(ctx, "/commit", map[string]any{"id": id}, &out)
}

// authorize adds the cache token to transfers to the server's own blob
// endpoint. Presigned storage URLs carry their own signature and are rejected
// when they get a second authorization.
func (c *client) authorize(req *http.Request) {
	if strings.HasPrefix(req.URL.String(), c.baseURL+"/blobs/") {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

// download opens the archive at url.
func (c *client) download(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, responseError(resp)
	}
	return resp.Body, nil
}

// upload PUTs the archive file to url.
func (c *client) upload(ctx context.Context, url string, f *os.File, size int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, f)
	if err != nil {
		return err
	}
	// Presigned PUTs need the length up front, so the archive is not chunked.
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/zstd")
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return responseError(resp)
	}
	return nil
}

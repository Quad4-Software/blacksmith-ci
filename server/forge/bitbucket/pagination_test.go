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

package bitbucket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/forge/bitbucket/fixtures"
	"codefloe.com/crowci/crow/v6/server/model"
)

// maxRequestsPerCollection caps how often the counting fixture below answers a
// request for the same collection. Anything past that is a runaway pagination
// loop, and answering with an error turns what would otherwise be a hung test
// run into an immediate, readable failure.
const maxRequestsPerCollection = 10

// countingHandler wraps the Bitbucket fixture and counts requests per path,
// failing the request once a single collection is asked for too many times.
type countingHandler struct {
	inner  http.Handler
	counts map[string]*atomic.Int64
}

func newCountingHandler(paths ...string) *countingHandler {
	h := &countingHandler{inner: fixtures.Handler(), counts: map[string]*atomic.Int64{}}
	for _, p := range paths {
		h.counts[p] = &atomic.Int64{}
	}
	return h
}

func (h *countingHandler) count(path string) int64 {
	return h.counts[path].Load()
}

func (h *countingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for suffix, counter := range h.counts {
		if !strings.HasSuffix(r.URL.Path, suffix) {
			continue
		}
		if counter.Add(1) > maxRequestsPerCollection {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"runaway pagination loop"}}`))
			return
		}
	}
	h.inner.ServeHTTP(w, r)
}

// TestDeactivateNonPaginatingHooksCollection guards against issue #1487.
//
// Bitbucket Cloud ignores the `page` parameter on the hooks collection and keeps
// returning the same page. A paginator that only stops once a page comes back
// short therefore never terminates.
func TestDeactivateNonPaginatingHooksCollection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := newCountingHandler("/hooks")
	s := httptest.NewServer(h)
	defer s.Close()

	c := &config{url: s.URL, API: s.URL}
	repo := &model.Repo{Owner: "test_name", Name: "repo_name"}

	err := c.Deactivate(t.Context(), fakeUser, repo, "http://127.0.0.1")
	require.NoError(t, err)

	// One list request (the response carries no `next` link), plus the delete.
	assert.LessOrEqual(t, h.count("/hooks"), int64(2),
		"Deactivate must not re-request a collection that ignores the page parameter")
}

// TestDeactivatePaginatedHooksCollection makes sure the `next`-driven paginator
// still walks every page of a properly paginating collection.
func TestDeactivatePaginatedHooksCollection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := newCountingHandler("/hooks")
	s := httptest.NewServer(h)
	defer s.Close()

	c := &config{url: s.URL, API: s.URL}
	repo := &model.Repo{Owner: "test_name", Name: "repo_hooks_paginated"}

	// The matching hook only lives on the second page, so Deactivate can just
	// find it if pagination followed the `next` link.
	err := c.Deactivate(t.Context(), fakeUser, repo, "http://127.0.0.1/second")
	require.NoError(t, err)

	assert.Equal(t, int64(2), h.count("/hooks"), "both hook pages must be fetched")
}

// TestReposStopsOnLastPage covers the remaining Bitbucket collections: a
// response without a `next` link is the last page, so each collection must be
// requested exactly once.
func TestReposStopsOnLastPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := newCountingHandler("/workspaces", "/permissions/repositories", "/2.0/repositories/test_name")
	s := httptest.NewServer(h)
	defer s.Close()

	c := &config{url: s.URL, API: s.URL}

	repos, err := c.Repos(t.Context(), fakeUser)
	require.NoError(t, err)
	assert.NotEmpty(t, repos)

	assert.Equal(t, int64(1), h.count("/workspaces"), "workspaces pagination must stop on the last page")
	assert.Equal(t, int64(1), h.count("/permissions/repositories"), "permissions pagination must stop on the last page")
	assert.Equal(t, int64(1), h.count("/2.0/repositories/test_name"), "repos pagination must stop on the last page")
}

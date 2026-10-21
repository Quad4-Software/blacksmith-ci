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
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/api"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/services/buildcache"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/datastore"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

// cacheStore keeps cache entries in a real SQLite database and serves one
// running workflow of a push to main.
type cacheStore struct {
	store.Store
	workflow *model.Workflow
}

func (s *cacheStore) WorkflowLoad(id int64) (*model.Workflow, error) {
	if id != s.workflow.ID {
		return nil, types.RecordNotExist
	}
	return s.workflow, nil
}

func (s *cacheStore) GetPipeline(int64) (*model.Pipeline, error) {
	return &model.Pipeline{ID: 2, RepoID: 3, Event: model.EventPush, Branch: "main", Ref: "refs/heads/main"}, nil
}

func (s *cacheStore) GetRepo(int64) (*model.Repo, error) {
	return &model.Repo{ID: 3, Branch: "main"}, nil
}

type harness struct {
	service  *buildcache.Service
	store    *cacheStore
	baseURL  string
	token    string
	workflow *model.Workflow
	// storageRoot holds the archives.
	storageRoot string
}

// newHarness serves the cache API from a local storage, which wrap may
// replace.
func newHarness(t *testing.T, wrap ...func(buildcache.Storage) buildcache.Storage) *harness {
	t.Helper()

	engine, err := datastore.NewEngine(&store.Opts{Driver: "sqlite3", Config: filepath.Join(t.TempDir(), "crow.sqlite")})
	require.NoError(t, err)
	require.NoError(t, engine.Migrate(context.Background(), true))
	t.Cleanup(func() { _ = engine.Close() })

	workflow := &model.Workflow{ID: 1, PipelineID: 2, State: model.StatusRunning}
	s := &cacheStore{Store: engine, workflow: workflow}

	storageRoot := t.TempDir()
	storage, err := buildcache.NewLocalStorage(storageRoot)
	require.NoError(t, err)
	for _, w := range wrap {
		storage = w(storage)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/v1/cache")
	group.POST("/restore", api.PostCacheRestore)
	group.POST("/reserve", api.PostCacheReserve)
	group.POST("/commit", api.PostCacheCommit)
	group.GET("/blobs/:id", api.GetCacheBlob)
	group.PUT("/blobs/:id", api.PutCacheBlob)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	service := buildcache.New(s, storage, buildcache.Config{
		BlobURL:    srv.URL + "/api/v1/cache/blobs",
		Secret:     "secret",
		PendingTTL: time.Hour,
		URLExpiry:  time.Hour,
	})
	previous := server.Config.Services.BuildCache
	server.Config.Services.BuildCache = service
	t.Cleanup(func() { server.Config.Services.BuildCache = previous })

	token, err := service.MintToken(workflow.ID)
	require.NoError(t, err)
	return &harness{service: service, store: s, baseURL: srv.URL + "/api/v1/cache", token: token, workflow: workflow, storageRoot: storageRoot}
}

// run runs the plugin in mode on workspace and returns its output.
func (h *harness) run(t *testing.T, mode, workspace string, specs ...cacheSpec) string {
	t.Helper()
	encoded, err := json.Marshal(specs)
	require.NoError(t, err)
	env := map[string]string{
		"PLUGIN_MODE":    mode,
		"PLUGIN_CACHES":  string(encoded),
		"CI_WORKSPACE":   workspace,
		"CI_CACHE_URL":   h.baseURL,
		"CI_CACHE_TOKEN": h.token,
	}
	var out bytes.Buffer
	run(context.Background(), func(k string) string { return env[k] }, &out)
	return out.String()
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	require.NoError(t, err)
	return string(data)
}

func TestSaveAndRestore(t *testing.T) {
	h := newHarness(t)
	spec := cacheSpec{Key: `go-{{ checksum "go.sum" }}`, RestoreKeys: []string{"go-"}, Paths: []string{".cache/go", "missing"}}

	first := t.TempDir()
	writeFile(t, first, "go.sum", "v1")
	writeFile(t, first, ".cache/go/mod/a.txt", "module a")
	writeFile(t, first, ".cache/go/bin/tool", "#!/bin/sh")
	require.NoError(t, os.Chmod(filepath.Join(first, ".cache/go/bin/tool"), 0o755))
	require.NoError(t, os.Symlink("mod/a.txt", filepath.Join(first, ".cache/go/link")))
	writeFile(t, first, "src/main.go", "not cached")

	out := h.run(t, "restore", first, spec)
	assert.Contains(t, out, "cache miss for go-")

	out = h.run(t, "save", first, spec)
	assert.Contains(t, out, "saved go-")
	assert.Contains(t, out, "path missing does not exist")

	out = h.run(t, "save", first, spec)
	assert.Contains(t, out, "is already cached, not saving")

	second := t.TempDir()
	writeFile(t, second, "go.sum", "v1")
	out = h.run(t, "restore", second, spec)
	assert.Contains(t, out, "exact match")
	assert.Equal(t, "module a", readFile(t, second, ".cache/go/mod/a.txt"))
	info, err := os.Stat(filepath.Join(second, ".cache/go/bin/tool"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	link, err := os.Readlink(filepath.Join(second, ".cache/go/link"))
	require.NoError(t, err)
	assert.Equal(t, "mod/a.txt", link)
	assert.NoFileExists(t, filepath.Join(second, "src/main.go"))

	// A changed lock file misses the key but restores through the restore key.
	third := t.TempDir()
	writeFile(t, third, "go.sum", "v2")
	out = h.run(t, "restore", third, spec)
	assert.Contains(t, out, "restore key match go-")
	assert.Equal(t, "module a", readFile(t, third, ".cache/go/mod/a.txt"))
}

func TestFinishedWorkflowCannotUseCache(t *testing.T) {
	h := newHarness(t)
	workspace := t.TempDir()
	writeFile(t, workspace, "a/file", "x")

	h.workflow.State = model.StatusSuccess
	out := h.run(t, "save", workspace, cacheSpec{Key: "k-{{ arch }}", Paths: []string{"a"}})
	assert.Contains(t, out, "warning: k-"+runtime.GOARCH+":", "the warning names the rendered key")
	assert.Contains(t, out, "cache token is not valid")
}

func TestEntryWithoutArchiveIsSavedAgain(t *testing.T) {
	h := newHarness(t)
	workspace := t.TempDir()
	writeFile(t, workspace, "a/file", "v1")
	spec := cacheSpec{Key: "k", Paths: []string{"a"}}
	assert.Contains(t, h.run(t, "save", workspace, spec), "saved k")

	// The archive vanishes from storage, e.g. deleted by a bucket lifecycle rule.
	archives, err := filepath.Glob(filepath.Join(h.storageRoot, "*", "*.tar.zst"))
	require.NoError(t, err)
	require.Len(t, archives, 1)
	require.NoError(t, os.Remove(archives[0]))

	restored := t.TempDir()
	assert.Contains(t, h.run(t, "restore", restored, spec), "cache miss for k")
	assert.Contains(t, h.run(t, "save", workspace, spec), "saved k")
	assert.Contains(t, h.run(t, "restore", restored, spec), "restoring k (exact match")
	assert.Equal(t, "v1", readFile(t, restored, "a/file"))
}

func TestCacheDisabled(t *testing.T) {
	var out bytes.Buffer
	env := map[string]string{"PLUGIN_MODE": "restore", "PLUGIN_CACHES": `[{"key":"k","paths":["a"]}]`}
	run(context.Background(), func(k string) string { return env[k] }, &out)
	assert.Contains(t, out.String(), "not enabled")
}

func TestKeyRenderFailureIsAWarning(t *testing.T) {
	h := newHarness(t)
	out := h.run(t, "restore", t.TempDir(), cacheSpec{Key: `{{ checksum "nope.lock" }}`, Paths: []string{"a"}})
	assert.True(t, strings.HasPrefix(out, "warning: "), out)
	assert.Contains(t, out, "match no file")
}

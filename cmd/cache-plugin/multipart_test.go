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
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/services/buildcache"
)

// partStorage takes uploads in parts like an object store: parts are PUT to
// its own server and assembled into the wrapped storage on completion. The
// first attempt at failPart answers 500.
type partStorage struct {
	buildcache.Storage
	srv      *httptest.Server
	failPart int

	mu      sync.Mutex
	uploads map[string]map[int][]byte
	failed  bool
	aborted []string
}

func newPartStorage(t *testing.T, storage buildcache.Storage, failPart int) *partStorage {
	t.Helper()
	p := &partStorage{Storage: storage, failPart: failPart, uploads: map[string]map[int][]byte{}}
	p.srv = httptest.NewServer(http.HandlerFunc(p.servePart))
	t.Cleanup(p.srv.Close)
	return p
}

func (p *partStorage) servePart(w http.ResponseWriter, r *http.Request) {
	number, err := strconv.Atoi(r.URL.Query().Get("partNumber"))
	if err != nil {
		http.Error(w, "bad part number", http.StatusBadRequest)
		return
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if number == p.failPart && !p.failed {
		p.failed = true
		http.Error(w, "try again", http.StatusInternalServerError)
		return
	}
	parts, ok := p.uploads[r.URL.Query().Get("uploadId")]
	if !ok {
		http.Error(w, "no such upload", http.StatusNotFound)
		return
	}
	parts[number] = data
}

func (p *partStorage) StartMultipart(_ context.Context, name string, parts int, _ time.Duration) (string, []string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	uploadID := fmt.Sprintf("upload-%d", len(p.uploads)+1)
	p.uploads[uploadID] = map[int][]byte{}
	urls := make([]string, parts)
	for i := range urls {
		urls[i] = fmt.Sprintf("%s/%s?partNumber=%d&uploadId=%s", p.srv.URL, name, i+1, uploadID)
	}
	return uploadID, urls, nil
}

func (p *partStorage) CompleteMultipart(ctx context.Context, name, uploadID string) error {
	p.mu.Lock()
	parts, ok := p.uploads[uploadID]
	delete(p.uploads, uploadID)
	p.mu.Unlock()
	if !ok || len(parts) == 0 {
		return buildcache.ErrObjectNotFound
	}
	var whole bytes.Buffer
	for number := 1; number <= len(parts); number++ {
		part, ok := parts[number]
		if !ok {
			return fmt.Errorf("part %d is missing", number)
		}
		whole.Write(part)
	}
	_, err := p.Write(ctx, name, &whole, int64(whole.Len()))
	return err
}

func (p *partStorage) AbortMultipart(_ context.Context, name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.aborted = append(p.aborted, name)
	return nil
}

func TestSaveAndRestoreInParts(t *testing.T) {
	var parts *partStorage
	h := newHarness(t, func(s buildcache.Storage) buildcache.Storage {
		parts = newPartStorage(t, s, 2)
		return parts
	})
	spec := cacheSpec{Key: "big", Paths: []string{"data"}}

	// Random data does not compress, so the archive spans three parts.
	content := make([]byte, 40<<20)
	_, err := rand.Read(content)
	require.NoError(t, err)
	first := t.TempDir()
	writeFile(t, first, "data/blob", string(content))

	out := h.run(t, "save", first, spec)
	assert.Contains(t, out, "uploading in 3 parts, 3 at a time")
	assert.Contains(t, out, "upload of part 2 failed, retrying")
	assert.Contains(t, out, "saved big")
	assert.Empty(t, parts.uploads, "the upload was completed")

	second := t.TempDir()
	out = h.run(t, "restore", second, spec)
	assert.Contains(t, out, "restoring big (exact match")
	assert.True(t, readFile(t, second, "data/blob") == string(content), "restored content differs")
}

func TestSmallArchiveIsUploadedWhole(t *testing.T) {
	var parts *partStorage
	h := newHarness(t, func(s buildcache.Storage) buildcache.Storage {
		parts = newPartStorage(t, s, 0)
		return parts
	})
	workspace := t.TempDir()
	writeFile(t, workspace, "data/file", "small")

	out := h.run(t, "save", workspace, cacheSpec{Key: "small", Paths: []string{"data"}})
	assert.NotContains(t, out, "parts")
	assert.Contains(t, out, "saved small")
}

func TestAbandonedUploadInPartsIsAborted(t *testing.T) {
	var parts *partStorage
	h := newHarness(t, func(s buildcache.Storage) buildcache.Storage {
		parts = newPartStorage(t, s, 0)
		return parts
	})
	access := buildcache.Access{RepoID: 3, ReadScopes: []string{"refs/heads/main"}, WriteScope: "refs/heads/main"}
	reservation, err := h.service.Reserve(context.Background(), access, "abandoned", 64<<20)
	require.NoError(t, err)
	require.NoError(t, h.service.UploadInParts(context.Background(), reservation))
	assert.Len(t, reservation.PartURLs, 4)
	assert.Equal(t, int64(16<<20), reservation.PartSize)

	require.NoError(t, h.service.Purge(context.Background(), 3))
	require.Len(t, parts.aborted, 1)
	assert.True(t, strings.HasSuffix(parts.aborted[0], ".tar.zst"))
}

// TestSaveAndRestoreInPartsOnS3 runs a save and restore of a large archive
// against a real bucket, set up with CROW_TEST_S3_ENDPOINT, _BUCKET, _REGION,
// _ACCESS_KEY and _SECRET_KEY. Objects go below the prefix crow-test.
func TestSaveAndRestoreInPartsOnS3(t *testing.T) {
	env := func(name string) string { return os.Getenv("CROW_TEST_S3_" + name) }
	if env("ENDPOINT") == "" {
		t.Skip("CROW_TEST_S3_ENDPOINT is not set")
	}
	storage, err := buildcache.NewS3Storage(context.Background(), buildcache.S3Options{
		Endpoint:  env("ENDPOINT"),
		Bucket:    env("BUCKET"),
		Region:    env("REGION"),
		AccessKey: env("ACCESS_KEY"),
		SecretKey: env("SECRET_KEY"),
		SSL:       true,
		Prefix:    "crow-test",
	})
	require.NoError(t, err)
	h := newHarness(t, func(buildcache.Storage) buildcache.Storage { return storage })
	t.Cleanup(func() { assert.NoError(t, h.service.Purge(context.Background(), 3)) })
	spec := cacheSpec{Key: "big", Paths: []string{"data"}}

	content := make([]byte, 400<<20)
	_, err = rand.Read(content)
	require.NoError(t, err)
	first := t.TempDir()
	writeFile(t, first, "data/blob", string(content))

	out := h.run(t, "save", first, spec)
	t.Log(out)
	assert.Contains(t, out, "uploading in 26 parts")
	assert.Contains(t, out, "saved big")

	second := t.TempDir()
	out = h.run(t, "restore", second, spec)
	t.Log(out)
	assert.True(t, readFile(t, second, "data/blob") == string(content), "restored content differs")
}

// Copyright 2025 Crow Authors
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

package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v5/server/model"
)

// fakeS3 is a minimal in-memory S3-compatible server for testing.
type fakeS3 struct {
	mu      sync.RWMutex
	objects map[string][]byte
	bucket  string
}

func newFakeS3(bucket string) *fakeS3 {
	return &fakeS3{
		objects: make(map[string][]byte),
		bucket:  bucket,
	}
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle bucket-level requests: /{bucket} or /{bucket}/
	isBucketLevel := path == "/"+f.bucket || path == "/"+f.bucket+"/"

	// Extract object key: strip /{bucket}/ prefix
	key := strings.TrimPrefix(path, "/"+f.bucket+"/")

	switch r.Method {
	case http.MethodHead:
		if isBucketLevel {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			return
		}
		f.mu.RLock()
		data, ok := f.objects[key]
		f.mu.RUnlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		f.setObjectHeaders(w, data)
		w.WriteHeader(http.StatusOK)

	case http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.objects[key] = body
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		if isBucketLevel || r.URL.Query().Get("list-type") == "2" {
			f.handleListObjects(w, r)
			return
		}
		f.mu.RLock()
		data, ok := f.objects[key]
		f.mu.RUnlock()
		if !ok {
			http.Error(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code></Error>`, http.StatusNotFound)
			return
		}
		f.setObjectHeaders(w, data)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)

	case http.MethodDelete:
		f.mu.Lock()
		delete(f.objects, key)
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (f *fakeS3) setObjectHeaders(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("ETag", `"test-etag"`)
}

func (f *fakeS3) handleListObjects(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")

	f.mu.RLock()
	var keys []string
	for key := range f.objects {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	f.mu.RUnlock()

	// Build a minimal ListBucketResult XML response
	var contents strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&contents, "<Contents><Key>%s</Key></Contents>", key)
	}

	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>%s</Name>
  <Prefix>%s</Prefix>
  <IsTruncated>false</IsTruncated>
  %s
</ListBucketResult>`, f.bucket, prefix, contents.String())

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml))
}

func newTestLogStore(t *testing.T) (*logStore, *httptest.Server) {
	t.Helper()
	bucket := "test-logs"
	fake := newFakeS3(bucket)
	server := httptest.NewServer(fake)

	endpoint := strings.TrimPrefix(server.URL, "http://")
	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4("", "", ""),
		Secure:       false,
		BucketLookup: minio.BucketLookupPath,
	})
	require.NoError(t, err)

	store := &logStore{
		client: client,
		bucket: bucket,
		prefix: "logs",
	}

	return store, server
}

func TestStepPrefix(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		step     *model.Step
		expected string
	}{
		{
			name:     "with prefix",
			prefix:   "logs",
			step:     &model.Step{ID: 42, PipelineID: 10},
			expected: "logs/10/42/",
		},
		{
			name:     "without prefix",
			prefix:   "",
			step:     &model.Step{ID: 1, PipelineID: 5},
			expected: "5/1/",
		},
		{
			name:     "nested prefix",
			prefix:   "ci/logs",
			step:     &model.Step{ID: 100, PipelineID: 200},
			expected: "ci/logs/200/100/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &logStore{prefix: tt.prefix}
			assert.Equal(t, tt.expected, store.stepPrefix(tt.step))
		})
	}
}

func TestChunkKey(t *testing.T) {
	store := &logStore{prefix: "logs"}
	step := &model.Step{ID: 42, PipelineID: 10}

	key := store.chunkKey(step)

	assert.True(t, strings.HasPrefix(key, "logs/10/42/"))
	assert.True(t, strings.HasSuffix(key, ".ndjson"))

	// Two consecutive calls should produce different keys
	key2 := store.chunkKey(step)
	// They should differ (nanosecond timestamp), though in rare cases they could match
	// on very fast systems - just check they're both valid format
	assert.True(t, strings.HasPrefix(key2, "logs/10/42/"))
	assert.True(t, strings.HasSuffix(key2, ".ndjson"))
}

func TestLogAppendAndFind(t *testing.T) {
	store, server := newTestLogStore(t)
	defer server.Close()

	step := &model.Step{ID: 1, PipelineID: 10}

	entries := []*model.LogEntry{
		{StepID: 1, Data: []byte("hello"), Line: 1, Time: 0, Type: model.LogEntryStdout},
		{StepID: 1, Data: []byte("world"), Line: 2, Time: 1, Type: model.LogEntryStdout},
	}

	// Append entries
	err := store.LogAppend(step, entries)
	require.NoError(t, err)

	// Find entries
	found, err := store.LogFind(step)
	require.NoError(t, err)
	assert.Len(t, found, 2)
	assert.Equal(t, 1, found[0].Line)
	assert.Equal(t, []byte("hello"), found[0].Data)
	assert.Equal(t, 2, found[1].Line)
	assert.Equal(t, []byte("world"), found[1].Data)
}

func TestLogAppendMultipleChunks(t *testing.T) {
	store, server := newTestLogStore(t)
	defer server.Close()

	step := &model.Step{ID: 1, PipelineID: 10}

	// Append first batch
	batch1 := []*model.LogEntry{
		{StepID: 1, Data: []byte("line1"), Line: 1, Time: 0},
		{StepID: 1, Data: []byte("line2"), Line: 2, Time: 1},
	}
	require.NoError(t, store.LogAppend(step, batch1))

	// Append second batch
	batch2 := []*model.LogEntry{
		{StepID: 1, Data: []byte("line3"), Line: 3, Time: 2},
		{StepID: 1, Data: []byte("line4"), Line: 4, Time: 3},
	}
	require.NoError(t, store.LogAppend(step, batch2))

	// Find should merge both chunks sorted by line number
	found, err := store.LogFind(step)
	require.NoError(t, err)
	assert.Len(t, found, 4)
	for i, entry := range found {
		assert.Equal(t, i+1, entry.Line)
	}
}

func TestLogAppendEmpty(t *testing.T) {
	store, server := newTestLogStore(t)
	defer server.Close()

	step := &model.Step{ID: 1, PipelineID: 10}

	// Empty append should be a no-op
	err := store.LogAppend(step, []*model.LogEntry{})
	assert.NoError(t, err)

	err = store.LogAppend(step, nil)
	assert.NoError(t, err)
}

func TestLogFindEmpty(t *testing.T) {
	store, server := newTestLogStore(t)
	defer server.Close()

	step := &model.Step{ID: 999, PipelineID: 10}

	// No logs stored for this step
	found, err := store.LogFind(step)
	assert.NoError(t, err)
	assert.Empty(t, found)
}

func TestLogDelete(t *testing.T) {
	store, server := newTestLogStore(t)
	defer server.Close()

	step := &model.Step{ID: 1, PipelineID: 10}

	// Store some logs across multiple chunks
	require.NoError(t, store.LogAppend(step, []*model.LogEntry{
		{StepID: 1, Data: []byte("hello"), Line: 1, Time: 0},
	}))
	require.NoError(t, store.LogAppend(step, []*model.LogEntry{
		{StepID: 1, Data: []byte("world"), Line: 2, Time: 1},
	}))

	// Verify logs exist
	found, err := store.LogFind(step)
	require.NoError(t, err)
	assert.Len(t, found, 2)

	// Delete logs
	err = store.LogDelete(step)
	require.NoError(t, err)

	// Verify logs are gone
	found, err = store.LogFind(step)
	require.NoError(t, err)
	assert.Empty(t, found)
}

func TestLogDeleteEmpty(t *testing.T) {
	store, server := newTestLogStore(t)
	defer server.Close()

	step := &model.Step{ID: 999, PipelineID: 10}

	// Deleting non-existent logs should not error
	err := store.LogDelete(step)
	assert.NoError(t, err)
}

func TestLogIsolationBetweenSteps(t *testing.T) {
	store, server := newTestLogStore(t)
	defer server.Close()

	step1 := &model.Step{ID: 1, PipelineID: 10}
	step2 := &model.Step{ID: 2, PipelineID: 10}

	// Store logs for step1
	require.NoError(t, store.LogAppend(step1, []*model.LogEntry{
		{StepID: 1, Data: []byte("step1-line"), Line: 1, Time: 0},
	}))

	// Store logs for step2
	require.NoError(t, store.LogAppend(step2, []*model.LogEntry{
		{StepID: 2, Data: []byte("step2-line"), Line: 1, Time: 0},
	}))

	// Each step should only see its own logs
	found1, err := store.LogFind(step1)
	require.NoError(t, err)
	assert.Len(t, found1, 1)
	assert.Equal(t, []byte("step1-line"), found1[0].Data)

	found2, err := store.LogFind(step2)
	require.NoError(t, err)
	assert.Len(t, found2, 1)
	assert.Equal(t, []byte("step2-line"), found2[0].Data)

	// Deleting step1 should not affect step2
	require.NoError(t, store.LogDelete(step1))

	found1, err = store.LogFind(step1)
	require.NoError(t, err)
	assert.Empty(t, found1)

	found2, err = store.LogFind(step2)
	require.NoError(t, err)
	assert.Len(t, found2, 1)
}

func TestLogIsolationBetweenPipelines(t *testing.T) {
	store, server := newTestLogStore(t)
	defer server.Close()

	step1 := &model.Step{ID: 1, PipelineID: 10}
	step2 := &model.Step{ID: 1, PipelineID: 20} // same step ID, different pipeline

	require.NoError(t, store.LogAppend(step1, []*model.LogEntry{
		{StepID: 1, Data: []byte("pipeline10"), Line: 1, Time: 0},
	}))
	require.NoError(t, store.LogAppend(step2, []*model.LogEntry{
		{StepID: 1, Data: []byte("pipeline20"), Line: 1, Time: 0},
	}))

	found1, err := store.LogFind(step1)
	require.NoError(t, err)
	assert.Len(t, found1, 1)
	assert.Equal(t, []byte("pipeline10"), found1[0].Data)

	found2, err := store.LogFind(step2)
	require.NoError(t, err)
	assert.Len(t, found2, 1)
	assert.Equal(t, []byte("pipeline20"), found2[0].Data)
}

func TestLogFindSortsEntriesByLine(t *testing.T) {
	store, server := newTestLogStore(t)
	defer server.Close()

	step := &model.Step{ID: 1, PipelineID: 10}

	// Append entries out of order across multiple chunks
	require.NoError(t, store.LogAppend(step, []*model.LogEntry{
		{StepID: 1, Data: []byte("third"), Line: 3, Time: 2},
	}))
	require.NoError(t, store.LogAppend(step, []*model.LogEntry{
		{StepID: 1, Data: []byte("first"), Line: 1, Time: 0},
	}))
	require.NoError(t, store.LogAppend(step, []*model.LogEntry{
		{StepID: 1, Data: []byte("second"), Line: 2, Time: 1},
	}))

	found, err := store.LogFind(step)
	require.NoError(t, err)
	require.Len(t, found, 3)
	assert.Equal(t, []byte("first"), found[0].Data)
	assert.Equal(t, []byte("second"), found[1].Data)
	assert.Equal(t, []byte("third"), found[2].Data)
}

func TestLogEntryTypes(t *testing.T) {
	store, server := newTestLogStore(t)
	defer server.Close()

	step := &model.Step{ID: 1, PipelineID: 10}

	entries := []*model.LogEntry{
		{StepID: 1, Data: []byte("stdout line"), Line: 1, Time: 0, Type: model.LogEntryStdout},
		{StepID: 1, Data: []byte("stderr line"), Line: 2, Time: 1, Type: model.LogEntryStderr},
		{StepID: 1, Data: []byte("0"), Line: 3, Time: 2, Type: model.LogEntryExitCode},
		{StepID: 1, Data: []byte("{}"), Line: 4, Time: 3, Type: model.LogEntryMetadata},
		{StepID: 1, Data: []byte("50"), Line: 5, Time: 4, Type: model.LogEntryProgress},
	}

	require.NoError(t, store.LogAppend(step, entries))

	found, err := store.LogFind(step)
	require.NoError(t, err)
	require.Len(t, found, 5)

	assert.Equal(t, model.LogEntryStdout, found[0].Type)
	assert.Equal(t, model.LogEntryStderr, found[1].Type)
	assert.Equal(t, model.LogEntryExitCode, found[2].Type)
	assert.Equal(t, model.LogEntryMetadata, found[3].Type)
	assert.Equal(t, model.LogEntryProgress, found[4].Type)
}

func TestNewLogStoreValidation(t *testing.T) {
	ctx := context.Background()

	_, err := NewLogStore(ctx, Options{})
	assert.ErrorContains(t, err, "S3 endpoint is required")

	_, err = NewLogStore(ctx, Options{Endpoint: "localhost:9000"})
	assert.ErrorContains(t, err, "S3 bucket is required")
}

func TestReadChunkParsesNDJSON(t *testing.T) {
	store, server := newTestLogStore(t)
	defer server.Close()

	step := &model.Step{ID: 1, PipelineID: 10}

	// Create well-formed NDJSON with various entries
	entries := []*model.LogEntry{
		{StepID: 1, Data: []byte("line with special chars: <>&\""), Line: 1, Time: 0, Type: model.LogEntryStdout},
		{StepID: 1, Data: []byte(""), Line: 2, Time: 1, Type: model.LogEntryStdout},
		{StepID: 1, Data: []byte("unicode: 日本語テスト"), Line: 3, Time: 2, Type: model.LogEntryStdout},
	}

	require.NoError(t, store.LogAppend(step, entries))

	found, err := store.LogFind(step)
	require.NoError(t, err)
	require.Len(t, found, 3)

	assert.Equal(t, []byte("line with special chars: <>&\""), found[0].Data)
	assert.Equal(t, []byte(""), found[1].Data)
	assert.Equal(t, []byte("unicode: 日本語テスト"), found[2].Data)
}

func TestLogAppendPreservesAllFields(t *testing.T) {
	store, server := newTestLogStore(t)
	defer server.Close()

	step := &model.Step{ID: 42, PipelineID: 10}

	entry := &model.LogEntry{
		StepID: 42,
		Data:   []byte("test data"),
		Line:   7,
		Time:   12345,
		Type:   model.LogEntryStderr,
	}

	require.NoError(t, store.LogAppend(step, []*model.LogEntry{entry}))

	found, err := store.LogFind(step)
	require.NoError(t, err)
	require.Len(t, found, 1)

	assert.Equal(t, int64(42), found[0].StepID)
	assert.Equal(t, []byte("test data"), found[0].Data)
	assert.Equal(t, 7, found[0].Line)
	assert.Equal(t, int64(12345), found[0].Time)
	assert.Equal(t, model.LogEntryStderr, found[0].Type)
}

func TestLogRoundTripJSON(t *testing.T) {
	// Verify that our NDJSON serialization round-trips correctly
	// by manually checking the format
	entry := &model.LogEntry{
		StepID: 1,
		Data:   []byte("hello world"),
		Line:   1,
		Time:   100,
		Type:   model.LogEntryStdout,
	}

	jsonLine, err := json.Marshal(entry)
	require.NoError(t, err)

	var parsed model.LogEntry
	require.NoError(t, json.Unmarshal(jsonLine, &parsed))

	assert.Equal(t, entry.StepID, parsed.StepID)
	assert.Equal(t, entry.Data, parsed.Data)
	assert.Equal(t, entry.Line, parsed.Line)
	assert.Equal(t, entry.Time, parsed.Time)
	assert.Equal(t, entry.Type, parsed.Type)
}

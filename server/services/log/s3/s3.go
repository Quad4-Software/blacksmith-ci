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

package s3

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/pipeline"
	"codefloe.com/crowci/crow/v5/server/model"
	logService "codefloe.com/crowci/crow/v5/server/services/log"
)

const (
	// MaxLineLength accounts for base64 overhead and JSON field wrappers.
	maxLineLength int = (pipeline.MaxLogLineLength/3)*4 + (64 * 1024) //nolint:mnd
)

type Options struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	SSL       bool
	PathStyle bool
	Prefix    string
}

type logStore struct {
	client *minio.Client
	bucket string
	prefix string
}

func NewLogStore(ctx context.Context, opts Options) (logService.Service, error) {
	if opts.Endpoint == "" {
		return nil, fmt.Errorf("S3 endpoint is required")
	}
	if opts.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}

	client, err := minio.New(opts.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, ""),
		Secure: opts.SSL,
		BucketLookup: func() minio.BucketLookupType {
			if opts.PathStyle {
				return minio.BucketLookupPath
			}
			return minio.BucketLookupAuto
		}(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second) //nolint:mnd
	defer cancel()

	exists, err := client.BucketExists(checkCtx, opts.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check if S3 bucket exists: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("S3 bucket %q does not exist", opts.Bucket)
	}

	return &logStore{
		client: client,
		bucket: opts.Bucket,
		prefix: opts.Prefix,
	}, nil
}

// stepPrefix returns the S3 key prefix for a step's log chunks.
func (l *logStore) stepPrefix(step *model.Step) string {
	parts := []string{}
	if l.prefix != "" {
		parts = append(parts, l.prefix)
	}
	parts = append(parts, fmt.Sprintf("%d", step.PipelineID), fmt.Sprintf("%d", step.ID))
	return strings.Join(parts, "/") + "/"
}

func (l *logStore) LogFind(step *model.Step) ([]*model.LogEntry, error) {
	ctx := context.Background()
	prefix := l.stepPrefix(step)

	var allEntries []*model.LogEntry

	objectCh := l.client.ListObjects(ctx, l.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for objInfo := range objectCh {
		if objInfo.Err != nil {
			return nil, fmt.Errorf("error listing S3 objects: %w", objInfo.Err)
		}

		entries, err := l.readChunk(ctx, objInfo.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to read log chunk %q: %w", objInfo.Key, err)
		}
		allEntries = append(allEntries, entries...)
	}

	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Line < allEntries[j].Line
	})

	return allEntries, nil
}

func (l *logStore) LogAppend(step *model.Step, logEntries []*model.LogEntry) error {
	if len(logEntries) == 0 {
		return nil
	}

	ctx := context.Background()
	key := l.chunkKey(step)

	var buf bytes.Buffer
	for _, entry := range logEntries {
		jsonLine, err := json.Marshal(entry)
		if err != nil {
			log.Error().Err(err).Msg("could not convert log entry to JSON")
			continue
		}
		buf.Write(jsonLine)
		buf.WriteByte('\n')
	}

	if buf.Len() == 0 {
		return nil
	}

	_, err := l.client.PutObject(ctx, l.bucket, key, &buf, int64(buf.Len()), minio.PutObjectOptions{
		ContentType: "application/x-ndjson",
	})
	if err != nil {
		return fmt.Errorf("failed to write log chunk to S3: %w", err)
	}

	return nil
}

func (l *logStore) LogDelete(step *model.Step) error {
	ctx := context.Background()
	prefix := l.stepPrefix(step)

	objectCh := l.client.ListObjects(ctx, l.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	var errs []error
	for objInfo := range objectCh {
		if objInfo.Err != nil {
			return fmt.Errorf("error listing S3 objects for deletion: %w", objInfo.Err)
		}

		if err := l.client.RemoveObject(ctx, l.bucket, objInfo.Key, minio.RemoveObjectOptions{}); err != nil {
			log.Error().Err(err).Str("key", objInfo.Key).Msg("failed to delete log chunk from S3")
			errs = append(errs, fmt.Errorf("failed to delete %q: %w", objInfo.Key, err))
		}
	}

	return errors.Join(errs...)
}

// chunkKey generates a unique key for a new log chunk.
func (l *logStore) chunkKey(step *model.Step) string {
	prefix := l.stepPrefix(step)
	return fmt.Sprintf("%s%d.ndjson", prefix, time.Now().UnixNano())
}

// readChunk reads and parses a single NDJSON chunk from S3.
func (l *logStore) readChunk(ctx context.Context, key string) ([]*model.LogEntry, error) {
	obj, err := l.client.GetObject(ctx, l.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 object: %w", err)
	}
	defer obj.Close()

	buf := make([]byte, 0, bufio.MaxScanTokenSize)
	s := bufio.NewScanner(obj)
	s.Buffer(buf, maxLineLength)

	var entries []*model.LogEntry
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		entry := &model.LogEntry{}
		if err := json.Unmarshal([]byte(line), entry); err != nil {
			return nil, fmt.Errorf("failed to parse log entry: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan S3 object: %w", err)
	}

	return entries, nil
}

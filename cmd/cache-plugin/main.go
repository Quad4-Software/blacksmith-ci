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

// Command cache-plugin is the image of the cache steps the compiler adds for a
// workflow's `cache` section. In restore mode it downloads and extracts the
// caches after clone; in save mode it archives and uploads them after the
// steps. A cache is an optimization, so every problem is reported as a warning
// and the step still succeeds.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"

	"codefloe.com/crowci/crow/v6/shared/cachekey"
)

// cacheSpec mirrors compiler.CacheSpec.
type cacheSpec struct {
	Key         string   `json:"key"`
	RestoreKeys []string `json:"restore_keys,omitempty"`
	Paths       []string `json:"paths"`
}

type plugin struct {
	client    *client
	workspace string
	out       io.Writer
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	run(ctx, os.Getenv, os.Stdout)
}

// run never fails: a broken cache must not fail the workflow, so problems are
// printed as warnings and the step exits successfully.
func run(ctx context.Context, getenv func(string) string, out io.Writer) {
	var specs []cacheSpec
	if err := json.Unmarshal([]byte(getenv("PLUGIN_CACHES")), &specs); err != nil {
		fmt.Fprintf(out, "warning: cannot read the cache configuration: %v\n", err)
		return
	}

	baseURL, token := getenv("CI_CACHE_URL"), getenv("CI_CACHE_TOKEN")
	if baseURL == "" || token == "" {
		fmt.Fprintln(out, "warning: build caching is not enabled on this Crow server, skipping")
		return
	}

	workspace := getenv("CI_WORKSPACE")
	if workspace == "" {
		workspace = "."
	}
	out = &syncWriter{w: out}
	p := &plugin{
		client:    &client{baseURL: baseURL, token: token, http: http.DefaultClient, out: out},
		workspace: workspace,
		out:       out,
	}

	mode := getenv("PLUGIN_MODE")
	if mode != "restore" && mode != "save" {
		fmt.Fprintf(out, "warning: unknown cache mode %q\n", mode)
		return
	}
	for _, spec := range specs {
		key, restoreKeys, err := p.render(spec)
		if err != nil {
			fmt.Fprintf(out, "warning: %s: %v\n", spec.Key, err)
			continue
		}
		if mode == "restore" {
			err = p.restore(ctx, spec, key, restoreKeys)
		} else {
			err = p.save(ctx, spec, key)
		}
		if err != nil {
			fmt.Fprintf(out, "warning: %s: %v\n", key, err)
		}
	}
}

func (p *plugin) render(spec cacheSpec) (string, []string, error) {
	key, err := cachekey.Render(spec.Key, p.workspace)
	if err != nil {
		return "", nil, fmt.Errorf("cannot render key: %w", err)
	}
	restoreKeys := make([]string, 0, len(spec.RestoreKeys))
	for _, restoreKey := range spec.RestoreKeys {
		rendered, err := cachekey.Render(restoreKey, p.workspace)
		if err != nil {
			return "", nil, fmt.Errorf("cannot render restore key %q: %w", restoreKey, err)
		}
		restoreKeys = append(restoreKeys, rendered)
	}
	return key, restoreKeys, nil
}

func (p *plugin) restore(ctx context.Context, spec cacheSpec, key string, restoreKeys []string) error {
	hit, err := p.client.restore(ctx, key, restoreKeys)
	if errors.Is(err, errMiss) {
		fmt.Fprintf(p.out, "cache miss for %s\n", key)
		return nil
	}
	if errors.Is(err, errEgress) {
		fmt.Fprintf(p.out, "not restoring %s: %v\n", key, err)
		return nil
	}
	if err != nil {
		return err
	}

	match := "exact match"
	if !hit.Exact {
		match = "restore key match " + hit.Key
	}
	fmt.Fprintf(p.out, "restoring %s (%s, %s from %s)\n", key, match, humanize.IBytes(uint64(max(hit.Size, 0))), hit.Scope)

	// Download and extraction are one attempt: a transfer that breaks off or
	// stalls is started over, after removing what it had extracted so far.
	start := time.Now()
	var files int
	err = withRetry(ctx, p.out, "restore of "+key, true, func() error {
		received := new(counter)
		attemptCtx, stop := watch(ctx, p.out, received, true, transferLine("downloaded", hit.Size))
		n, err := p.restoreOnce(attemptCtx, hit.URL, spec.Paths, received)
		files = n
		return stop(err)
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(p.out, "restored %s (%d files in %s)\n", key, files, time.Since(start).Round(time.Second))
	return nil
}

// restoreOnce downloads the archive at url and extracts it, counting the
// downloaded bytes in received. A failed extraction is undone.
func (p *plugin) restoreOnce(ctx context.Context, url string, paths []string, received *counter) (int, error) {
	body, err := p.client.download(ctx, url)
	if err != nil {
		return 0, fmt.Errorf("download: %w", err)
	}
	defer body.Close()
	extracted, err := extractArchive(received.reader(body), p.workspace, paths)
	if err != nil {
		extracted.undo()
		return 0, fmt.Errorf("extract: %w", err)
	}
	return extracted.files, nil
}

func (p *plugin) save(ctx context.Context, spec cacheSpec, key string) error {
	// A key is saved once; an exact hit means there is nothing to upload.
	if exists, err := p.client.exists(ctx, key); err != nil {
		return err
	} else if exists {
		fmt.Fprintf(p.out, "%s is already cached, not saving\n", key)
		return nil
	}

	archive, err := os.CreateTemp("", "crow-cache-*.tar.zst")
	if err != nil {
		return err
	}
	defer os.Remove(archive.Name())
	defer archive.Close()

	fmt.Fprintf(p.out, "archiving %s for %s\n", strings.Join(spec.Paths, ", "), key)
	start := time.Now()
	archived := new(counter)
	_, stop := watch(ctx, p.out, archived, false, func(done int64, elapsed time.Duration) string {
		return fmt.Sprintf("archiving: %d files, %s read, %s", archived.files.Load(), humanize.IBytes(uint64(done)), rate(done, elapsed))
	})
	missing, err := writeArchive(archive, p.workspace, spec.Paths, archived)
	_ = stop(nil)
	if err != nil {
		return fmt.Errorf("archive: %w", err)
	}
	for _, path := range missing {
		fmt.Fprintf(p.out, "warning: %s: path %s does not exist\n", key, path)
	}
	if len(missing) == len(spec.Paths) {
		fmt.Fprintf(p.out, "nothing to cache for %s\n", key)
		return nil
	}

	size, err := archive.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return err
	}
	fmt.Fprintf(p.out, "archived %d files (%s) into %s in %s\n", archived.files.Load(),
		humanize.IBytes(uint64(archived.bytes.Load())), humanize.IBytes(uint64(size)), time.Since(start).Round(time.Second))

	reservation, err := p.client.reserve(ctx, key, size)
	switch {
	case errors.Is(err, errExists):
		fmt.Fprintf(p.out, "%s was cached or is being saved by another pipeline meanwhile, not saving\n", key)
		return nil
	case errors.Is(err, errReadOnly), errors.Is(err, errQuota):
		return fmt.Errorf("not saved: %w", err)
	case err != nil:
		return err
	}

	start = time.Now()
	if err := p.client.upload(ctx, reservation, archive, size); err != nil {
		return fmt.Errorf("upload: %w", err)
	}
	if err := p.client.commit(ctx, reservation.ID, reservation.UploadID); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	fmt.Fprintf(p.out, "saved %s (%s uploaded in %s)\n", key, humanize.IBytes(uint64(size)), time.Since(start).Round(time.Second))
	return nil
}

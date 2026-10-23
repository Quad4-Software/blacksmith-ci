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

package buildcache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ErrObjectTooLarge is returned by Storage.Write when the stream is longer
// than the limit it was given.
var ErrObjectTooLarge = errors.New("cache archive exceeds its reserved size")

// ErrObjectNotFound is returned by Storage.Size when the object does not exist.
var ErrObjectNotFound = errors.New("cache archive does not exist")

// Storage holds cache archives. Object names are built by the service from
// entry IDs and never contain user input.
type Storage interface {
	// DownloadURL returns a URL the cache step fetches the object from
	// directly, or "" when the archive is streamed through the server.
	DownloadURL(ctx context.Context, name string, expiry time.Duration) (string, error)
	// UploadURL returns a URL the cache step PUTs the object to directly, or ""
	// when the archive is streamed through the server.
	UploadURL(ctx context.Context, name string, expiry time.Duration) (string, error)
	// Open reads an object streamed through the server.
	Open(ctx context.Context, name string) (io.ReadCloser, error)
	// Write stores an object streamed through the server, failing with
	// ErrObjectTooLarge once more than limit bytes arrive.
	Write(ctx context.Context, name string, r io.Reader, limit int64) (int64, error)
	// Size returns the stored size of an object, or ErrObjectNotFound.
	Size(ctx context.Context, name string) (int64, error)
	// Delete removes an object; deleting a missing object is not an error.
	Delete(ctx context.Context, name string) error
}

// MultipartStorage is a Storage that also takes an object in parts, each
// uploaded through its own presigned URL. Object stores accept a single upload
// at a few MB/s per connection, so a large archive uploads many times faster
// in parallel parts.
type MultipartStorage interface {
	Storage
	// StartMultipart begins an upload of name in parts and returns its ID and
	// one presigned PUT URL per part, in part order.
	StartMultipart(ctx context.Context, name string, parts int, expiry time.Duration) (string, []string, error)
	// CompleteMultipart assembles the uploaded parts of an upload into name.
	CompleteMultipart(ctx context.Context, name, uploadID string) error
	// AbortMultipart drops the unfinished uploads of name and their parts.
	AbortMultipart(ctx context.Context, name string) error
}

// localStorage keeps archives in a directory on the server and streams them
// through the cache API.
type localStorage struct {
	root string
}

const (
	localDirPerm  = 0o750
	localFilePerm = 0o640
)

// NewLocalStorage stores archives below root, creating it when missing.
func NewLocalStorage(root string) (Storage, error) {
	if root == "" {
		return nil, errors.New("cache directory is required")
	}
	if err := os.MkdirAll(root, localDirPerm); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}
	return &localStorage{root: root}, nil
}

func (l *localStorage) path(name string) string {
	return filepath.Join(l.root, filepath.FromSlash(name))
}

func (l *localStorage) DownloadURL(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

func (l *localStorage) UploadURL(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

func (l *localStorage) Open(_ context.Context, name string) (io.ReadCloser, error) {
	return os.Open(l.path(name))
}

// Write streams into a temporary file next to the target and renames it into
// place, so a reader never sees a partial archive.
func (l *localStorage) Write(_ context.Context, name string, r io.Reader, limit int64) (int64, error) {
	target := l.path(name)
	if err := os.MkdirAll(filepath.Dir(target), localDirPerm); err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".upload-*")
	if err != nil {
		return 0, err
	}
	defer os.Remove(tmp.Name())

	// Reading one byte past the limit tells an over-long stream apart from one
	// that is exactly limit bytes long.
	n, err := io.Copy(tmp, io.LimitReader(r, limit+1))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, ErrObjectTooLarge
	}
	if err := os.Chmod(tmp.Name(), localFilePerm); err != nil {
		return n, err
	}
	return n, os.Rename(tmp.Name(), target)
}

func (l *localStorage) Size(_ context.Context, name string) (int64, error) {
	info, err := os.Stat(l.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return 0, ErrObjectNotFound
	}
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (l *localStorage) Delete(_ context.Context, name string) error {
	err := os.Remove(l.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// S3Options configures the S3 storage.
type S3Options struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	SSL       bool
	PathStyle bool
	Prefix    string
	// PublicOnly refuses to connect to loopback, private, link-local and
	// other non-public addresses, for endpoints that users enter.
	PublicOnly bool
}

// s3Storage keeps archives in an S3-compatible bucket. The cache step talks to
// the bucket directly through presigned URLs, so archives never pass through
// the server and the step never sees the bucket credentials.
type s3Storage struct {
	client *minio.Client
	bucket string
	prefix string
}

const (
	s3CheckTimeout = 10 * time.Second
	// S3 lists at most this many parts of an upload per request.
	s3MaxListParts = 1000
)

// NewS3Storage connects to the bucket and verifies that it exists.
func NewS3Storage(ctx context.Context, opts S3Options) (Storage, error) {
	if opts.Endpoint == "" {
		return nil, errors.New("S3 endpoint is required")
	}
	if opts.Bucket == "" {
		return nil, errors.New("S3 bucket is required")
	}

	lookup := minio.BucketLookupAuto
	if opts.PathStyle {
		lookup = minio.BucketLookupPath
	}
	minioOpts := &minio.Options{
		Creds:        credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, ""),
		Secure:       opts.SSL,
		Region:       opts.Region,
		BucketLookup: lookup,
	}
	if opts.PublicOnly {
		minioOpts.Transport = publicOnlyTransport()
	}
	client, err := minio.New(opts.Endpoint, minioOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	checkCtx, cancel := context.WithTimeout(ctx, s3CheckTimeout)
	defer cancel()
	exists, err := client.BucketExists(checkCtx, opts.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check if S3 bucket exists: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("S3 bucket %q does not exist", opts.Bucket)
	}

	return &s3Storage{client: client, bucket: opts.Bucket, prefix: opts.Prefix}, nil
}

func (s *s3Storage) key(name string) string {
	if s.prefix == "" {
		return name
	}
	return s.prefix + "/" + name
}

func (s *s3Storage) DownloadURL(ctx context.Context, name string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, s.key(name), expiry, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *s3Storage) UploadURL(ctx context.Context, name string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedPutObject(ctx, s.bucket, s.key(name), expiry)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *s3Storage) StartMultipart(ctx context.Context, name string, parts int, expiry time.Duration) (string, []string, error) {
	core := minio.Core{Client: s.client}
	uploadID, err := core.NewMultipartUpload(ctx, s.bucket, s.key(name), minio.PutObjectOptions{ContentType: "application/zstd"})
	if err != nil {
		return "", nil, err
	}
	urls := make([]string, parts)
	for i := range urls {
		u, err := s.client.Presign(ctx, http.MethodPut, s.bucket, s.key(name), expiry, url.Values{
			"partNumber": {strconv.Itoa(i + 1)},
			"uploadId":   {uploadID},
		})
		if err != nil {
			return "", nil, errors.Join(err, core.AbortMultipartUpload(ctx, s.bucket, s.key(name), uploadID))
		}
		urls[i] = u.String()
	}
	return uploadID, urls, nil
}

// CompleteMultipart assembles every part the store received. The cache step
// does not report the parts' ETags, the store lists them.
func (s *s3Storage) CompleteMultipart(ctx context.Context, name, uploadID string) error {
	core := minio.Core{Client: s.client}
	var parts []minio.CompletePart
	marker := 0
	for {
		result, err := core.ListObjectParts(ctx, s.bucket, s.key(name), uploadID, marker, s3MaxListParts)
		if err != nil {
			return err
		}
		for _, part := range result.ObjectParts {
			parts = append(parts, minio.CompletePart{PartNumber: part.PartNumber, ETag: part.ETag})
		}
		if !result.IsTruncated {
			break
		}
		marker = result.NextPartNumberMarker
	}
	if len(parts) == 0 {
		return ErrObjectNotFound
	}
	_, err := core.CompleteMultipartUpload(ctx, s.bucket, s.key(name), uploadID, parts, minio.PutObjectOptions{ContentType: "application/zstd"})
	return err
}

func (s *s3Storage) AbortMultipart(ctx context.Context, name string) error {
	return s.client.RemoveIncompleteUpload(ctx, s.bucket, s.key(name))
}

func (s *s3Storage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	return s.client.GetObject(ctx, s.bucket, s.key(name), minio.GetObjectOptions{})
}

func (s *s3Storage) Write(ctx context.Context, name string, r io.Reader, limit int64) (int64, error) {
	info, err := s.client.PutObject(ctx, s.bucket, s.key(name), io.LimitReader(r, limit), -1, minio.PutObjectOptions{
		ContentType: "application/zstd",
	})
	return info.Size, err
}

func (s *s3Storage) Size(ctx context.Context, name string) (int64, error) {
	info, err := s.client.StatObject(ctx, s.bucket, s.key(name), minio.StatObjectOptions{})
	if minio.ToErrorResponse(err).StatusCode == http.StatusNotFound {
		return 0, ErrObjectNotFound
	}
	if err != nil {
		return 0, err
	}
	return info.Size, nil
}

func (s *s3Storage) Delete(ctx context.Context, name string) error {
	return s.client.RemoveObject(ctx, s.bucket, s.key(name), minio.RemoveObjectOptions{})
}

// ErrPrivateEndpoint is returned when a user's bucket endpoint resolves to an
// address that is not public.
var ErrPrivateEndpoint = errors.New("the endpoint is not a public address")

// cgnat is the shared address space of carrier-grade NAT, 100.64.0.0/10,
// which also holds VPN and tailnet addresses.
var _, cgnat, _ = net.ParseCIDR("100.64.0.0/10")

// bucketDialTimeout bounds connecting to, and keeps alive connections with, a
// user's bucket.
const bucketDialTimeout = 30 * time.Second

// isPublicIP reports whether ip is routable on the internet.
func isPublicIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !cgnat.Contains(ip)
}

// publicOnlyTransport dials only public addresses. The check runs on the
// address actually dialed, after name resolution, so a name that resolves to
// an internal address, now or after a rebinding, is refused. It ignores
// proxies from the environment, which would hide the dialed address.
func publicOnlyTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   bucketDialTimeout,
		KeepAlive: bucketDialTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if ip := net.ParseIP(host); ip == nil || !isPublicIP(ip) {
				return fmt.Errorf("%w: %s", ErrPrivateEndpoint, host)
			}
			return nil
		},
	}
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		transport = &http.Transport{}
	}
	transport = transport.Clone()
	transport.Proxy = nil
	transport.DialContext = dialer.DialContext
	return transport
}

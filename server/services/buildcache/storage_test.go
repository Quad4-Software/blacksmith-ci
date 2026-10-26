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
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPublicIP(t *testing.T) {
	for addr, public := range map[string]bool{
		"8.8.8.8":         true,
		"2a00:1450::1":    true,
		"127.0.0.1":       false,
		"::1":             false,
		"10.1.2.3":        false,
		"172.16.0.1":      false,
		"192.168.1.1":     false,
		"169.254.169.254": false,
		"100.100.1.1":     false,
		"fd00::1":         false,
		"fe80::1":         false,
		"0.0.0.0":         false,
	} {
		assert.Equal(t, public, isPublicIP(net.ParseIP(addr)), addr)
	}
}

func TestPublicOnlyRefusesInternalEndpoints(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	endpoint := strings.TrimPrefix(srv.URL, "http://")

	_, err := NewS3Storage(context.Background(), S3Options{Endpoint: endpoint, Bucket: "cache", PublicOnly: true})
	require.ErrorIs(t, err, ErrPrivateEndpoint)
	assert.False(t, called, "the request never reaches the internal endpoint")

	_, err = NewS3Storage(context.Background(), S3Options{Endpoint: endpoint, Bucket: "cache"})
	assert.NotErrorIs(t, err, ErrPrivateEndpoint, "the server's own storage may be internal")
	assert.True(t, called)
}

func TestOpenS3StorageDoesNotConnect(t *testing.T) {
	// Nothing listens on port 1, so only Check notices.
	storage, err := OpenS3Storage(S3Options{Endpoint: "127.0.0.1:1", Bucket: "cache"})
	require.NoError(t, err)
	checker, ok := storage.(Checker)
	require.True(t, ok)
	require.Error(t, checker.Check(context.Background()))

	_, err = NewS3Storage(context.Background(), S3Options{Endpoint: "127.0.0.1:1", Bucket: "cache"})
	require.Error(t, err, "NewS3Storage still requires a reachable bucket")
}

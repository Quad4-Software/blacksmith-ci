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

package cache

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/common"
	"codefloe.com/crowci/crow/v6/crow-go/crow"
	crow_mocks "codefloe.com/crowci/crow/v6/crow-go/crow/mocks"
)

func TestCacheOutput(t *testing.T) {
	lastUsed := time.Now().Add(-2 * time.Hour).Unix()

	tests := []struct {
		name        string
		args        []string
		disabled    bool
		expected    string
		expectedErr string
		wantErr     bool
	}{
		{
			name:     "table output with default columns",
			args:     []string{},
			expected: "ID  KEY        SCOPE           SIZE     LAST USED    PENDING\n1   go-abc123  main            1.5 MiB  2 hours ago  no\n2   node-def   refs/tags/v1.0  512 B    -            yes\n",
		},
		{
			name:     "table output with custom columns",
			args:     []string{"ls", "--output", "table=Key,Scope"},
			expected: "KEY        SCOPE\ngo-abc123  main\nnode-def   refs/tags/v1.0\n",
		},
		{
			name:     "table output with no header",
			args:     []string{"ls", "--output-no-headers", "--output", "table=ID,Pending"},
			expected: "1  no\n2  yes\n",
		},
		{
			name:     "go-template output",
			args:     []string{"ls", "--output", "go-template={{ .Enabled }} {{ .Size }}{{ range .Entries }} {{ .Scope }}{{ end }}"},
			expected: "true 1573376 refs/heads/main refs/tags/v1.0\n",
		},
		{
			name:    "go-template output without template",
			args:    []string{"ls", "--output", "go-template"},
			wantErr: true,
		},
		{
			name:        "disabled caching prints a note",
			args:        []string{"ls", "--output", "table=ID"},
			disabled:    true,
			expected:    "ID\n1\n2\n",
			expectedErr: "Note: build caching is disabled on the server\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caches := &crow.RepoCaches{
				Enabled: !tt.disabled,
				Size:    1573376,
				Entries: []*crow.BuildCache{
					{ID: 1, Key: "go-abc123", Scope: "refs/heads/main", Size: 1572864, Committed: true, LastUsed: lastUsed},
					{ID: 2, Key: "node-def", Scope: "refs/tags/v1.0", Size: 512},
				},
			}

			command := &cli.Command{
				Writer: io.Discard,
				Name:   "ls",
				Flags:  common.OutputFlags("table"),
				Action: func(_ context.Context, c *cli.Command) error {
					var out, errOut bytes.Buffer
					err := cacheOutput(c, caches, &out, &errOut)

					if tt.wantErr {
						assert.Error(t, err)
						return nil
					}

					assert.NoError(t, err)
					assert.Equal(t, tt.expected, out.String())
					assert.Equal(t, tt.expectedErr, errOut.String())

					return nil
				},
			}

			_ = command.Run(t.Context(), tt.args)
		})
	}
}

func TestCacheRemove(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		deleteErr error
		wantCall  bool
		wantErr   string
	}{
		{
			name:     "success with repo full name",
			args:     []string{"rm", "org/repo", "7"},
			wantCall: true,
		},
		{
			name:     "success with repo id",
			args:     []string{"rm", "1", "7"},
			wantCall: true,
		},
		{
			name:    "missing cache id",
			args:    []string{"rm", "org/repo"},
			wantErr: "expected arguments <repo-id|repo-full-name> <cache-id>",
		},
		{
			name:    "invalid cache id",
			args:    []string{"rm", "org/repo", "abc"},
			wantErr: "invalid cache id 'abc': strconv.ParseInt: parsing \"abc\": invalid syntax",
		},
		{
			name:      "api error",
			args:      []string{"rm", "org/repo", "7"},
			deleteErr: errors.New("api error"),
			wantCall:  true,
			wantErr:   "api error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := crow_mocks.NewMockClient(t)
			mockClient.On("RepoLookup", "org/repo").Maybe().Return(&crow.Repo{ID: 1}, nil)
			if tt.wantCall {
				mockClient.On("RepoCacheDelete", int64(1), int64(7)).Return(tt.deleteErr).Once()
			}

			command := &cli.Command{
				Writer: io.Discard,
				Name:   "rm",
				Action: func(_ context.Context, c *cli.Command) error {
					return cacheRemoveWithClient(c, mockClient)
				},
			}

			err := command.Run(t.Context(), tt.args)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestCachePurge(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		purgeErr error
		wantCall bool
		wantErr  string
	}{
		{
			name:     "success",
			args:     []string{"purge", "org/repo"},
			wantCall: true,
		},
		{
			name:    "missing repo",
			args:    []string{"purge"},
			wantErr: "missing required argument repo-id / repo-full-name",
		},
		{
			name:     "api error",
			args:     []string{"purge", "org/repo"},
			purgeErr: errors.New("api error"),
			wantCall: true,
			wantErr:  "api error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := crow_mocks.NewMockClient(t)
			mockClient.On("RepoLookup", mock.Anything).Maybe().Return(&crow.Repo{ID: 1}, nil)
			if tt.wantCall {
				mockClient.On("RepoCachePurge", int64(1)).Return(tt.purgeErr).Once()
			}

			command := &cli.Command{
				Writer: io.Discard,
				Name:   "purge",
				Action: func(_ context.Context, c *cli.Command) error {
					return cachePurgeWithClient(c, mockClient)
				},
			}

			err := command.Run(t.Context(), tt.args)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

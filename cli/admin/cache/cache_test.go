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

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/common"
	"codefloe.com/crowci/crow/v6/crow-go/crow"
	crow_mocks "codefloe.com/crowci/crow/v6/crow-go/crow/mocks"
)

const gib = 1 << 30

func TestParseSizeLimit(t *testing.T) {
	tests := []struct {
		value   string
		want    int64
		wantErr bool
	}{
		{value: "default", want: 0},
		{value: "Unlimited", want: -1},
		{value: "none", want: -1},
		{value: "10GiB", want: 10 * gib},
		{value: "500MB", want: 500_000_000},
		{value: "0", wantErr: true},
		{value: "-1", wantErr: true},
		{value: "lots", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := parseSizeLimit(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseTTL(t *testing.T) {
	tests := []struct {
		value   string
		want    int64
		wantErr bool
	}{
		{value: "default", want: 0},
		{value: "unlimited", want: -1},
		{value: "7d", want: 7 * 86400},
		{value: "2w", want: 14 * 86400},
		{value: "36h", want: 36 * 3600},
		{value: "90m", want: 5400},
		{value: "0d", wantErr: true},
		{value: "500ms", wantErr: true},
		{value: "-1d", wantErr: true},
		{value: "xd", wantErr: true},
		{value: "soon", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := parseTTL(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func testOrg() *crow.BuildCacheOrg {
	return &crow.BuildCacheOrg{
		BuildCacheOrgSummary: crow.BuildCacheOrgSummary{
			OrgID:       5,
			Name:        "octocat",
			IsUser:      true,
			Size:        3 * gib,
			Entries:     12,
			Repos:       2,
			EgressMonth: gib,
			Usage:       crow.BuildCacheUsage{Hits: 6, PartialHits: 2, Misses: 1, Denied: 1},
			Settings:    crow.BuildCacheOrgLimits{MaxSize: 20 * gib, MaxEgress: 0, TTL: -1},
			Limits:      crow.BuildCacheLimits{MaxSize: 20 * gib, MaxEgress: 50 * gib, TTL: 0},
		},
		RepoList: []*crow.BuildCacheRepoSummary{
			{RepoID: 1, FullName: "octocat/hello", Size: 2 * gib, Entries: 8, Usage: crow.BuildCacheUsage{Hits: 3, Misses: 1, Egress: gib}},
		},
	}
}

func TestCacheListOutput(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		disabled    bool
		expected    string
		expectedErr string
	}{
		{
			name: "table output with default columns",
			args: []string{"ls"},
			expected: "ID  NAME     TYPE  SIZE     SIZE LIMIT  EGRESS   EGRESS LIMIT         RETENTION  HIT RATE  BUCKET\n" +
				"5   octocat  user  3.0 GiB  20 GiB      1.0 GiB  50 GiB (default)     unlimited  80%       no\n" +
				"9   acme     org   0 B      unlimited   0 B      unlimited (default)  7d         -         yes\n",
		},
		{
			name:     "table output with custom columns",
			args:     []string{"ls", "--output", "table=Name,Size_Limit"},
			expected: "NAME     SIZE LIMIT\noctocat  20 GiB\nacme     unlimited\n",
		},
		{
			name:     "go-template output",
			args:     []string{"ls", "--output", "go-template={{ range .Overview.Orgs }}{{ .Name }} {{ end }}"},
			expected: "octocat acme \n",
		},
		{
			name:        "disabled caching prints a note",
			args:        []string{"ls"},
			disabled:    true,
			expected:    "ID  NAME  TYPE  SIZE  SIZE LIMIT  EGRESS  EGRESS LIMIT  RETENTION  HIT RATE  BUCKET\n",
			expectedErr: "Note: build caching is disabled on the server\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caches := &crow.AdminCaches{}
			if !tt.disabled {
				caches.Enabled = true
				caches.Overview = &crow.BuildCacheOverview{Orgs: []*crow.BuildCacheOrgSummary{
					&testOrg().BuildCacheOrgSummary,
					{
						OrgID:    9,
						Name:     "acme",
						Settings: crow.BuildCacheOrgLimits{MaxSize: -1, TTL: 7 * 86400},
						Limits:   crow.BuildCacheLimits{TTL: 7 * 86400},
						Bucket:   &crow.BuildCacheBucket{Endpoint: "s3.example.com", Bucket: "caches"},
					},
				}}
			}

			var out, errOut bytes.Buffer
			command := &cli.Command{
				Writer: io.Discard,
				Name:   "ls",
				Flags:  common.OutputFlags("table"),
				Action: func(_ context.Context, c *cli.Command) error {
					return cacheListOutput(c, caches, &out, &errOut)
				},
			}

			assert.NoError(t, command.Run(t.Context(), tt.args))
			assert.Equal(t, tt.expected, out.String())
			assert.Equal(t, tt.expectedErr, errOut.String())
		})
	}
}

func TestCacheShow(t *testing.T) {
	mockClient := crow_mocks.NewMockClient(t)
	mockClient.On("OrgLookup", "octocat").Return(&crow.Org{ID: 5}, nil).Once()
	mockClient.On("AdminCacheOrg", int64(5)).Return(testOrg(), nil).Once()

	var out bytes.Buffer
	command := &cli.Command{
		Writer: &out,
		Name:   "show",
		Flags:  common.OutputFlags("table"),
		Action: func(_ context.Context, c *cli.Command) error {
			return cacheShowWithClient(c, mockClient)
		},
	}

	assert.NoError(t, command.Run(t.Context(), []string{"show", "octocat"}))
	assert.Equal(t, `Name:          octocat (user)
ID:            5
Size:          3.0 GiB in 12 entries across 2 repositories
Size limit:    20 GiB
Egress:        1.0 GiB this month
Egress limit:  50 GiB (default)
Retention:     unlimited
Hit rate:      80% over 30 days

REPO           SIZE     ENTRIES  HIT RATE  EGRESS 30D
octocat/hello  2.0 GiB  8        75%       1.0 GiB
`, out.String())
}

func TestCacheSet(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    *crow.BuildCacheOrgLimits
		wantErr string
	}{
		{
			name: "changes only the given limits",
			args: []string{"set", "5", "--max-egress", "100GiB", "--ttl", "default"},
			want: &crow.BuildCacheOrgLimits{MaxSize: 20 * gib, MaxEgress: 100 * gib, TTL: 0},
		},
		{
			name: "lifts a limit",
			args: []string{"set", "5", "--max-size", "unlimited"},
			want: &crow.BuildCacheOrgLimits{MaxSize: -1, MaxEgress: 0, TTL: -1},
		},
		{
			name:    "requires a limit",
			args:    []string{"set", "5"},
			wantErr: "nothing to set: pass --max-size, --max-egress or --ttl",
		},
		{
			name:    "requires an owner",
			args:    []string{"set", "--ttl", "7d"},
			wantErr: errMissingOrg.Error(),
		},
		{
			name:    "rejects an invalid size",
			args:    []string{"set", "5", "--max-size", "0"},
			wantErr: `invalid size "0": must be positive; use "default" for the server default`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := crow_mocks.NewMockClient(t)
			mockClient.On("AdminCacheOrg", int64(5)).Maybe().Return(testOrg(), nil)
			if tt.want != nil {
				mockClient.On("AdminCacheOrgUpdate", int64(5), tt.want).Return(testOrg(), nil).Once()
			}

			command := &cli.Command{
				Writer: io.Discard,
				Name:   "set",
				Flags:  cacheSetFlags(),
				Action: func(_ context.Context, c *cli.Command) error {
					return cacheSetWithClient(c, mockClient)
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
		expected string
		wantErr  string
	}{
		{
			name:     "success",
			args:     []string{"purge", "5"},
			expected: "Removed 3 build caches (2.0 GiB) of 5\n",
		},
		{
			name:    "missing owner",
			args:    []string{"purge"},
			wantErr: errMissingOrg.Error(),
		},
		{
			name:     "api error",
			args:     []string{"purge", "5"},
			purgeErr: errors.New("api error"),
			wantErr:  "api error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := crow_mocks.NewMockClient(t)
			mockClient.On("AdminCacheOrgPurge", int64(5)).Maybe().Return(&crow.BuildCacheCleanup{Removed: 3, Freed: 2 * gib}, tt.purgeErr)

			var out bytes.Buffer
			command := &cli.Command{
				Writer: &out,
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
			assert.Equal(t, tt.expected, out.String())
		})
	}
}

func TestCacheCleanup(t *testing.T) {
	mockClient := crow_mocks.NewMockClient(t)
	mockClient.On("AdminCacheCleanup").Return(&crow.BuildCacheCleanup{Removed: 4, Freed: 1024}, nil).Once()

	var out bytes.Buffer
	command := &cli.Command{
		Writer: &out,
		Name:   "cleanup",
		Action: func(_ context.Context, c *cli.Command) error {
			return cacheCleanupWithClient(c, mockClient)
		},
	}

	assert.NoError(t, command.Run(t.Context(), []string{"cleanup"}))
	assert.Equal(t, "Removed 4 build caches (1.0 KiB)\n", out.String())
}

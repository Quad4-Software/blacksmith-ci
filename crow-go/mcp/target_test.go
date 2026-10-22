// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/crow-go/crow"
	crow_mocks "codefloe.com/crowci/crow/v6/crow-go/crow/mocks"
)

var testInstance = Instance{
	Name:       "codefloe",
	URL:        "https://ci.codefloe.com",
	ForgeHosts: []string{"codefloe.com"},
	TokenEnv:   "X",
}

func newTestRegistry(t *testing.T, client crow.Client) *Registry {
	t.Helper()
	reg, err := NewRegistry(&Config{Instances: []Instance{testInstance}}, func(string, TokenFunc) crow.Client {
		return client
	})
	require.NoError(t, err)
	return reg
}

type fakeDiscoverer struct {
	co  *Checkout
	err error
}

func (f fakeDiscoverer) Discover(context.Context) (*Checkout, error) { return f.co, f.err }

func TestParseWebURL(t *testing.T) {
	tests := []struct {
		raw     string
		want    WebURL
		wantErr bool
	}{
		{raw: "https://ci.x/repos/34/pipeline/12", want: WebURL{Host: "ci.x", RepoID: 34, Number: 12}},
		{raw: "https://ci.x/repos/34/pipeline/12/5", want: WebURL{Host: "ci.x", RepoID: 34, Number: 12, StepPID: 5}},
		{raw: "https://h/ci/repos/3/pipeline/9/2/", want: WebURL{Host: "h", Root: "/ci", RepoID: 3, Number: 9, StepPID: 2}},
		{raw: "https://ci.x/repos/34/pipeline/12/5?foo=1#x", want: WebURL{Host: "ci.x", RepoID: 34, Number: 12, StepPID: 5}},
		{raw: "https://ci.x/repos/34/pipeline/12/graph", want: WebURL{Host: "ci.x", RepoID: 34, Number: 12}},
		{raw: "https://ci.x:8443/repos/34/pipeline/12", want: WebURL{Host: "ci.x", RepoID: 34, Number: 12}},
		{raw: "https://ci.x/repos/34", wantErr: true},
		{raw: "https://ci.x/repos/abc/pipeline/1", wantErr: true},
		{raw: "not a url", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := ParseWebURL(tt.raw)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveURLUnknownHost(t *testing.T) {
	client := crow_mocks.NewMockClient(t)
	reg := newTestRegistry(t, client)
	_, err := reg.Resolve(t.Context(), TargetInput{URL: "https://evil.example/repos/1/pipeline/2"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "evil.example")
}

func TestResolveURL(t *testing.T) {
	client := crow_mocks.NewMockClient(t)
	client.EXPECT().Repo(int64(34)).Return(&crow.Repo{ID: 34, FullName: "crowci/crow"}, nil)
	client.EXPECT().Pipeline(int64(34), int64(12)).Return(&crow.Pipeline{Number: 12}, nil)
	reg := newTestRegistry(t, client)

	res, err := reg.Resolve(t.Context(), TargetInput{URL: "https://ci.codefloe.com/repos/34/pipeline/12/5"}, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(34), res.Repo.ID)
	require.Len(t, res.Pipelines, 1)
	assert.Equal(t, int64(12), res.Pipelines[0].Number)
	assert.Equal(t, 5, res.StepPID)
}

func TestResolveExplicitNumericRepo(t *testing.T) {
	client := crow_mocks.NewMockClient(t)
	client.EXPECT().Repo(int64(34)).Return(&crow.Repo{ID: 34}, nil)
	client.EXPECT().Pipeline(int64(34), int64(7)).Return(&crow.Pipeline{Number: 7}, nil)
	reg := newTestRegistry(t, client)

	res, err := reg.Resolve(t.Context(), TargetInput{Instance: "codefloe", Repo: "34", Number: 7}, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(7), res.Pipelines[0].Number)
}

func TestResolveExplicitRepoWithoutNumberUsesLatest(t *testing.T) {
	client := crow_mocks.NewMockClient(t)
	client.EXPECT().RepoLookup("a/b").Return(&crow.Repo{ID: 3}, nil)
	client.EXPECT().PipelineLast(int64(3), crow.PipelineLastOptions{}).Return(&crow.Pipeline{Number: 99}, nil)
	reg := newTestRegistry(t, client)

	res, err := reg.Resolve(t.Context(), TargetInput{Instance: "codefloe", Repo: "a/b"}, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(99), res.Pipelines[0].Number)
}

func TestResolveNoTargetWithoutDiscoverer(t *testing.T) {
	reg := newTestRegistry(t, crow_mocks.NewMockClient(t))
	_, err := reg.Resolve(t.Context(), TargetInput{}, nil)
	require.Error(t, err)
}

func TestResolveDiscoveryMatchesHEADReturnsPushAndPR(t *testing.T) {
	client := crow_mocks.NewMockClient(t)
	client.EXPECT().RepoLookup("a/b").Return(&crow.Repo{ID: 3}, nil)
	client.EXPECT().PipelineList(int64(3), mock.Anything).Return([]*crow.Pipeline{
		{Number: 12, Commit: "abc123456789", Event: "push", Status: "success"},
		{Number: 11, Commit: "abc123456789", Event: "pull_request", Status: "failure"},
		{Number: 10, Commit: "fff", Event: "push", Status: "failure"},
	}, nil)
	reg := newTestRegistry(t, client)

	res, err := reg.Resolve(t.Context(), TargetInput{}, fakeDiscoverer{co: &Checkout{
		Remotes: []Remote{{Name: "origin", Host: "codefloe.com", FullName: "a/b"}},
		Branch:  "feat",
		Commit:  "abc123456789",
	}})
	require.NoError(t, err)
	require.Len(t, res.Pipelines, 2)
	assert.Equal(t, int64(11), res.Pipelines[0].Number, "failed pipeline first")
	assert.Equal(t, int64(12), res.Pipelines[1].Number)
	assert.Contains(t, res.Note, "matched HEAD abc1234")
}

func TestResolveDiscoveryFallsBackToBranch(t *testing.T) {
	client := crow_mocks.NewMockClient(t)
	client.EXPECT().RepoLookup("a/b").Return(&crow.Repo{ID: 3}, nil)
	client.EXPECT().PipelineList(int64(3), mock.Anything).Return([]*crow.Pipeline{
		{Number: 9, Commit: "old2", Event: "pull_request", Branch: "main", Refspec: "feat:main", Status: "failure"},
		{Number: 8, Commit: "old1", Event: "push", Branch: "feat", Status: "success"},
		{Number: 7, Commit: "old0", Event: "push", Branch: "feat", Status: "success"},
		{Number: 6, Commit: "x", Event: "push", Branch: "main", Status: "success"},
	}, nil)
	reg := newTestRegistry(t, client)

	res, err := reg.Resolve(t.Context(), TargetInput{}, fakeDiscoverer{co: &Checkout{
		Remotes: []Remote{{Name: "origin", Host: "codefloe.com", FullName: "a/b"}},
		Branch:  "feat",
		Commit:  "new123456789",
	}})
	require.NoError(t, err)
	require.Len(t, res.Pipelines, 2)
	assert.Equal(t, int64(9), res.Pipelines[0].Number)
	assert.Equal(t, int64(8), res.Pipelines[1].Number)
	assert.Contains(t, res.Note, "no pipeline for HEAD new1234")
	assert.Contains(t, res.Note, "feat")
}

func TestResolveDiscoveryPrefersUpstreamRemote(t *testing.T) {
	client := crow_mocks.NewMockClient(t)
	client.EXPECT().RepoLookup("up/b").Return(&crow.Repo{ID: 5}, nil)
	client.EXPECT().PipelineList(int64(5), mock.Anything).Return([]*crow.Pipeline{{Number: 1, Commit: "c"}}, nil)
	reg := newTestRegistry(t, client)

	res, err := reg.Resolve(t.Context(), TargetInput{}, fakeDiscoverer{co: &Checkout{
		Remotes: []Remote{
			{Name: "origin", Host: "codefloe.com", FullName: "fork/b"},
			{Name: "upstream", Host: "codefloe.com", FullName: "up/b"},
		},
		Commit: "c",
	}})
	require.NoError(t, err)
	assert.Equal(t, int64(5), res.Repo.ID)
}

func TestResolveDiscoverySkipsRemoteWithoutRepo(t *testing.T) {
	client := crow_mocks.NewMockClient(t)
	client.EXPECT().RepoLookup("up/b").Return(nil, errors.New("not found"))
	client.EXPECT().RepoList(crow.RepoListOptions{}).Return(nil, nil)
	client.EXPECT().RepoLookup("fork/b").Return(&crow.Repo{ID: 6}, nil)
	client.EXPECT().PipelineList(int64(6), mock.Anything).Return([]*crow.Pipeline{{Number: 1, Commit: "c"}}, nil)
	reg := newTestRegistry(t, client)

	res, err := reg.Resolve(t.Context(), TargetInput{}, fakeDiscoverer{co: &Checkout{
		Remotes: []Remote{
			{Name: "origin", Host: "codefloe.com", FullName: "fork/b"},
			{Name: "upstream", Host: "codefloe.com", FullName: "up/b"},
		},
		Commit: "c",
	}})
	require.NoError(t, err)
	assert.Equal(t, int64(6), res.Repo.ID)
}

func TestResolveNoMatchingInstanceListsForgeHosts(t *testing.T) {
	reg := newTestRegistry(t, crow_mocks.NewMockClient(t))
	_, err := reg.Resolve(t.Context(), TargetInput{}, fakeDiscoverer{co: &Checkout{
		Remotes: []Remote{{Name: "origin", Host: "github.com", FullName: "a/b"}},
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github.com")
	assert.Contains(t, err.Error(), "codefloe.com")
}

func TestWebLinkWithRoot(t *testing.T) {
	assert.Equal(t, "https://ci.codefloe.com/repos/3/pipeline/9", WebLink(testInstance, "", 3, 9, 0))
	assert.Equal(t, "https://h/ci/repos/3/pipeline/9/2", WebLink(Instance{URL: "https://h/ci/"}, "", 3, 9, 2))
	assert.Equal(t, "https://h/ci/repos/3/pipeline/9", WebLink(Instance{URL: "https://h"}, "/ci", 3, 9, 0))
}

func TestResolveDiscoveryNothingFoundReturnsEmpty(t *testing.T) {
	client := crow_mocks.NewMockClient(t)
	client.EXPECT().RepoLookup("a/b").Return(&crow.Repo{ID: 3}, nil)
	client.EXPECT().PipelineList(int64(3), mock.Anything).Return(nil, nil)
	reg := newTestRegistry(t, client)

	res, err := reg.Resolve(t.Context(), TargetInput{}, fakeDiscoverer{co: &Checkout{
		Remotes: []Remote{{Name: "origin", Host: "codefloe.com", FullName: "a/b"}},
		Branch:  "feat",
		Commit:  "new",
	}})
	require.NoError(t, err)
	assert.Empty(t, res.Pipelines)
	assert.False(t, res.MatchedHead)
}

func TestResolveDiscoveryAvoidsStaleDuplicateRepo(t *testing.T) {
	client := crow_mocks.NewMockClient(t)
	client.EXPECT().RepoLookup("crowci/crow").Return(&crow.Repo{ID: 1, FullName: "crowci/crow", ForgeURL: "https://codeberg.org/crowci/crow"}, nil)
	client.EXPECT().RepoList(crow.RepoListOptions{}).Return([]*crow.Repo{
		{ID: 2, FullName: "other/x", ForgeURL: "https://codefloe.com/other/x", IsActive: true},
		{ID: 34, FullName: "crowci/crow", ForgeURL: "https://codefloe.com/crowci/crow", IsActive: true},
	}, nil)
	client.EXPECT().PipelineList(int64(34), mock.Anything).Return([]*crow.Pipeline{{Number: 1, Commit: "c"}}, nil)
	reg := newTestRegistry(t, client)

	res, err := reg.Resolve(t.Context(), TargetInput{}, fakeDiscoverer{co: &Checkout{
		Remotes: []Remote{{Name: "origin", Host: "codefloe.com", FullName: "crowci/crow"}},
		Commit:  "c",
	}})
	require.NoError(t, err)
	assert.Equal(t, int64(34), res.Repo.ID)
}

func TestResolveDiscoveryStaleDuplicateWithoutListScope(t *testing.T) {
	client := crow_mocks.NewMockClient(t)
	client.EXPECT().RepoLookup("crowci/crow").Return(&crow.Repo{ID: 1, FullName: "crowci/crow", ForgeURL: "https://codeberg.org/crowci/crow"}, nil)
	client.EXPECT().RepoList(crow.RepoListOptions{}).Return(nil, errors.New("403 user:read required"))
	reg := newTestRegistry(t, client)

	_, err := reg.Resolve(t.Context(), TargetInput{}, fakeDiscoverer{co: &Checkout{
		Remotes: []Remote{{Name: "origin", Host: "codefloe.com", FullName: "crowci/crow"}},
		Commit:  "c",
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "codeberg.org")
	assert.Contains(t, err.Error(), "pass url")
}

func TestResolveDiscoveryReportsPreferredRemoteError(t *testing.T) {
	client := crow_mocks.NewMockClient(t)
	client.EXPECT().RepoLookup("crowci/crow").Return(nil, errors.New("origin-failure"))
	client.EXPECT().RepoLookup("someone/crow").Return(nil, errors.New("fork-failure"))
	client.EXPECT().RepoList(crow.RepoListOptions{}).Return(nil, nil)
	reg := newTestRegistry(t, client)

	_, err := reg.Resolve(t.Context(), TargetInput{}, fakeDiscoverer{co: &Checkout{
		Remotes: []Remote{
			{Name: "origin", Host: "codefloe.com", FullName: "crowci/crow"},
			{Name: "zfork", Host: "codefloe.com", FullName: "someone/crow"},
		},
		Commit: "c",
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "origin-failure")
}

func TestResolveDiscoveryKeepsNewestPerEvent(t *testing.T) {
	client := crow_mocks.NewMockClient(t)
	client.EXPECT().RepoLookup("a/b").Return(&crow.Repo{ID: 3}, nil)
	client.EXPECT().PipelineList(int64(3), mock.Anything).Return([]*crow.Pipeline{
		{Number: 13, Commit: "c", Event: "push", Status: "running"},
		{Number: 12, Commit: "c", Event: "push", Status: "failure"},
		{Number: 11, Commit: "c", Event: "pull_request", Status: "failure"},
	}, nil)
	reg := newTestRegistry(t, client)

	res, err := reg.Resolve(t.Context(), TargetInput{}, fakeDiscoverer{co: &Checkout{
		Remotes: []Remote{{Name: "origin", Host: "codefloe.com", FullName: "a/b"}},
		Commit:  "c",
	}})
	require.NoError(t, err)
	var numbers []int64
	for _, p := range res.Pipelines {
		numbers = append(numbers, p.Number)
	}
	assert.Equal(t, []int64{11, 13}, numbers)
}

func TestResolveDiscoveryDetachedHeadHasNoBranchFallback(t *testing.T) {
	client := crow_mocks.NewMockClient(t)
	client.EXPECT().RepoLookup("a/b").Return(&crow.Repo{ID: 3}, nil)
	client.EXPECT().PipelineList(int64(3), mock.Anything).Return([]*crow.Pipeline{
		{Number: 9, Commit: "tagcommit", Event: "tag", Ref: "refs/tags/v1"},
	}, nil)
	reg := newTestRegistry(t, client)

	res, err := reg.Resolve(t.Context(), TargetInput{}, fakeDiscoverer{co: &Checkout{
		Remotes: []Remote{{Name: "origin", Host: "codefloe.com", FullName: "a/b"}},
		Commit:  "detached",
	}})
	require.NoError(t, err)
	assert.Empty(t, res.Pipelines)
	assert.NotContains(t, res.Note, "branch ")
}

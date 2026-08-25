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

package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server/forge"
	forge_mocks "codefloe.com/crowci/crow/v6/server/forge/mocks"
	"codefloe.com/crowci/crow/v6/server/model"
)

// tagForgeStub is a forge that also implements TagForge, which MockForge on its
// own deliberately does not: tag support is optional per forge.
type tagForgeStub struct {
	*forge_mocks.MockForge

	// gotTag records the tag name TagHead was called with, so tests can assert
	// the refs/tags/ prefix is stripped before it reaches the forge.
	gotTag string
	commit *model.Commit
	err    error
}

var _ forge.TagForge = new(tagForgeStub)

func (s *tagForgeStub) Tags(context.Context, *model.User, *model.Repo, *model.ListOptions) ([]string, error) {
	return []string{"v1.0.0"}, nil
}

func (s *tagForgeStub) TagHead(_ context.Context, _ *model.User, _ *model.Repo, tag string) (*model.Commit, error) {
	s.gotTag = tag
	return s.commit, s.err
}

func testGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

func statusOf(t *testing.T, err error) int {
	t.Helper()
	var statusErr huma.StatusError
	require.ErrorAs(t, err, &statusErr)
	return statusErr.GetStatus()
}

func TestResolveManualRefBranch(t *testing.T) {
	repo := &model.Repo{Owner: "crowci", Name: "crow", FullName: "crowci/crow"}
	user := &model.User{Login: "pat-s"}
	commit := &model.Commit{SHA: "deadbeef", Message: "a commit", ForgeURL: "https://example.com/c"}

	mockForge := forge_mocks.NewMockForge(t)
	mockForge.On("BranchHead", mock.Anything, user, repo, "main").Return(commit, nil)

	target, err := resolveManualRef(testGinContext(), mockForge, user, repo, "main", "")
	require.NoError(t, err)

	assert.Equal(t, commit, target.Commit)
	assert.Equal(t, "main", target.Branch)
	// Branch runs keep the bare branch name as the ref, as they always have.
	assert.Equal(t, "main", target.Ref)
}

func TestResolveManualRefTag(t *testing.T) {
	repo := &model.Repo{Owner: "crowci", Name: "crow", FullName: "crowci/crow"}
	user := &model.User{Login: "pat-s"}
	commit := &model.Commit{SHA: "cafe1234", Message: "tagged commit"}

	tests := []struct {
		name string
		tag  string
	}{
		{name: "bare tag name", tag: "v1.0.0"},
		{name: "already prefixed", tag: "refs/tags/v1.0.0"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &tagForgeStub{MockForge: forge_mocks.NewMockForge(t), commit: commit}

			target, err := resolveManualRef(testGinContext(), stub, user, repo, "", test.tag)
			require.NoError(t, err)

			assert.Equal(t, "v1.0.0", stub.gotTag, "the forge must receive the bare tag name")
			assert.Equal(t, commit, target.Commit)
			assert.Equal(t, "refs/tags/v1.0.0", target.Ref)
			// A tag is not on a branch, so `when: branch` must not match it.
			assert.Empty(t, target.Branch)
		})
	}
}

func TestResolveManualRefRejectsAmbiguousInput(t *testing.T) {
	repo := &model.Repo{Owner: "crowci", Name: "crow"}
	user := &model.User{Login: "pat-s"}

	tests := []struct {
		name   string
		branch string
		tag    string
	}{
		{name: "neither"},
		{name: "both", branch: "main", tag: "v1.0.0"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// NewMockForge asserts no unexpected calls, so this also proves the
			// forge is never touched for invalid input.
			mockForge := forge_mocks.NewMockForge(t)

			_, err := resolveManualRef(testGinContext(), mockForge, user, repo, test.branch, test.tag)

			require.Error(t, err)
			assert.Equal(t, 400, statusOf(t, err))
		})
	}
}

func TestResolveManualRefTagOnForgeWithoutTagSupport(t *testing.T) {
	repo := &model.Repo{Owner: "crowci", Name: "crow"}
	user := &model.User{Login: "pat-s"}

	mockForge := forge_mocks.NewMockForge(t)

	_, err := resolveManualRef(testGinContext(), mockForge, user, repo, "", "v1.0.0")

	require.Error(t, err)
	assert.Equal(t, 501, statusOf(t, err))
}

func TestResolveManualRefPropagatesForgeFailure(t *testing.T) {
	repo := &model.Repo{Owner: "crowci", Name: "crow"}
	user := &model.User{Login: "pat-s"}

	stub := &tagForgeStub{MockForge: forge_mocks.NewMockForge(t), err: errors.New("tag not found")}

	_, err := resolveManualRef(testGinContext(), stub, user, repo, "", "nope")

	require.Error(t, err)
	assert.Equal(t, 500, statusOf(t, err))
}

func TestManualTmpPipelineCarriesResolvedRef(t *testing.T) {
	target := &manualRef{
		Commit: &model.Commit{SHA: "cafe1234"},
		Ref:    "refs/tags/v1.0.0",
	}

	tmp := manualTmpPipeline(target)

	assert.Equal(t, model.EventManual, tmp.Event)
	assert.Equal(t, "cafe1234", tmp.Commit)
	assert.Equal(t, "refs/tags/v1.0.0", tmp.Ref)
	assert.Empty(t, tmp.Branch)
}

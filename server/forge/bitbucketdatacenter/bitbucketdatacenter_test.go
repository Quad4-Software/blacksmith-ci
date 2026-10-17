// Copyright 2024 Woodpecker Authors
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

package bitbucketdatacenter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/forge/bitbucketdatacenter/fixtures"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestNew(t *testing.T) {
	forge, err := New(Opts{
		URL:          "http://localhost:8080",
		Username:     "0ZXh0IjoiI",
		Password:     "I1NiIsInR5",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	})
	assert.NoError(t, err)
	assert.NotNil(t, forge)
	cl, ok := forge.(*client)
	assert.True(t, ok)
	assert.Equal(t, &client{
		url:          "http://localhost:8080",
		urlAPI:       "http://localhost:8080/rest",
		username:     "0ZXh0IjoiI",
		password:     "I1NiIsInR5",
		clientID:     "client-id",
		clientSecret: "client-secret",
	}, cl)
}

func TestBitbucketDC(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := fixtures.Server()
	defer s.Close()
	c := &client{
		urlAPI: s.URL,
	}

	server.Config.Server.StatusContext = "ci/crow"
	server.Config.Server.StatusContextFormat = "{{ .context }}/{{ .event }}/{{ .workflow }}"

	ctx := t.Context()

	repo, err := c.Repo(ctx, fakeUser, model.ForgeRemoteID("1234"), "PRJ", "repo-slug")
	assert.NoError(t, err)
	assert.Equal(t, &model.Repo{
		Name:          "repo-slug-2",
		Owner:         "PRJ",
		Perm:          &model.Perm{Pull: true, Push: true},
		Branch:        "main",
		IsSCMPrivate:  true,
		PREnabled:     true,
		ForgeRemoteID: model.ForgeRemoteID("1234"),
		FullName:      "PRJ/repo-slug-2",
	}, repo)

	// org
	org, err := c.Org(ctx, fakeUser, "ORG")
	assert.NoError(t, err)
	assert.Equal(t, &model.Org{
		Name:   "ORG",
		IsUser: false,
	}, org)

	// user
	org, err = c.Org(ctx, fakeUser, "~ORG")
	assert.NoError(t, err)
	assert.Equal(t, &model.Org{
		Name:   "~ORG",
		IsUser: true,
	}, org)

	// Execute the Status method
	err = c.Status(ctx, fakeUser, fakeRepo, fakePipeline, fakeWorkflow)
	assert.NoError(t, err)
}

var (
	fakeUser = &model.User{
		AccessToken: "fake",
		Expiry:      time.Now().Add(1 * time.Hour).Unix(),
	}

	fakeRepo = &model.Repo{
		ID:     1,
		Owner:  "test-owner",
		Name:   "test-repo",
		Branch: "main",
	}

	fakePipeline = &model.Pipeline{
		ID:       1,
		Number:   42,
		Commit:   "3ce383490b3d90d79460c60f67ba2580acc6cc59",
		Started:  1759825800,
		Finished: 1759825883,
		Ref:      "refs/heads/feature-branch",
		Event:    model.EventPush,
	}

	fakeWorkflow = &model.Workflow{
		ID:    1,
		PID:   1,
		Name:  "build",
		State: model.StatusSuccess,
	}
)

func TestBitbucketDatacenterTeamMembershipsReturnsNil(t *testing.T) {
	t.Parallel()
	c := &client{}
	got, err := c.TeamMemberships(context.Background(), &model.User{})
	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestGetUserAndRepoUsesDriverForgeID(t *testing.T) {
	errNotFound := errors.New("not found")
	mockStore := store_mocks.NewMockStore(t)
	// The webhook repo comes from convertRepo, which leaves ForgeID unset.
	// The lookup must use the driver's forge ID instead.
	mockStore.On("GetRepoByForgeAndRemoteID", int64(7), model.ForgeRemoteID("42")).Return(nil, errNotFound).Once()

	c := &client{forgeID: 7}
	ctx := store.InjectToContext(t.Context(), mockStore)

	_, _, err := c.getUserAndRepo(ctx, &model.Repo{ForgeRemoteID: "42"})
	require.ErrorIs(t, err, errNotFound)
}

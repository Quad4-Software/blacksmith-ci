package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
	store_mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

// The active-repos overview must include repos the user only has read (pull)
// access to, not just ones they can push/admin. Otherwise read-only org
// members never see their org repos in the overview (issue #1225). At the
// store layer this means requesting owned=false.
func TestListActiveReposWithPipelinesIncludesReadOnly(t *testing.T) {
	user := &model.User{ID: 1, Login: "joe"}

	readOnlyRepo := &model.Repo{ID: 42, FullName: "org/repo", IsActive: true}

	mockStore := store_mocks.NewMockStore(t)
	mockStore.On("RepoList", user, false, true).Return([]*model.Repo{readOnlyRepo}, nil)
	mockStore.On("GetRepoLatestPipelines", []int64{readOnlyRepo.ID}).Return([]*model.Pipeline{}, nil)

	repos, err := listActiveReposWithPipelines(mockStore, user)
	assert.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, readOnlyRepo.ID, repos[0].ID)
}

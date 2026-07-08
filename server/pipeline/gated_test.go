package pipeline

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
	mocks "codefloe.com/crowci/crow/v6/server/store/mocks"
)

func TestSetRequireApprovalState(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		repo          *model.Repo
		pipeline      *model.Pipeline
		expectBlocked bool
	}{
		{
			name: "by-pass for cron",
			repo: &model.Repo{
				RequireApproval: model.RequireApprovalAllEvents,
			},
			pipeline: &model.Pipeline{
				Event: model.EventCron,
			},
			expectBlocked: false,
		},
		{
			name: "by-pass for manual pipeline",
			repo: &model.Repo{
				RequireApproval: model.RequireApprovalAllEvents,
			},
			pipeline: &model.Pipeline{
				Event: model.EventManual,
			},
			expectBlocked: false,
		},
		{
			name: "require approval for fork PRs",
			repo: &model.Repo{
				RequireApproval: model.RequireApprovalForks,
			},
			pipeline: &model.Pipeline{
				Event:    model.EventPull,
				FromFork: true,
			},
			expectBlocked: true,
		},
		{
			name: "require approval for PRs",
			repo: &model.Repo{
				RequireApproval: model.RequireApprovalPullRequests,
			},
			pipeline: &model.Pipeline{
				Event:    model.EventPull,
				FromFork: false,
			},
			expectBlocked: true,
		},
		{
			name: "require approval for edited PRs",
			repo: &model.Repo{
				RequireApproval: model.RequireApprovalPullRequests,
			},
			pipeline: &model.Pipeline{
				Event:    model.EventPullEdited,
				FromFork: false,
			},
			expectBlocked: true,
		},
		{
			name: "require approval for everything",
			repo: &model.Repo{
				RequireApproval: model.RequireApprovalAllEvents,
			},
			pipeline: &model.Pipeline{
				Event: model.EventPush,
			},
			expectBlocked: true,
		},
	}

	for _, tc := range testCases {
		mockStore := mocks.NewMockStore(t)
		if needsApproval(tc.repo, tc.pipeline) {
			mockStore.On("ApprovedPullRequestFind", tc.repo.ID, tc.pipeline.Ref).Return(nil, fmt.Errorf("not found"))
		}
		setApprovalState(mockStore, tc.repo, tc.pipeline)
		assert.Equal(t, tc.expectBlocked, tc.pipeline.Status == model.StatusBlocked)
	}
}

func TestSetRequireApprovalStateAlwaysApproved(t *testing.T) {
	t.Parallel()

	repo := &model.Repo{
		ID:              1,
		RequireApproval: model.RequireApprovalForks,
	}
	pl := &model.Pipeline{
		Event:    model.EventPull,
		FromFork: true,
		Ref:      "refs/pull/42/head",
	}

	mockStore := mocks.NewMockStore(t)
	mockStore.On("ApprovedPullRequestFind", repo.ID, pl.Ref).Return(&model.ApprovedPullRequest{
		ID:     1,
		RepoID: repo.ID,
		Ref:    pl.Ref,
	}, nil)

	setApprovalState(mockStore, repo, pl)
	assert.NotEqual(t, model.StatusBlocked, pl.Status, "pipeline should not be blocked when PR is always-approved")
}

func TestSetRequireApprovalStateAlwaysApprovedDifferentRef(t *testing.T) {
	t.Parallel()

	repo := &model.Repo{
		ID:              1,
		RequireApproval: model.RequireApprovalForks,
	}
	pl := &model.Pipeline{
		Event:    model.EventPull,
		FromFork: true,
		Ref:      "refs/pull/42/head",
	}

	mockStore := mocks.NewMockStore(t)
	// PR 42 is NOT always-approved, only PR 99 is
	mockStore.On("ApprovedPullRequestFind", repo.ID, "refs/pull/42/head").Return(nil, fmt.Errorf("not found"))

	setApprovalState(mockStore, repo, pl)
	assert.Equal(t, model.StatusBlocked, pl.Status, "pipeline should be blocked when a different PR is always-approved but not this one")
}

func TestSetRequireApprovalStateAlwaysApprovedAllEvents(t *testing.T) {
	t.Parallel()

	repo := &model.Repo{
		ID:              1,
		RequireApproval: model.RequireApprovalAllEvents,
	}
	pl := &model.Pipeline{
		Event: model.EventPush,
		Ref:   "refs/heads/main",
	}

	mockStore := mocks.NewMockStore(t)
	mockStore.On("ApprovedPullRequestFind", repo.ID, pl.Ref).Return(&model.ApprovedPullRequest{
		ID:     1,
		RepoID: repo.ID,
		Ref:    pl.Ref,
	}, nil)

	setApprovalState(mockStore, repo, pl)
	assert.NotEqual(t, model.StatusBlocked, pl.Status, "pipeline should not be blocked when ref is always-approved under RequireApprovalAllEvents")
}

func TestSetRequireApprovalStateStoreErrorFallsBackToBlocked(t *testing.T) {
	t.Parallel()

	repo := &model.Repo{
		ID:              1,
		RequireApproval: model.RequireApprovalForks,
	}
	pl := &model.Pipeline{
		Event:    model.EventPull,
		FromFork: true,
		Ref:      "refs/pull/42/head",
	}

	mockStore := mocks.NewMockStore(t)
	// Simulate a database error
	mockStore.On("ApprovedPullRequestFind", repo.ID, pl.Ref).Return(nil, fmt.Errorf("connection refused"))

	setApprovalState(mockStore, repo, pl)
	assert.Equal(t, model.StatusBlocked, pl.Status, "pipeline should still be blocked when store returns an error (safe default)")
}

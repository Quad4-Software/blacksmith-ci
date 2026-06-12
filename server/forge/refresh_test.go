// Copyright 2025 Crow CI Authors
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

package forge

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"codefloe.com/crowci/crow/v5/server/forge/types"
	"codefloe.com/crowci/crow/v5/server/model"
	mocks_store "codefloe.com/crowci/crow/v5/server/store/mocks"
)

// mockForgeWithRefresher implements both Forge and Refresher interfaces for testing.
type mockForgeWithRefresher struct {
	mock.Mock
}

func (m *mockForgeWithRefresher) Refresh(ctx context.Context, user *model.User) (bool, error) {
	args := m.Called(ctx, user)
	return args.Bool(0), args.Error(1)
}

// Implement minimal Forge interface methods (not used in refresh tests).
func (m *mockForgeWithRefresher) Name() string { return "mock" }
func (m *mockForgeWithRefresher) URL() string  { return "http://mock" }
func (m *mockForgeWithRefresher) Login(context.Context, *types.OAuthRequest) (*model.User, string, error) {
	return nil, "", nil
}

func (m *mockForgeWithRefresher) Auth(context.Context, string, string) (string, error) {
	return "", nil
}

func (m *mockForgeWithRefresher) Teams(context.Context, *model.User) ([]*model.Team, error) {
	return nil, nil
}

func (m *mockForgeWithRefresher) TeamMemberships(context.Context, *model.User) ([]*model.TeamMembership, error) {
	return nil, nil
}

func (m *mockForgeWithRefresher) Repo(context.Context, *model.User, model.ForgeRemoteID, string, string) (*model.Repo, error) {
	return nil, nil
}

func (m *mockForgeWithRefresher) Repos(context.Context, *model.User) ([]*model.Repo, error) {
	return nil, nil
}

func (m *mockForgeWithRefresher) File(context.Context, *model.User, *model.Repo, *model.Pipeline, string) ([]byte, error) {
	return nil, nil
}

func (m *mockForgeWithRefresher) Dir(context.Context, *model.User, *model.Repo, *model.Pipeline, string) ([]*types.FileMeta, error) {
	return nil, nil
}

func (m *mockForgeWithRefresher) Status(context.Context, *model.User, *model.Repo, *model.Pipeline, *model.Workflow) error {
	return nil
}

func (m *mockForgeWithRefresher) Netrc(*model.User, *model.Repo) (*model.Netrc, error) {
	return nil, nil
}

func (m *mockForgeWithRefresher) Activate(context.Context, *model.User, *model.Repo, string) error {
	return nil
}

func (m *mockForgeWithRefresher) Deactivate(context.Context, *model.User, *model.Repo, string) error {
	return nil
}

func (m *mockForgeWithRefresher) Branches(context.Context, *model.User, *model.Repo, *model.ListOptions) ([]string, error) {
	return nil, nil
}

func (m *mockForgeWithRefresher) BranchHead(context.Context, *model.User, *model.Repo, string) (*model.Commit, error) {
	return nil, nil
}

func (m *mockForgeWithRefresher) PullRequests(context.Context, *model.User, *model.Repo, *model.ListOptions) ([]*model.PullRequest, error) {
	return nil, nil
}

func (m *mockForgeWithRefresher) Hook(context.Context, *http.Request) (*model.Repo, *model.Pipeline, error) {
	return nil, nil, nil
}

func (m *mockForgeWithRefresher) OrgMembership(context.Context, *model.User, string) (*model.OrgPerm, error) {
	return nil, nil
}

func (m *mockForgeWithRefresher) Org(context.Context, *model.User, string) (*model.Org, error) {
	return nil, nil
}

const testForgeID = int64(1)

func TestRefreshUserForge_TokenNotExpiring(t *testing.T) {
	// Token expires in 2 hours - should not refresh
	user := &model.User{
		ID:     1,
		Login:  "testuser",
		Expiry: time.Now().Add(2 * time.Hour).Unix(),
	}

	mockForge := &mockForgeWithRefresher{}
	mockStore := mocks_store.NewMockStore(t)

	// Should not call Refresh or any store methods
	err := RefreshUserForge(context.Background(), mockForge, mockStore, user, testForgeID)

	assert.NoError(t, err)
	mockForge.AssertNotCalled(t, "Refresh", mock.Anything, mock.Anything)
}

func TestRefreshUserForge_TokenExpiringSoon(t *testing.T) {
	ClearCooldowns()

	// Token expires in 10 minutes - should refresh
	originalExpiry := time.Now().Add(10 * time.Minute).Unix()
	newExpiry := time.Now().Add(2 * time.Hour).Unix()

	user := &model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}

	mockForge := &mockForgeWithRefresher{}
	mockStore := mocks_store.NewMockStore(t)

	// UserForgeGet after acquiring lock
	mockStore.On("UserForgeGet", int64(1), testForgeID).Return(&model.UserForge{
		UserID:       1,
		ForgeID:      testForgeID,
		ForgeLogin:   "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}, nil).Once()

	// Refresh should be called and update the temp user
	mockForge.On("Refresh", mock.Anything, mock.AnythingOfType("*model.User")).
		Run(func(args mock.Arguments) {
			//nolint:forcetypeassert
			u := args.Get(1).(*model.User)
			u.AccessToken = "new-access-token"
			u.RefreshToken = "new-refresh-token"
			u.Expiry = newExpiry
		}).
		Return(true, nil).Once()

	// UserForgeUpdate should be called to persist the new tokens
	mockStore.On("UserForgeUpdate", mock.AnythingOfType("*model.UserForge")).Return(nil).Once()

	err := RefreshUserForge(context.Background(), mockForge, mockStore, user, testForgeID)

	assert.NoError(t, err)
	assert.Equal(t, "new-access-token", user.AccessToken)
	assert.Equal(t, "new-refresh-token", user.RefreshToken)
	assert.Equal(t, newExpiry, user.Expiry)
}

func TestRefreshUserForge_TokenAlreadyRefreshedByAnotherRequest(t *testing.T) {
	ClearCooldowns()

	// Token expires in 10 minutes - should check for refresh
	originalExpiry := time.Now().Add(10 * time.Minute).Unix()
	newExpiry := time.Now().Add(2 * time.Hour).Unix()

	user := &model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}

	mockForge := &mockForgeWithRefresher{}
	mockStore := mocks_store.NewMockStore(t)

	// When we reload the UserForge, it has already been refreshed by another goroutine
	mockStore.On("UserForgeGet", int64(1), testForgeID).Return(&model.UserForge{
		UserID:       1,
		ForgeID:      testForgeID,
		ForgeLogin:   "testuser",
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		Expiry:       newExpiry,
	}, nil).Once()

	// Refresh should NOT be called since token was already refreshed
	err := RefreshUserForge(context.Background(), mockForge, mockStore, user, testForgeID)

	assert.NoError(t, err)
	assert.Equal(t, "new-access-token", user.AccessToken)
	assert.Equal(t, "new-refresh-token", user.RefreshToken)
	assert.Equal(t, newExpiry, user.Expiry)
	mockForge.AssertNotCalled(t, "Refresh", mock.Anything, mock.Anything)
}

func TestRefreshUserForge_TokenAlreadyUsedError_RefreshedElsewhere(t *testing.T) {
	ClearCooldowns()

	// Simulates the case where another server instance refreshed the token
	originalExpiry := time.Now().Add(10 * time.Minute).Unix()
	newExpiry := time.Now().Add(2 * time.Hour).Unix()

	user := &model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}

	mockForge := &mockForgeWithRefresher{}
	mockStore := mocks_store.NewMockStore(t)

	// First UserForgeGet returns the original tokens (not yet refreshed)
	mockStore.On("UserForgeGet", int64(1), testForgeID).Return(&model.UserForge{
		UserID:       1,
		ForgeID:      testForgeID,
		ForgeLogin:   "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}, nil).Once()

	// Refresh fails with "token already used"
	mockForge.On("Refresh", mock.Anything, mock.AnythingOfType("*model.User")).
		Return(false, errors.New("oauth2: \"unauthorized_client\" \"token was already used\"")).Once()

	// Second UserForgeGet (after error) returns the refreshed tokens
	mockStore.On("UserForgeGet", int64(1), testForgeID).Return(&model.UserForge{
		UserID:       1,
		ForgeID:      testForgeID,
		ForgeLogin:   "testuser",
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		Expiry:       newExpiry,
	}, nil).Once()

	err := RefreshUserForge(context.Background(), mockForge, mockStore, user, testForgeID)

	assert.NoError(t, err)
	assert.Equal(t, "new-access-token", user.AccessToken)
	assert.Equal(t, "new-refresh-token", user.RefreshToken)
	assert.Equal(t, newExpiry, user.Expiry)
}

func TestRefreshUserForge_TokenAlreadyUsedError_NotRefreshedElsewhere(t *testing.T) {
	ClearCooldowns()

	// Token was not refreshed elsewhere - should set cooldown and return ErrTokenInvalidated
	// but should NOT clear tokens from DB (to avoid permanent lockout)
	originalExpiry := time.Now().Add(10 * time.Minute).Unix()

	user := &model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}

	mockForge := &mockForgeWithRefresher{}
	mockStore := mocks_store.NewMockStore(t)

	// First UserForgeGet returns the original tokens
	mockStore.On("UserForgeGet", int64(1), testForgeID).Return(&model.UserForge{
		UserID:       1,
		ForgeID:      testForgeID,
		ForgeLogin:   "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}, nil).Once()

	// Refresh fails with "token already used"
	tokenErr := errors.New("oauth2: \"unauthorized_client\" \"token was already used\"")
	mockForge.On("Refresh", mock.Anything, mock.AnythingOfType("*model.User")).
		Return(false, tokenErr).Once()

	// Second UserForgeGet returns the same expiry (not refreshed elsewhere)
	mockStore.On("UserForgeGet", int64(1), testForgeID).Return(&model.UserForge{
		UserID:       1,
		ForgeID:      testForgeID,
		ForgeLogin:   "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}, nil).Once()

	// UserForgeUpdate should NOT be called — tokens are preserved in DB
	err := RefreshUserForge(context.Background(), mockForge, mockStore, user, testForgeID)

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTokenInvalidated)
	mockStore.AssertNotCalled(t, "UserForgeUpdate", mock.Anything)
}

func TestRefreshUserForge_DBSaveFailure(t *testing.T) {
	ClearCooldowns()

	// If UserForgeUpdate fails, the error should be returned
	originalExpiry := time.Now().Add(10 * time.Minute).Unix()
	newExpiry := time.Now().Add(2 * time.Hour).Unix()

	user := &model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}

	mockForge := &mockForgeWithRefresher{}
	mockStore := mocks_store.NewMockStore(t)

	mockStore.On("UserForgeGet", int64(1), testForgeID).Return(&model.UserForge{
		UserID:       1,
		ForgeID:      testForgeID,
		ForgeLogin:   "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}, nil).Once()

	mockForge.On("Refresh", mock.Anything, mock.AnythingOfType("*model.User")).
		Run(func(args mock.Arguments) {
			//nolint:forcetypeassert
			u := args.Get(1).(*model.User)
			u.AccessToken = "new-access-token"
			u.RefreshToken = "new-refresh-token"
			u.Expiry = newExpiry
		}).
		Return(true, nil).Once()

	// UserForgeUpdate fails with database error
	dbErr := errors.New("database is locked")
	mockStore.On("UserForgeUpdate", mock.AnythingOfType("*model.UserForge")).Return(dbErr).Once()

	err := RefreshUserForge(context.Background(), mockForge, mockStore, user, testForgeID)

	// Error should be returned, not swallowed
	assert.Error(t, err)
	assert.Equal(t, dbErr, err)
}

func TestRefreshUserForge_ConcurrentRequests(t *testing.T) {
	ClearCooldowns()

	// Test that concurrent refresh attempts for the same user+forge are serialized
	originalExpiry := time.Now().Add(10 * time.Minute).Unix()
	newExpiry := time.Now().Add(2 * time.Hour).Unix()

	mockForge := &mockForgeWithRefresher{}
	mockStore := mocks_store.NewMockStore(t)

	var refreshCount atomic.Int32
	var userForgeGetCount atomic.Int32

	// UserForgeGet will be called multiple times
	mockStore.On("UserForgeGet", int64(1), testForgeID).Return(func(_ int64, _ int64) *model.UserForge {
		count := userForgeGetCount.Add(1)
		// First call returns old token, subsequent calls return new token
		if count == 1 {
			return &model.UserForge{
				UserID:       1,
				ForgeID:      testForgeID,
				ForgeLogin:   "testuser",
				AccessToken:  "old-access-token",
				RefreshToken: "old-refresh-token",
				Expiry:       originalExpiry,
			}
		}
		return &model.UserForge{
			UserID:       1,
			ForgeID:      testForgeID,
			ForgeLogin:   "testuser",
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
			Expiry:       newExpiry,
		}
	}, nil)

	// Refresh should only be called once due to locking
	mockForge.On("Refresh", mock.Anything, mock.AnythingOfType("*model.User")).
		Run(func(args mock.Arguments) {
			refreshCount.Add(1)
			//nolint:forcetypeassert
			u := args.Get(1).(*model.User)
			u.AccessToken = "new-access-token"
			u.RefreshToken = "new-refresh-token"
			u.Expiry = newExpiry
			// Simulate some delay
			time.Sleep(50 * time.Millisecond)
		}).
		Return(true, nil).Maybe()

	mockStore.On("UserForgeUpdate", mock.AnythingOfType("*model.UserForge")).Return(nil).Maybe()

	// Launch multiple concurrent refresh requests
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			user := &model.User{
				ID:           1,
				Login:        "testuser",
				AccessToken:  "old-access-token",
				RefreshToken: "old-refresh-token",
				Expiry:       originalExpiry,
			}
			err := RefreshUserForge(context.Background(), mockForge, mockStore, user, testForgeID)
			assert.NoError(t, err)
			// All users should end up with the new token
			assert.Equal(t, "new-access-token", user.AccessToken)
		}()
	}
	wg.Wait()

	// Refresh should only have been called once (or at most a few times if timing is unlucky)
	assert.LessOrEqual(t, int(refreshCount.Load()), 2, "Refresh should be called at most twice due to locking")
}

func TestIsTokenAlreadyUsedError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "token was already used",
			err:      errors.New("oauth2: \"unauthorized_client\" \"token was already used\""),
			expected: true,
		},
		{
			name:     "token has been revoked",
			err:      errors.New("token has been revoked"),
			expected: true,
		},
		{
			name:     "invalid_grant error",
			err:      errors.New("oauth2: invalid_grant"),
			expected: true,
		},
		{
			name:     "unrelated error",
			err:      errors.New("network timeout"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTokenAlreadyUsedError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRefreshUserForge_CooldownPreventsRetryLoop(t *testing.T) {
	ClearCooldowns()

	originalExpiry := time.Now().Add(10 * time.Minute).Unix()

	mockForge := &mockForgeWithRefresher{}
	mockStore := mocks_store.NewMockStore(t)

	// First UserForgeGet returns the original tokens
	mockStore.On("UserForgeGet", int64(1), testForgeID).Return(&model.UserForge{
		UserID:       1,
		ForgeID:      testForgeID,
		ForgeLogin:   "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}, nil).Once()

	// Refresh fails with "token already used"
	tokenErr := errors.New("oauth2: \"unauthorized_client\" \"token was already used\"")
	mockForge.On("Refresh", mock.Anything, mock.AnythingOfType("*model.User")).
		Return(false, tokenErr).Once()

	// Second UserForgeGet returns the same expiry (not refreshed elsewhere)
	mockStore.On("UserForgeGet", int64(1), testForgeID).Return(&model.UserForge{
		UserID:       1,
		ForgeID:      testForgeID,
		ForgeLogin:   "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}, nil).Once()

	user := &model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}

	// First call should fail with ErrTokenInvalidated
	err := RefreshUserForge(context.Background(), mockForge, mockStore, user, testForgeID)
	assert.ErrorIs(t, err, ErrTokenInvalidated)

	// Second call should immediately return ErrTokenInvalidated due to cooldown
	// WITHOUT making any additional store or forge calls
	err = RefreshUserForge(context.Background(), mockForge, mockStore, user, testForgeID)
	assert.ErrorIs(t, err, ErrTokenInvalidated)

	// Third call should also be blocked
	err = RefreshUserForge(context.Background(), mockForge, mockStore, user, testForgeID)
	assert.ErrorIs(t, err, ErrTokenInvalidated)

	// Verify that UserForgeGet and Refresh were only called for the first attempt
	mockStore.AssertNumberOfCalls(t, "UserForgeGet", 2) // First attempt calls UserForgeGet twice
	mockForge.AssertNumberOfCalls(t, "Refresh", 1)
	mockStore.AssertNotCalled(t, "UserForgeUpdate", mock.Anything)
}

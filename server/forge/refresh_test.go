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
	store_types "codefloe.com/crowci/crow/v5/server/store/types"
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

func TestRefresh_TokenNotExpiring(t *testing.T) {
	// Token expires in 2 hours - should not refresh
	user := &model.User{
		ID:     1,
		Login:  "testuser",
		Expiry: time.Now().Add(2 * time.Hour).Unix(),
	}

	mockForge := &mockForgeWithRefresher{}
	mockStore := mocks_store.NewMockStore(t)

	// Should not call Refresh or any store methods
	err := Refresh(context.Background(), mockForge, mockStore, user)

	assert.NoError(t, err)
	mockForge.AssertNotCalled(t, "Refresh", mock.Anything, mock.Anything)
}

func TestRefresh_TokenExpiringSoon(t *testing.T) {
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

	// First GetUser call after acquiring lock
	mockStore.On("GetUser", int64(1)).Return(&model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}, nil).Once()

	// Refresh should be called and update the user
	mockForge.On("Refresh", mock.Anything, mock.AnythingOfType("*model.User")).
		Run(func(args mock.Arguments) {
			//nolint:forcetypeassert
			u := args.Get(1).(*model.User)
			u.AccessToken = "new-access-token"
			u.RefreshToken = "new-refresh-token"
			u.Expiry = newExpiry
		}).
		Return(true, nil).Once()

	// UpdateUser should be called to persist the new tokens
	mockStore.On("UpdateUser", mock.AnythingOfType("*model.User")).Return(nil).Once()

	// Multi-forge: syncUserForgeTokens is called but legacy user has no UserForge record
	mockStore.On("UserForgeGet", int64(1), int64(0)).Return(nil, store_types.RecordNotExist).Once()

	err := Refresh(context.Background(), mockForge, mockStore, user)

	assert.NoError(t, err)
	assert.Equal(t, "new-access-token", user.AccessToken)
	assert.Equal(t, "new-refresh-token", user.RefreshToken)
	assert.Equal(t, newExpiry, user.Expiry)
}

func TestRefresh_TokenAlreadyRefreshedByAnotherRequest(t *testing.T) {
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

	// When we reload the user, it has already been refreshed by another goroutine
	mockStore.On("GetUser", int64(1)).Return(&model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		Expiry:       newExpiry,
	}, nil).Once()

	// Refresh should NOT be called since token was already refreshed
	err := Refresh(context.Background(), mockForge, mockStore, user)

	assert.NoError(t, err)
	assert.Equal(t, "new-access-token", user.AccessToken)
	assert.Equal(t, "new-refresh-token", user.RefreshToken)
	assert.Equal(t, newExpiry, user.Expiry)
	mockForge.AssertNotCalled(t, "Refresh", mock.Anything, mock.Anything)
}

func TestRefresh_TokenAlreadyUsedError_RefreshedElsewhere(t *testing.T) {
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

	// First GetUser returns the original user (not yet refreshed)
	mockStore.On("GetUser", int64(1)).Return(&model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}, nil).Once()

	// Refresh fails with "token already used"
	mockForge.On("Refresh", mock.Anything, mock.AnythingOfType("*model.User")).
		Return(false, errors.New("oauth2: \"unauthorized_client\" \"token was already used\"")).Once()

	// Second GetUser (after error) returns the refreshed user
	mockStore.On("GetUser", int64(1)).Return(&model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		Expiry:       newExpiry,
	}, nil).Once()

	err := Refresh(context.Background(), mockForge, mockStore, user)

	assert.NoError(t, err)
	assert.Equal(t, "new-access-token", user.AccessToken)
	assert.Equal(t, "new-refresh-token", user.RefreshToken)
	assert.Equal(t, newExpiry, user.Expiry)
}

func TestRefresh_TokenAlreadyUsedError_NotRefreshedElsewhere(t *testing.T) {
	// Token was not refreshed elsewhere - should clear tokens and return ErrTokenInvalidated
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

	// First GetUser returns the original user
	mockStore.On("GetUser", int64(1)).Return(&model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}, nil).Once()

	// Refresh fails with "token already used"
	tokenErr := errors.New("oauth2: \"unauthorized_client\" \"token was already used\"")
	mockForge.On("Refresh", mock.Anything, mock.AnythingOfType("*model.User")).
		Return(false, tokenErr).Once()

	// Second GetUser returns the same expiry (not refreshed elsewhere)
	mockStore.On("GetUser", int64(1)).Return(&model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}, nil).Once()

	// Token is permanently invalid - UpdateUser should be called to clear the tokens
	mockStore.On("UpdateUser", mock.AnythingOfType("*model.User")).Return(nil).Once()

	// UserForgeGet may be called to clear UserForge tokens (may not exist)
	mockStore.On("UserForgeGet", int64(1), int64(0)).Return(nil, errors.New("not found")).Maybe()

	err := Refresh(context.Background(), mockForge, mockStore, user)

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTokenInvalidated)
}

func TestRefresh_DBSaveFailure(t *testing.T) {
	// Clear cooldown state from previous tests
	ClearCooldowns()

	// If UpdateUser fails, the error should be returned (critical fix)
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

	mockStore.On("GetUser", int64(1)).Return(&model.User{
		ID:           1,
		Login:        "testuser",
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

	// UpdateUser fails with database error
	dbErr := errors.New("database is locked")
	mockStore.On("UpdateUser", mock.AnythingOfType("*model.User")).Return(dbErr).Once()

	err := Refresh(context.Background(), mockForge, mockStore, user)

	// Error should be returned, not swallowed
	assert.Error(t, err)
	assert.Equal(t, dbErr, err)
}

func TestRefresh_ConcurrentRequests(t *testing.T) {
	// Clear cooldown state from previous tests
	ClearCooldowns()

	// Test that concurrent refresh attempts for the same user are serialized
	originalExpiry := time.Now().Add(10 * time.Minute).Unix()
	newExpiry := time.Now().Add(2 * time.Hour).Unix()

	mockForge := &mockForgeWithRefresher{}
	mockStore := mocks_store.NewMockStore(t)

	var refreshCount atomic.Int32
	var getUserCount atomic.Int32

	// GetUser will be called multiple times
	mockStore.On("GetUser", int64(1)).Return(func(id int64) *model.User {
		count := getUserCount.Add(1)
		// First call returns old token, subsequent calls return new token
		if count == 1 {
			return &model.User{
				ID:           1,
				Login:        "testuser",
				AccessToken:  "old-access-token",
				RefreshToken: "old-refresh-token",
				Expiry:       originalExpiry,
			}
		}
		return &model.User{
			ID:           1,
			Login:        "testuser",
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

	mockStore.On("UpdateUser", mock.AnythingOfType("*model.User")).Return(nil).Maybe()

	// Multi-forge: syncUserForgeTokens is called but legacy user has no UserForge record
	mockStore.On("UserForgeGet", int64(1), int64(0)).Return(nil, store_types.RecordNotExist).Maybe()

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
			err := Refresh(context.Background(), mockForge, mockStore, user)
			assert.NoError(t, err)
			// All users should end up with the new token
			assert.Equal(t, "new-access-token", user.AccessToken)
		}()
	}
	wg.Wait()

	// Refresh should only have been called once (or at most a few times if timing is unlucky)
	// The key is that we don't get 5 refresh calls
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

func TestRefresh_CooldownPreventsRetryLoop(t *testing.T) {
	// Test that after a permanent token failure, subsequent requests
	// are blocked by cooldown to prevent tight retry loops
	ClearCooldowns()

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

	// First GetUser returns the original user
	mockStore.On("GetUser", int64(1)).Return(&model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}, nil).Once()

	// Refresh fails with "token already used"
	tokenErr := errors.New("oauth2: \"unauthorized_client\" \"token was already used\"")
	mockForge.On("Refresh", mock.Anything, mock.AnythingOfType("*model.User")).
		Return(false, tokenErr).Once()

	// Second GetUser returns the same expiry (not refreshed elsewhere)
	mockStore.On("GetUser", int64(1)).Return(&model.User{
		ID:           1,
		Login:        "testuser",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		Expiry:       originalExpiry,
	}, nil).Once()

	// Token is permanently invalid - UpdateUser should be called to clear the tokens
	mockStore.On("UpdateUser", mock.AnythingOfType("*model.User")).Return(nil).Once()

	// UserForgeGet may be called to clear UserForge tokens (may not exist)
	mockStore.On("UserForgeGet", int64(1), int64(0)).Return(nil, errors.New("not found")).Maybe()

	// First call should fail with ErrTokenInvalidated
	err := Refresh(context.Background(), mockForge, mockStore, user)
	assert.ErrorIs(t, err, ErrTokenInvalidated)

	// Second call should immediately return ErrTokenInvalidated due to cooldown
	// WITHOUT making any additional store or forge calls
	err = Refresh(context.Background(), mockForge, mockStore, user)
	assert.ErrorIs(t, err, ErrTokenInvalidated)

	// Third call should also be blocked
	err = Refresh(context.Background(), mockForge, mockStore, user)
	assert.ErrorIs(t, err, ErrTokenInvalidated)

	// Verify that GetUser and Refresh were only called once (for the first attempt)
	mockStore.AssertNumberOfCalls(t, "GetUser", 2) // First attempt calls GetUser twice
	mockForge.AssertNumberOfCalls(t, "Refresh", 1)
}

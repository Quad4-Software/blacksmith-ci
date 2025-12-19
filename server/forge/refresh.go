// Copyright 2023 Woodpecker Authors
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
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/store"
)

// Refresher refreshes an oauth token and expiration for the given user. It
// returns true if the token was refreshed, false if the token was not refreshed,
// and error if it failed to refresh.
type Refresher interface {
	Refresh(context.Context, *model.User) (bool, error)
}

// userRefreshLocks prevents concurrent token refresh attempts for the same user.
// This is critical because OAuth refresh tokens are single-use: if two requests
// try to refresh simultaneously, one will fail with "token already used".
var userRefreshLocks sync.Map

// getUserLock returns a mutex for the given user ID, creating one if necessary.
func getUserLock(userID int64) *sync.Mutex {
	lock, _ := userRefreshLocks.LoadOrStore(userID, &sync.Mutex{})
	//nolint:forcetypeassert
	return lock.(*sync.Mutex)
}

// Remaining ttl of 30 minutes (1800 seconds) until a token is refreshed.
const tokenMinTTL = 1800

func Refresh(c context.Context, forge Forge, _store store.Store, user *model.User) error {
	refresher, ok := forge.(Refresher)
	if !ok {
		return nil
	}

	// Check if token needs refresh before acquiring lock (fast path)
	currentTime := time.Now().UTC().Unix()
	if currentTime < (user.Expiry - tokenMinTTL) {
		return nil
	}

	// Acquire per-user lock to prevent concurrent refresh attempts
	lock := getUserLock(user.ID)
	lock.Lock()
	defer lock.Unlock()

	// Re-check after acquiring lock: another goroutine may have refreshed the token
	// Reload user from DB to get the latest token state
	freshUser, err := _store.GetUser(user.ID)
	if err != nil {
		log.Error().Err(err).Int64("user_id", user.ID).Msg("failed to reload user for token refresh check")
		return err
	}

	// Check again with fresh data - token may have been refreshed by another request
	currentTime = time.Now().UTC().Unix()
	if currentTime < (freshUser.Expiry - tokenMinTTL) {
		// Token was refreshed by another request, update the caller's user object
		user.AccessToken = freshUser.AccessToken
		user.RefreshToken = freshUser.RefreshToken
		user.Expiry = freshUser.Expiry
		log.Debug().Str("user", user.Login).Msg("token was already refreshed by another request")
		return nil
	}

	// Perform the actual token refresh
	refreshed, err := refresher.Refresh(c, freshUser)
	if err != nil {
		// Check if this is a "token already used" error - could mean another instance refreshed it
		if isTokenAlreadyUsedError(err) {
			log.Warn().Err(err).Str("user", user.Login).Msg("refresh token was already used, checking if token was refreshed elsewhere")

			// Reload user one more time to check if the token was refreshed by another server instance
			reloadedUser, reloadErr := _store.GetUser(user.ID)
			if reloadErr == nil && reloadedUser.Expiry > freshUser.Expiry {
				// Token was refreshed elsewhere, use the new values
				user.AccessToken = reloadedUser.AccessToken
				user.RefreshToken = reloadedUser.RefreshToken
				user.Expiry = reloadedUser.Expiry
				log.Info().Str("user", user.Login).Msg("token was refreshed by another server instance")
				return nil
			}
		}

		log.Error().Err(err).Str("user", user.Login).Msg("failed to refresh OAuth token")
		return err
	}

	if refreshed {
		// Save the new tokens to DB - this is critical, don't ignore the error!
		if err := _store.UpdateUser(freshUser); err != nil {
			log.Error().Err(err).Str("user", user.Login).Msg("failed saving user to DB after refreshing OAuth token - tokens will be lost!")
			return err
		}

		// Update the caller's user object with the new tokens
		user.AccessToken = freshUser.AccessToken
		user.RefreshToken = freshUser.RefreshToken
		user.Expiry = freshUser.Expiry

		log.Debug().Str("user", user.Login).Msg("successfully refreshed OAuth token")
	}

	return nil
}

// isTokenAlreadyUsedError checks if the error indicates the refresh token was already consumed.
func isTokenAlreadyUsedError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "token was already used") ||
		strings.Contains(errStr, "token has been revoked") ||
		strings.Contains(errStr, "invalid_grant")
}

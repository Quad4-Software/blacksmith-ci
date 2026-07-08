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
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
)

// ErrTokenInvalidated is returned when a token has been permanently invalidated
// and the user needs to re-authenticate. The tokens have been cleared from the DB.
var ErrTokenInvalidated = errors.New("oauth token permanently invalidated, re-authentication required")

// Refresher refreshes an oauth token and expiration for the given user. It
// returns true if the token was refreshed, false if the token was not refreshed,
// and error if it failed to refresh.
type Refresher interface {
	Refresh(context.Context, *model.User) (bool, error)
}

// refreshCooldownDuration is how long to wait before retrying after a permanent token failure.
const refreshCooldownDuration = 30 * time.Second

// ClearCooldowns clears all cooldown state. This is exported for testing purposes.
func ClearCooldowns() {
	userForgeCooldowns = sync.Map{}
}

// Remaining ttl of 30 minutes (1800 seconds) until a token is refreshed.
const tokenMinTTL = 1800

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

// userForgeLockKey generates a unique lock key for a user+forge combination.
func userForgeLockKey(userID, forgeID int64) string {
	return fmt.Sprintf("%d-%d", userID, forgeID)
}

// userForgeRefreshLocks prevents concurrent token refresh attempts for the same user+forge.
var userForgeRefreshLocks sync.Map

// getUserForgeLock returns a mutex for the given user+forge combination, creating one if necessary.
func getUserForgeLock(userID, forgeID int64) *sync.Mutex {
	key := userForgeLockKey(userID, forgeID)
	lock, _ := userForgeRefreshLocks.LoadOrStore(key, &sync.Mutex{})
	//nolint:forcetypeassert
	return lock.(*sync.Mutex)
}

// userForgeCooldowns tracks user+forge combinations with recent refresh failures.
var userForgeCooldowns sync.Map

// isUserForgeCooldown checks if a user+forge is in cooldown period after a refresh failure.
func isUserForgeCooldown(userID, forgeID int64) bool {
	key := userForgeLockKey(userID, forgeID)
	if cooldownUntil, ok := userForgeCooldowns.Load(key); ok {
		//nolint:forcetypeassert
		return time.Now().Before(cooldownUntil.(time.Time))
	}
	return false
}

// setUserForgeCooldown sets a cooldown period for a user+forge after a permanent refresh failure.
func setUserForgeCooldown(userID, forgeID int64) {
	key := userForgeLockKey(userID, forgeID)
	userForgeCooldowns.Store(key, time.Now().Add(refreshCooldownDuration))
}

// RefreshUserForge refreshes OAuth tokens for a forge connection using the
// UserForge table as the canonical source of truth. The user parameter is
// modified in-place with the refreshed tokens.
func RefreshUserForge(c context.Context, forge Forge, _store store.Store, user *model.User, forgeID int64) error {
	refresher, ok := forge.(Refresher)
	if !ok {
		return nil
	}

	// Check if token needs refresh before acquiring lock (fast path)
	currentTime := time.Now().UTC().Unix()
	if currentTime < (user.Expiry - tokenMinTTL) {
		return nil
	}

	// Check if user+forge is in cooldown period after a recent permanent failure
	if isUserForgeCooldown(user.ID, forgeID) {
		return ErrTokenInvalidated
	}

	// Acquire per-user-forge lock to prevent concurrent refresh attempts
	lock := getUserForgeLock(user.ID, forgeID)
	lock.Lock()
	defer lock.Unlock()

	// Re-check after acquiring lock: another goroutine may have refreshed the token
	// Reload UserForge from DB to get the latest token state
	freshUserForge, err := _store.UserForgeGet(user.ID, forgeID)
	if err != nil {
		log.Error().Err(err).Int64("user_id", user.ID).Int64("forge_id", forgeID).Msg("failed to reload UserForge for token refresh check")
		return err
	}

	// Check again with fresh data - token may have been refreshed by another request
	currentTime = time.Now().UTC().Unix()
	if currentTime < (freshUserForge.Expiry - tokenMinTTL) {
		// Token was refreshed by another request, update the caller's user object
		user.AccessToken = freshUserForge.AccessToken
		user.RefreshToken = freshUserForge.RefreshToken
		user.Expiry = freshUserForge.Expiry
		log.Debug().Str("user", user.Login).Int64("forge_id", forgeID).Msg("UserForge token was already refreshed by another request")
		return nil
	}

	// Create a temporary user object with UserForge tokens for the refresher
	tempUser := &model.User{
		ID:           user.ID,
		Login:        freshUserForge.ForgeLogin,
		AccessToken:  freshUserForge.AccessToken,
		RefreshToken: freshUserForge.RefreshToken,
		Expiry:       freshUserForge.Expiry,
		ForgeID:      forgeID,
	}

	// Perform the actual token refresh
	refreshed, err := refresher.Refresh(c, tempUser)
	if err != nil {
		// Check if this is a "token already used" error - could mean another instance refreshed it
		if isTokenAlreadyUsedError(err) {
			log.Warn().Err(err).Str("user", user.Login).Int64("forge_id", forgeID).Msg("UserForge refresh token was already used, checking if token was refreshed elsewhere")

			// Reload UserForge one more time to check if the token was refreshed by another server instance
			reloadedUserForge, reloadErr := _store.UserForgeGet(user.ID, forgeID)
			// Check if the token was refreshed: either expiry increased OR refresh token changed
			// (comparing refresh tokens handles the case where we loaded freshUserForge after another instance already saved)
			if reloadErr == nil && (reloadedUserForge.Expiry > freshUserForge.Expiry ||
				(reloadedUserForge.RefreshToken != "" && reloadedUserForge.RefreshToken != freshUserForge.RefreshToken)) {
				// Token was refreshed elsewhere, use the new values
				user.AccessToken = reloadedUserForge.AccessToken
				user.RefreshToken = reloadedUserForge.RefreshToken
				user.Expiry = reloadedUserForge.Expiry
				log.Info().Str("user", user.Login).Int64("forge_id", forgeID).Msg("UserForge token was refreshed by another server instance")
				return nil
			}

			// Token appears invalid — set cooldown to avoid tight retry loops, but do NOT
			// clear tokens from DB. Clearing causes a permanent lockout until the user
			// manually re-authenticates. By keeping the tokens, a subsequent re-login
			// (which overwrites them) restores normal operation without data loss.
			log.Warn().Str("user", user.Login).Int64("forge_id", forgeID).Msg("UserForge OAuth refresh token was consumed but no newer token found; setting cooldown")
			setUserForgeCooldown(user.ID, forgeID)
			return ErrTokenInvalidated
		}

		log.Error().Err(err).Str("user", user.Login).Int64("forge_id", forgeID).Msg("failed to refresh UserForge OAuth token")
		return err
	}

	if refreshed {
		// Update the UserForge record with the new tokens
		freshUserForge.AccessToken = tempUser.AccessToken
		freshUserForge.RefreshToken = tempUser.RefreshToken
		freshUserForge.Expiry = tempUser.Expiry

		if err := _store.UserForgeUpdate(freshUserForge); err != nil {
			log.Error().Err(err).Str("user", user.Login).Int64("forge_id", forgeID).Msg("failed saving UserForge to DB after refreshing OAuth token - tokens will be lost!")
			return err
		}

		// Update the caller's user object with the new tokens
		user.AccessToken = tempUser.AccessToken
		user.RefreshToken = tempUser.RefreshToken
		user.Expiry = tempUser.Expiry

		log.Debug().Str("user", user.Login).Int64("forge_id", forgeID).Msg("successfully refreshed UserForge OAuth token")
	}

	return nil
}

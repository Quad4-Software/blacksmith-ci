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

	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/store"
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

// refreshFailureCooldowns tracks users with recent refresh failures to prevent tight retry loops.
// Maps user ID to the time when they can next attempt a refresh.
var refreshFailureCooldowns sync.Map

// refreshCooldownDuration is how long to wait before retrying after a permanent token failure.
const refreshCooldownDuration = 30 * time.Second

// isInCooldown checks if a user is in cooldown period after a refresh failure.
func isInCooldown(userID int64) bool {
	if cooldownUntil, ok := refreshFailureCooldowns.Load(userID); ok {
		//nolint:forcetypeassert
		return time.Now().Before(cooldownUntil.(time.Time))
	}
	return false
}

// setCooldown sets a cooldown period for a user after a permanent refresh failure.
func setCooldown(userID int64) {
	refreshFailureCooldowns.Store(userID, time.Now().Add(refreshCooldownDuration))
}

// ClearCooldowns clears all cooldown state. This is exported for testing purposes.
func ClearCooldowns() {
	refreshFailureCooldowns = sync.Map{}
	userForgeCooldowns = sync.Map{}
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

	// Check if user is in cooldown period after a recent permanent failure
	// This prevents tight retry loops when tokens are permanently invalid
	if isInCooldown(user.ID) {
		return ErrTokenInvalidated
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
			// Check if the token was refreshed: either expiry increased OR refresh token changed
			// (comparing refresh tokens handles the case where we loaded freshUser after another instance already saved)
			if reloadErr == nil && (reloadedUser.Expiry > freshUser.Expiry ||
				(reloadedUser.RefreshToken != "" && reloadedUser.RefreshToken != freshUser.RefreshToken)) {
				// Token was refreshed elsewhere, use the new values
				user.AccessToken = reloadedUser.AccessToken
				user.RefreshToken = reloadedUser.RefreshToken
				user.Expiry = reloadedUser.Expiry
				log.Info().Str("user", user.Login).Msg("token was refreshed by another server instance")
				return nil
			}

			// Token is permanently invalid - clear it from DB and set cooldown to prevent tight loops
			log.Warn().Str("user", user.Login).Msg("OAuth token permanently invalid, clearing tokens from DB")
			clearUserTokens(_store, freshUser)
			setCooldown(user.ID)
			return ErrTokenInvalidated
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

		// Also update UserForge record if it exists (multi-forge support)
		syncUserForgeTokens(c, _store, freshUser)

		// Update the caller's user object with the new tokens
		user.AccessToken = freshUser.AccessToken
		user.RefreshToken = freshUser.RefreshToken
		user.Expiry = freshUser.Expiry

		log.Debug().Str("user", user.Login).Msg("successfully refreshed OAuth token")
	}

	return nil
}

// syncUserForgeTokens updates the UserForge record with refreshed tokens.
// This keeps the UserForge table in sync with refreshed tokens for multi-forge support.
func syncUserForgeTokens(_ context.Context, _store store.Store, user *model.User) {
	userForge, err := _store.UserForgeGet(user.ID, user.ForgeID)
	if err != nil {
		// UserForge may not exist for legacy users - that's okay
		return
	}

	userForge.AccessToken = user.AccessToken
	userForge.RefreshToken = user.RefreshToken
	userForge.Expiry = user.Expiry

	if err := _store.UserForgeUpdate(userForge); err != nil {
		log.Error().Err(err).
			Int64("user_id", user.ID).
			Int64("forge_id", user.ForgeID).
			Msg("failed to sync refreshed tokens to UserForge")
	}
}

// clearUserTokens clears invalid OAuth tokens from both the users and user_forges tables.
// This is called when a token is permanently invalid to prevent endless refresh loops.
func clearUserTokens(_store store.Store, user *model.User) {
	// Clear tokens from user record
	user.AccessToken = ""
	user.RefreshToken = ""
	user.Expiry = 0
	if err := _store.UpdateUser(user); err != nil {
		log.Error().Err(err).Str("user", user.Login).Msg("failed to clear invalid tokens from user record")
	}

	// Also clear from UserForge if it exists
	userForge, err := _store.UserForgeGet(user.ID, user.ForgeID)
	if err != nil {
		return // UserForge may not exist
	}

	userForge.AccessToken = ""
	userForge.RefreshToken = ""
	userForge.Expiry = 0
	if err := _store.UserForgeUpdate(userForge); err != nil {
		log.Error().Err(err).
			Int64("user_id", user.ID).
			Int64("forge_id", user.ForgeID).
			Msg("failed to clear invalid tokens from UserForge record")
	}
}

// clearUserForgeTokens clears invalid OAuth tokens from a UserForge record.
func clearUserForgeTokens(_store store.Store, userForge *model.UserForge) {
	userForge.AccessToken = ""
	userForge.RefreshToken = ""
	userForge.Expiry = 0
	if err := _store.UserForgeUpdate(userForge); err != nil {
		log.Error().Err(err).
			Int64("user_id", userForge.UserID).
			Int64("forge_id", userForge.ForgeID).
			Msg("failed to clear invalid tokens from UserForge record")
	}
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

// RefreshUserForge refreshes OAuth tokens for a secondary (non-primary) forge connection.
// This is similar to Refresh but operates on UserForge records instead of User records.
// The user parameter is modified in-place with the refreshed tokens.
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

			// Token is permanently invalid - clear it from DB and set cooldown to prevent tight loops
			log.Warn().Str("user", user.Login).Int64("forge_id", forgeID).Msg("UserForge OAuth token permanently invalid, clearing tokens from DB")
			clearUserForgeTokens(_store, freshUserForge)
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

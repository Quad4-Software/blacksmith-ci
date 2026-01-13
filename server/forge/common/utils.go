// Copyright 2022 Woodpecker Authors
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

package common

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"

	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/store"
)

func ExtractHostFromCloneURL(cloneURL string) (string, error) {
	u, err := url.Parse(cloneURL)
	if err != nil {
		return "", err
	}

	if !strings.Contains(u.Host, ":") {
		return u.Host, nil
	}

	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", err
	}

	return host, nil
}

func UserToken(ctx context.Context, r *model.Repo, u *model.User) string {
	if u != nil && r != nil {
		// For multi-forge support: look up the user's token for the repo's forge.
		// This handles cases where:
		// 1. User's primary forge differs from repo's forge
		// 2. User's ForgeID is 0 (legacy data before multi-forge)
		_store, ok := store.TryFromContext(ctx)
		if ok {
			if userForge, err := _store.UserForgeGet(u.ID, r.ForgeID); err == nil && userForge != nil {
				return userForge.AccessToken
			}
		}
		// Fall back to user's primary token if no UserForge entry found
		return u.AccessToken
	}

	if u != nil {
		return u.AccessToken
	}

	user, err := RepoUser(ctx, r)
	if err != nil {
		log.Error().Err(err).Msg("could not get repo user")
		return ""
	}
	return user.AccessToken
}

func RepoUser(ctx context.Context, r *model.Repo) (*model.User, error) {
	_store, ok := store.TryFromContext(ctx)
	if !ok {
		return nil, errors.New("could not get store from context")
	}
	if r == nil {
		log.Error().Msg("cannot get user token by empty repo")
		return nil, errors.New("cannot get user token by empty repo")
	}
	user, err := _store.GetUser(r.UserID)
	if err != nil {
		return nil, err
	}

	// For multi-forge support: use tokens from UserForge if available for this repo's forge.
	// This ensures we use the correct OAuth tokens for the forge the repo belongs to.
	if userForge, ufErr := _store.UserForgeGet(user.ID, r.ForgeID); ufErr == nil && userForge != nil {
		user.AccessToken = userForge.AccessToken
		user.RefreshToken = userForge.RefreshToken
		user.Expiry = userForge.Expiry
		user.ForgeID = userForge.ForgeID
		user.ForgeRemoteID = userForge.ForgeRemoteID
	}

	return user, nil
}

func RepoUserForgeID(ctx context.Context, repoForgeID model.ForgeRemoteID) (*model.User, error) {
	_store, ok := store.TryFromContext(ctx)
	if !ok {
		return nil, errors.New("could not get store from context")
	}
	r, err := _store.GetRepoForgeID(repoForgeID)
	if err != nil {
		return nil, err
	}
	return RepoUser(ctx, r)
}

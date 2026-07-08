// Copyright 2026 Crow CI Authors
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

package bootstrap

import (
	"encoding/base32"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/tink-crypto/tink-go/v2/subtle/random"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store"
	"codefloe.com/crowci/crow/v6/server/store/datastore"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

const minAdminTokenLen = 32

const bootstrapTokenName = "admin-bootstrap"

// AdminToken reconciles the bootstrap admin token from CROW_ADMIN_TOKEN.
//
// When raw is empty the feature is disabled and nothing happens. Otherwise it
// ensures a dedicated forge-less system admin user owns exactly one access
// token whose hash equals SHA256(raw): the token is created if missing and any
// other token owned by that user is deleted, so changing the secret rotates
// (and revokes) the previous credential.
func AdminToken(s store.Store, raw string) error {
	if raw == "" {
		return nil
	}
	if len(raw) < minAdminTokenLen {
		return fmt.Errorf("CROW_ADMIN_TOKEN must be at least %d characters", minAdminTokenLen)
	}

	user, err := ensureSystemAdmin(s)
	if err != nil {
		return fmt.Errorf("ensure system admin user: %w", err)
	}

	hash := datastore.HashAccessToken(raw)

	tokens, err := s.AccessTokenList(user.ID, &model.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("list bootstrap tokens: %w", err)
	}

	matched := false
	for _, t := range tokens {
		if t.TokenHash == hash {
			matched = true
			continue
		}
		if err := s.AccessTokenDelete(t); err != nil {
			return fmt.Errorf("revoke stale bootstrap token: %w", err)
		}
	}

	if !matched {
		scopes := make(model.JSONSliceString, 0, len(model.AllScopes()))
		for _, sc := range model.AllScopes() {
			scopes = append(scopes, string(sc))
		}
		token := &model.AccessToken{
			UserID:    user.ID,
			Name:      bootstrapTokenName,
			TokenHash: hash,
			Scopes:    scopes,
			CreatedAt: time.Now().Unix(),
		}
		if err := s.AccessTokenCreate(token); err != nil {
			return fmt.Errorf("create bootstrap token: %w", err)
		}
		log.Info().Msg("created admin bootstrap token from CROW_ADMIN_TOKEN")
	}

	return nil
}

func ensureSystemAdmin(s store.Store) (*model.User, error) {
	user, err := s.GetUserLogin(model.SystemAdminLogin)
	if err == nil {
		if !user.Admin {
			user.Admin = true
			if err := s.UpdateUser(user); err != nil {
				return nil, err
			}
		}
		return user, nil
	}
	if !errors.Is(err, types.RecordNotExist) {
		return nil, err
	}

	user = &model.User{
		Login:         model.SystemAdminLogin,
		Admin:         true,
		Hash:          base32.StdEncoding.EncodeToString(random.GetRandomBytes(32)),
		ForgeID:       1,
		ForgeRemoteID: model.ForgeRemoteID("0"),
	}
	if err := s.CreateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

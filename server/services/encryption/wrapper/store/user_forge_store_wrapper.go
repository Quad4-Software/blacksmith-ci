// Copyright 2026 Crow Authors
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

package store

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/services/encryption/types"
	"codefloe.com/crowci/crow/v6/server/store"
)

type EncryptedUserForgeStore struct {
	store      store.Store
	encryption types.EncryptionService
}

func NewUserForgeStore(userForgeStore store.Store) *EncryptedUserForgeStore {
	wrapper := EncryptedUserForgeStore{userForgeStore, nil}
	return &wrapper
}

func (wrapper *EncryptedUserForgeStore) SetEncryptionService(service types.EncryptionService) error {
	if wrapper.encryption != nil {
		return errors.New(errMessageInitSeveralTimes)
	}
	wrapper.encryption = service
	return nil
}

func (wrapper *EncryptedUserForgeStore) EnableEncryption() error {
	log.Info().Msg("Enabling encryption for user forges")
	userForges, err := wrapper.store.UserForgeListAll()
	if err != nil {
		return fmt.Errorf("failed to enable user forge encryption: %w", err)
	}
	for _, userForge := range userForges {
		if err := wrapper.encrypt(userForge); err != nil {
			return err
		}
		if err := wrapper._save(userForge); err != nil {
			return err
		}
	}
	log.Info().Msg("User forge encryption enabled successfully")
	return nil
}

func (wrapper *EncryptedUserForgeStore) MigrateEncryption(newEncryptionService types.EncryptionService) error {
	log.Info().Msg("Migrating user forge encryption")
	userForges, err := wrapper.store.UserForgeListAll()
	if err != nil {
		return fmt.Errorf("failed to migrate user forge encryption: %w", err)
	}
	if err := wrapper.decryptList(userForges); err != nil {
		return err
	}
	wrapper.encryption = newEncryptionService
	for _, userForge := range userForges {
		if err := wrapper.encrypt(userForge); err != nil {
			return err
		}
		if err := wrapper._save(userForge); err != nil {
			return err
		}
	}
	log.Info().Msg("User forge encryption migration completed successfully")
	return nil
}

func (wrapper *EncryptedUserForgeStore) encrypt(userForge *model.UserForge) error {
	associatedData := strconv.FormatInt(userForge.ID, 10)

	// Encrypt access token if present.
	if userForge.AccessToken != "" {
		encryptedAccessToken, err := wrapper.encryption.Encrypt(userForge.AccessToken, associatedData+":access")
		if err != nil {
			return fmt.Errorf("failed to encrypt user forge %d: %w", userForge.ID, err)
		}
		userForge.AccessToken = encryptedAccessToken
	}

	// Encrypt refresh token if present.
	if userForge.RefreshToken != "" {
		encryptedRefreshToken, err := wrapper.encryption.Encrypt(userForge.RefreshToken, associatedData+":refresh")
		if err != nil {
			return fmt.Errorf("failed to encrypt user forge %d: %w", userForge.ID, err)
		}
		userForge.RefreshToken = encryptedRefreshToken
	}

	return nil
}

func (wrapper *EncryptedUserForgeStore) decrypt(userForge *model.UserForge) error {
	associatedData := strconv.FormatInt(userForge.ID, 10)

	// Decrypt access token if present.
	if userForge.AccessToken != "" {
		decryptedAccessToken, err := wrapper.encryption.Decrypt(userForge.AccessToken, associatedData+":access")
		if err != nil {
			return fmt.Errorf("failed to decrypt user forge %d: %w", userForge.ID, err)
		}
		userForge.AccessToken = decryptedAccessToken
	}

	// Decrypt refresh token if present.
	if userForge.RefreshToken != "" {
		decryptedRefreshToken, err := wrapper.encryption.Decrypt(userForge.RefreshToken, associatedData+":refresh")
		if err != nil {
			return fmt.Errorf("failed to decrypt user forge %d: %w", userForge.ID, err)
		}
		userForge.RefreshToken = decryptedRefreshToken
	}

	return nil
}

func (wrapper *EncryptedUserForgeStore) decryptList(userForges []*model.UserForge) error {
	for _, userForge := range userForges {
		err := wrapper.decrypt(userForge)
		if err != nil {
			return fmt.Errorf("failed to decrypt user forge %d: %w", userForge.ID, err)
		}
	}
	return nil
}

func (wrapper *EncryptedUserForgeStore) _save(userForge *model.UserForge) error {
	err := wrapper.store.UserForgeUpdate(userForge)
	if err != nil {
		log.Err(err).Msg("failed to save user forge during encryption")
		return err
	}
	return nil
}

// UserForgeList returns all forge connections for a user.
func (wrapper *EncryptedUserForgeStore) UserForgeList(userID int64) ([]*model.UserForge, error) {
	userForges, err := wrapper.store.UserForgeList(userID)
	if err != nil {
		return nil, err
	}
	if wrapper.encryption != nil {
		if err := wrapper.decryptList(userForges); err != nil {
			return nil, err
		}
	}
	return userForges, nil
}

// UserForgeGet returns a user's forge connection for a specific forge.
func (wrapper *EncryptedUserForgeStore) UserForgeGet(userID, forgeID int64) (*model.UserForge, error) {
	userForge, err := wrapper.store.UserForgeGet(userID, forgeID)
	if err != nil {
		return nil, err
	}
	if wrapper.encryption != nil {
		if err := wrapper.decrypt(userForge); err != nil {
			return nil, err
		}
	}
	return userForge, nil
}

// UserForgeGetByRemoteID finds a user forge connection by forge ID and forge remote ID.
func (wrapper *EncryptedUserForgeStore) UserForgeGetByRemoteID(forgeID int64, remoteID model.ForgeRemoteID) (*model.UserForge, error) {
	userForge, err := wrapper.store.UserForgeGetByRemoteID(forgeID, remoteID)
	if err != nil {
		return nil, err
	}
	if wrapper.encryption != nil {
		if err := wrapper.decrypt(userForge); err != nil {
			return nil, err
		}
	}
	return userForge, nil
}

// UserForgeGetPrimary returns the user's primary forge connection.
func (wrapper *EncryptedUserForgeStore) UserForgeGetPrimary(userID int64) (*model.UserForge, error) {
	userForge, err := wrapper.store.UserForgeGetPrimary(userID)
	if err != nil {
		return nil, err
	}
	if wrapper.encryption != nil {
		if err := wrapper.decrypt(userForge); err != nil {
			return nil, err
		}
	}
	return userForge, nil
}

// UserForgeCreate creates a new user forge connection.
//
// Tokens must be encrypted *after* the row is inserted: the ID is part of the
// AEAD associated data, and an auto-increment ID is only assigned by the
// database during the insert. Encrypting before the insert would bake the
// placeholder ID `0` into the ciphertext, after which every subsequent
// UserForgeGet would fail to decrypt because the row's real ID no longer
// matches the AAD bound to the ciphertext. So we insert first, then encrypt
// and update, then decrypt back so the caller still sees plaintext.
func (wrapper *EncryptedUserForgeStore) UserForgeCreate(userForge *model.UserForge) error {
	if err := wrapper.store.UserForgeCreate(userForge); err != nil {
		return err
	}

	if wrapper.encryption == nil {
		return nil
	}

	if userForge.AccessToken == "" && userForge.RefreshToken == "" {
		return nil
	}

	if err := wrapper.encrypt(userForge); err != nil {
		return err
	}
	if err := wrapper.store.UserForgeUpdate(userForge); err != nil {
		return err
	}
	// Decrypt back so the caller continues to see plaintext.
	return wrapper.decrypt(userForge)
}

// UserForgeUpdate updates a user forge connection.
func (wrapper *EncryptedUserForgeStore) UserForgeUpdate(userForge *model.UserForge) error {
	if wrapper.encryption == nil {
		return wrapper.store.UserForgeUpdate(userForge)
	}

	if err := wrapper.encrypt(userForge); err != nil {
		return err
	}
	if err := wrapper.store.UserForgeUpdate(userForge); err != nil {
		return err
	}
	// Decrypt back so the caller continues to see plaintext.
	return wrapper.decrypt(userForge)
}

// UserForgeDelete deletes a user forge connection.
func (wrapper *EncryptedUserForgeStore) UserForgeDelete(userForge *model.UserForge) error {
	return wrapper.store.UserForgeDelete(userForge)
}

// UserForgeListAll returns all user forge connections (for encryption migration).
func (wrapper *EncryptedUserForgeStore) UserForgeListAll() ([]*model.UserForge, error) {
	return wrapper.store.UserForgeListAll()
}

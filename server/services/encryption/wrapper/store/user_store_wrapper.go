// Copyright 2025 Crow Authors
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

	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/server/services/encryption/types"
	"codeberg.org/crowci/crow/v5/server/store"
)

type EncryptedUserStore struct {
	store      store.Store
	encryption types.EncryptionService
}

func NewUserStore(userStore store.Store) *EncryptedUserStore {
	wrapper := EncryptedUserStore{userStore, nil}
	return &wrapper
}

func (wrapper *EncryptedUserStore) SetEncryptionService(service types.EncryptionService) error {
	if wrapper.encryption != nil {
		return errors.New(errMessageInitSeveralTimes)
	}
	wrapper.encryption = service
	return nil
}

func (wrapper *EncryptedUserStore) EnableEncryption() error {
	log.Info().Msg(logMessageEnablingUsersEncryption)
	users, err := wrapper.store.UserListAll()
	if err != nil {
		return fmt.Errorf(errMessageTemplateFailedToEnableUser, err)
	}
	for _, user := range users {
		if err := wrapper.encrypt(user); err != nil {
			return err
		}
		if err := wrapper._save(user); err != nil {
			return err
		}
	}
	log.Info().Msg(logMessageEnablingUsersEncryptionSuccess)
	return nil
}

func (wrapper *EncryptedUserStore) MigrateEncryption(newEncryptionService types.EncryptionService) error {
	log.Info().Msg(logMessageMigratingUsersEncryption)
	users, err := wrapper.store.UserListAll()
	if err != nil {
		return fmt.Errorf(errMessageTemplateFailedToMigrateUser, err)
	}
	if err := wrapper.decryptList(users); err != nil {
		return err
	}
	wrapper.encryption = newEncryptionService
	for _, user := range users {
		if err := wrapper.encrypt(user); err != nil {
			return err
		}
		if err := wrapper._save(user); err != nil {
			return err
		}
	}
	log.Info().Msg(logMessageMigratingUsersEncryptionSuccess)
	return nil
}

func (wrapper *EncryptedUserStore) encrypt(user *model.User) error {
	associatedData := strconv.FormatInt(user.ID, 10)

	// Encrypt access token if present.
	if user.AccessToken != "" {
		encryptedAccessToken, err := wrapper.encryption.Encrypt(user.AccessToken, associatedData+":access")
		if err != nil {
			return fmt.Errorf(errMessageTemplateFailedToEncryptUser, user.ID, err)
		}
		user.AccessToken = encryptedAccessToken
	}

	// Encrypt refresh token if present.
	if user.RefreshToken != "" {
		encryptedRefreshToken, err := wrapper.encryption.Encrypt(user.RefreshToken, associatedData+":refresh")
		if err != nil {
			return fmt.Errorf(errMessageTemplateFailedToEncryptUser, user.ID, err)
		}
		user.RefreshToken = encryptedRefreshToken
	}

	return nil
}

func (wrapper *EncryptedUserStore) decrypt(user *model.User) error {
	associatedData := strconv.FormatInt(user.ID, 10)

	// Decrypt access token if present.
	if user.AccessToken != "" {
		decryptedAccessToken, err := wrapper.encryption.Decrypt(user.AccessToken, associatedData+":access")
		if err != nil {
			return fmt.Errorf(errMessageTemplateFailedToDecryptUser, user.ID, err)
		}
		user.AccessToken = decryptedAccessToken
	}

	// Decrypt refresh token if present.
	if user.RefreshToken != "" {
		decryptedRefreshToken, err := wrapper.encryption.Decrypt(user.RefreshToken, associatedData+":refresh")
		if err != nil {
			return fmt.Errorf(errMessageTemplateFailedToDecryptUser, user.ID, err)
		}
		user.RefreshToken = decryptedRefreshToken
	}

	return nil
}

func (wrapper *EncryptedUserStore) decryptList(users []*model.User) error {
	for _, user := range users {
		err := wrapper.decrypt(user)
		if err != nil {
			return fmt.Errorf(errMessageTemplateFailedToDecryptUser, user.ID, err)
		}
	}
	return nil
}

func (wrapper *EncryptedUserStore) _save(user *model.User) error {
	err := wrapper.store.UpdateUser(user)
	if err != nil {
		log.Err(err).Msg(errMessageTemplateUserStorageError)
		return err
	}
	return nil
}

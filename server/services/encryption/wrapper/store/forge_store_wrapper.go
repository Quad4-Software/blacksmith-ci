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

	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/server/services/encryption/types"
	"codefloe.com/crowci/crow/v5/server/store"
)

type EncryptedForgeStore struct {
	store      store.Store
	encryption types.EncryptionService
}

func NewForgeStore(forgeStore store.Store) *EncryptedForgeStore {
	wrapper := EncryptedForgeStore{forgeStore, nil}
	return &wrapper
}

func (wrapper *EncryptedForgeStore) SetEncryptionService(service types.EncryptionService) error {
	if wrapper.encryption != nil {
		return errors.New(errMessageInitSeveralTimes)
	}
	wrapper.encryption = service
	return nil
}

func (wrapper *EncryptedForgeStore) EnableEncryption() error {
	log.Info().Msg(logMessageEnablingForgesEncryption)
	forges, err := wrapper.store.ForgeListAll()
	if err != nil {
		return fmt.Errorf(errMessageTemplateFailedToEnableForge, err)
	}
	for _, forge := range forges {
		if err := wrapper.encrypt(forge); err != nil {
			return err
		}
		if err := wrapper._save(forge); err != nil {
			return err
		}
	}
	log.Info().Msg(logMessageEnablingForgesEncryptionSuccess)
	return nil
}

func (wrapper *EncryptedForgeStore) MigrateEncryption(newEncryptionService types.EncryptionService) error {
	log.Info().Msg(logMessageMigratingForgesEncryption)
	forges, err := wrapper.store.ForgeListAll()
	if err != nil {
		return fmt.Errorf(errMessageTemplateFailedToMigrateForge, err)
	}
	if err := wrapper.decryptList(forges); err != nil {
		return err
	}
	wrapper.encryption = newEncryptionService
	for _, forge := range forges {
		if err := wrapper.encrypt(forge); err != nil {
			return err
		}
		if err := wrapper._save(forge); err != nil {
			return err
		}
	}
	log.Info().Msg(logMessageMigratingForgesEncryptionSuccess)
	return nil
}

func (wrapper *EncryptedForgeStore) encrypt(forge *model.Forge) error {
	// Only encrypt if there's a client secret.
	if forge.ClientSecret == "" {
		return nil
	}

	encryptedValue, err := wrapper.encryption.Encrypt(forge.ClientSecret, strconv.FormatInt(forge.ID, 10))
	if err != nil {
		return fmt.Errorf(errMessageTemplateFailedToEncryptForge, forge.ID, err)
	}
	forge.ClientSecret = encryptedValue
	return nil
}

func (wrapper *EncryptedForgeStore) decrypt(forge *model.Forge) error {
	// Only decrypt if there's a client secret.
	if forge.ClientSecret == "" {
		return nil
	}

	decryptedValue, err := wrapper.encryption.Decrypt(forge.ClientSecret, strconv.FormatInt(forge.ID, 10))
	if err != nil {
		return fmt.Errorf(errMessageTemplateFailedToDecryptForge, forge.ID, err)
	}
	forge.ClientSecret = decryptedValue
	return nil
}

func (wrapper *EncryptedForgeStore) decryptList(forges []*model.Forge) error {
	for _, forge := range forges {
		err := wrapper.decrypt(forge)
		if err != nil {
			return fmt.Errorf(errMessageTemplateFailedToDecryptForge, forge.ID, err)
		}
	}
	return nil
}

func (wrapper *EncryptedForgeStore) _save(forge *model.Forge) error {
	err := wrapper.store.ForgeUpdate(forge)
	if err != nil {
		log.Err(err).Msg(errMessageTemplateForgeStorageError)
		return err
	}
	return nil
}

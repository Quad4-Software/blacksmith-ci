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

type EncryptedRegistryStore struct {
	store      store.Store
	encryption types.EncryptionService
}

func NewRegistryStore(registryStore store.Store) *EncryptedRegistryStore {
	wrapper := EncryptedRegistryStore{registryStore, nil}
	return &wrapper
}

func (wrapper *EncryptedRegistryStore) SetEncryptionService(service types.EncryptionService) error {
	if wrapper.encryption != nil {
		return errors.New(errMessageInitSeveralTimes)
	}
	wrapper.encryption = service
	return nil
}

func (wrapper *EncryptedRegistryStore) EnableEncryption() error {
	log.Info().Msg(logMessageEnablingRegistriesEncryption)
	registries, err := wrapper.store.RegistryListAll()
	if err != nil {
		return fmt.Errorf(errMessageTemplateFailedToEnableRegistry, err)
	}
	for _, registry := range registries {
		if err := wrapper.encrypt(registry); err != nil {
			return err
		}
		if err := wrapper._save(registry); err != nil {
			return err
		}
	}
	log.Info().Msg(logMessageEnablingRegistriesEncryptionSuccess)
	return nil
}

func (wrapper *EncryptedRegistryStore) MigrateEncryption(newEncryptionService types.EncryptionService) error {
	log.Info().Msg(logMessageMigratingRegistriesEncryption)
	registries, err := wrapper.store.RegistryListAll()
	if err != nil {
		return fmt.Errorf(errMessageTemplateFailedToMigrateRegistry, err)
	}
	if err := wrapper.decryptList(registries); err != nil {
		return err
	}
	wrapper.encryption = newEncryptionService
	for _, registry := range registries {
		if err := wrapper.encrypt(registry); err != nil {
			return err
		}
		if err := wrapper._save(registry); err != nil {
			return err
		}
	}
	log.Info().Msg(logMessageMigratingRegistriesEncryptionSuccess)
	return nil
}

func (wrapper *EncryptedRegistryStore) encrypt(registry *model.Registry) error {
	encryptedValue, err := wrapper.encryption.Encrypt(registry.Password, strconv.FormatInt(registry.ID, 10))
	if err != nil {
		return fmt.Errorf(errMessageTemplateFailedToEncryptRegistry, registry.ID, err)
	}
	registry.Password = encryptedValue
	return nil
}

func (wrapper *EncryptedRegistryStore) decrypt(registry *model.Registry) error {
	decryptedValue, err := wrapper.encryption.Decrypt(registry.Password, strconv.FormatInt(registry.ID, 10))
	if err != nil {
		return fmt.Errorf(errMessageTemplateFailedToDecryptRegistry, registry.ID, err)
	}
	registry.Password = decryptedValue
	return nil
}

func (wrapper *EncryptedRegistryStore) decryptList(registries []*model.Registry) error {
	for _, registry := range registries {
		err := wrapper.decrypt(registry)
		if err != nil {
			return fmt.Errorf(errMessageTemplateFailedToDecryptRegistry, registry.ID, err)
		}
	}
	return nil
}

func (wrapper *EncryptedRegistryStore) _save(registry *model.Registry) error {
	err := wrapper.store.RegistryUpdate(registry)
	if err != nil {
		log.Err(err).Msg(errMessageTemplateRegistryStorageError)
		return err
	}
	return nil
}

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
	"codefloe.com/crowci/crow/v6/server/model"
)

func (wrapper *EncryptedForgeStore) ForgeCreate(forge *model.Forge) error {
	// Create forge first to get ID.
	err := wrapper.store.ForgeCreate(forge)
	if err != nil {
		return err
	}

	// Encrypt client secret after forge is created (has ID).
	if forge.ClientSecret != "" {
		err = wrapper.encrypt(forge)
		if err != nil {
			return err
		}
		err = wrapper.store.ForgeUpdate(forge)
		if err != nil {
			return err
		}
		// Decrypt back to return plaintext to caller.
		err = wrapper.decrypt(forge)
		if err != nil {
			return err
		}
	}

	return nil
}

func (wrapper *EncryptedForgeStore) ForgeGet(id int64) (*model.Forge, error) {
	result, err := wrapper.store.ForgeGet(id)
	if err != nil {
		return nil, err
	}
	err = wrapper.decrypt(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (wrapper *EncryptedForgeStore) ForgeList(p *model.ListOptions) ([]*model.Forge, error) {
	results, err := wrapper.store.ForgeList(p)
	if err != nil {
		return nil, err
	}
	err = wrapper.decryptList(results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (wrapper *EncryptedForgeStore) ForgeListAll() ([]*model.Forge, error) {
	results, err := wrapper.store.ForgeListAll()
	if err != nil {
		return nil, err
	}
	err = wrapper.decryptList(results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (wrapper *EncryptedForgeStore) ForgeUpdate(forge *model.Forge) error {
	err := wrapper.encrypt(forge)
	if err != nil {
		return err
	}

	err = wrapper.store.ForgeUpdate(forge)
	if err != nil {
		return err
	}

	err = wrapper.decrypt(forge)
	if err != nil {
		return err
	}
	return nil
}

func (wrapper *EncryptedForgeStore) ForgeDelete(forge *model.Forge) error {
	return wrapper.store.ForgeDelete(forge)
}

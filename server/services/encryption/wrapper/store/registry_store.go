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
	"fmt"

	"codefloe.com/crowci/crow/v6/server/model"
)

func (wrapper *EncryptedRegistryStore) RegistryFind(repo *model.Repo, addr string) (*model.Registry, error) {
	result, err := wrapper.store.RegistryFind(repo, addr)
	if err != nil {
		return nil, err
	}
	err = wrapper.decrypt(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (wrapper *EncryptedRegistryStore) RegistryList(repo *model.Repo, includeGlobalAndOrg bool, p *model.ListOptions) ([]*model.Registry, error) {
	results, err := wrapper.store.RegistryList(repo, includeGlobalAndOrg, p)
	if err != nil {
		return nil, err
	}
	err = wrapper.decryptList(results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (wrapper *EncryptedRegistryStore) RegistryListAll() ([]*model.Registry, error) {
	results, err := wrapper.store.RegistryListAll()
	if err != nil {
		return nil, err
	}
	err = wrapper.decryptList(results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (wrapper *EncryptedRegistryStore) RegistryCreate(registry *model.Registry) error {
	// Create registry with actual data first to get ID.
	err := wrapper.store.RegistryCreate(registry)
	if err != nil {
		return err
	}

	// Encrypt password after registry is created (has ID for associated data).
	if registry.Password != "" {
		err = wrapper.encrypt(registry)
		if err != nil {
			deleteErr := wrapper.store.RegistryDelete(registry)
			if deleteErr != nil {
				return fmt.Errorf(errMessageTemplateFailedToRollbackRegistryCreation, err, deleteErr.Error())
			}
			return err
		}
		err = wrapper.store.RegistryUpdate(registry)
		if err != nil {
			deleteErr := wrapper.store.RegistryDelete(registry)
			if deleteErr != nil {
				return fmt.Errorf(errMessageTemplateFailedToRollbackRegistryCreation, err, deleteErr.Error())
			}
			return err
		}
		// Decrypt back to return plaintext to caller.
		err = wrapper.decrypt(registry)
		if err != nil {
			return err
		}
	}

	return nil
}

func (wrapper *EncryptedRegistryStore) RegistryUpdate(registry *model.Registry) error {
	err := wrapper.encrypt(registry)
	if err != nil {
		return err
	}

	err = wrapper.store.RegistryUpdate(registry)
	if err != nil {
		return err
	}

	err = wrapper.decrypt(registry)
	if err != nil {
		return err
	}
	return nil
}

func (wrapper *EncryptedRegistryStore) RegistryDelete(registry *model.Registry) error {
	return wrapper.store.RegistryDelete(registry)
}

func (wrapper *EncryptedRegistryStore) OrgRegistryFind(orgID int64, name string) (*model.Registry, error) {
	result, err := wrapper.store.OrgRegistryFind(orgID, name)
	if err != nil {
		return nil, err
	}

	err = wrapper.decrypt(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (wrapper *EncryptedRegistryStore) OrgRegistryList(orgID int64, p *model.ListOptions) ([]*model.Registry, error) {
	results, err := wrapper.store.OrgRegistryList(orgID, p)
	if err != nil {
		return nil, err
	}

	err = wrapper.decryptList(results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (wrapper *EncryptedRegistryStore) GlobalRegistryFind(name string) (*model.Registry, error) {
	result, err := wrapper.store.GlobalRegistryFind(name)
	if err != nil {
		return nil, err
	}

	err = wrapper.decrypt(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (wrapper *EncryptedRegistryStore) GlobalRegistryList(p *model.ListOptions) ([]*model.Registry, error) {
	results, err := wrapper.store.GlobalRegistryList(p)
	if err != nil {
		return nil, err
	}

	err = wrapper.decryptList(results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (wrapper *EncryptedRegistryStore) UserRegistryList(orgIDs []int64, includeGlobal bool, searchTerm string, p *model.ListOptions) ([]*model.Registry, error) {
	results, err := wrapper.store.UserRegistryList(orgIDs, includeGlobal, searchTerm, p)
	if err != nil {
		return nil, err
	}

	err = wrapper.decryptList(results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

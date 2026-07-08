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

func (wrapper *EncryptedUserStore) GetUser(id int64) (*model.User, error) {
	result, err := wrapper.store.GetUser(id)
	if err != nil {
		return nil, err
	}
	err = wrapper.decrypt(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (wrapper *EncryptedUserStore) GetUserRemoteID(remoteID model.ForgeRemoteID, login string) (*model.User, error) {
	result, err := wrapper.store.GetUserRemoteID(remoteID, login)
	if err != nil {
		return nil, err
	}
	err = wrapper.decrypt(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (wrapper *EncryptedUserStore) GetUserLogin(login string) (*model.User, error) {
	result, err := wrapper.store.GetUserLogin(login)
	if err != nil {
		return nil, err
	}
	err = wrapper.decrypt(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (wrapper *EncryptedUserStore) GetUserList(p *model.ListOptions) ([]*model.User, error) {
	results, err := wrapper.store.GetUserList(p)
	if err != nil {
		return nil, err
	}
	err = wrapper.decryptList(results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (wrapper *EncryptedUserStore) GetUserCount() (int64, error) {
	return wrapper.store.GetUserCount()
}

func (wrapper *EncryptedUserStore) CreateUser(user *model.User) error {
	// For user creation, we need to encrypt tokens before creating.
	// Unlike secrets/registries, users get their ID from the org creation in the same transaction.
	// So we create first, then encrypt and update.
	err := wrapper.store.CreateUser(user)
	if err != nil {
		return err
	}

	// Encrypt tokens after user is created (has ID).
	if user.AccessToken != "" || user.RefreshToken != "" {
		err = wrapper.encrypt(user)
		if err != nil {
			return err
		}
		err = wrapper.store.UpdateUser(user)
		if err != nil {
			return err
		}
		// Decrypt back to return plaintext to caller.
		err = wrapper.decrypt(user)
		if err != nil {
			return err
		}
	}

	return nil
}

func (wrapper *EncryptedUserStore) UpdateUser(user *model.User) error {
	err := wrapper.encrypt(user)
	if err != nil {
		return err
	}

	err = wrapper.store.UpdateUser(user)
	if err != nil {
		return err
	}

	err = wrapper.decrypt(user)
	if err != nil {
		return err
	}
	return nil
}

func (wrapper *EncryptedUserStore) DeleteUser(user *model.User) error {
	return wrapper.store.DeleteUser(user)
}

func (wrapper *EncryptedUserStore) UserListAll() ([]*model.User, error) {
	results, err := wrapper.store.UserListAll()
	if err != nil {
		return nil, err
	}
	err = wrapper.decryptList(results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

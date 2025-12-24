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

package encryption

import (
	"errors"
	"fmt"

	"github.com/urfave/cli/v3"

	"codeberg.org/crowci/crow/v5/server/services/encryption/types"
	"codeberg.org/crowci/crow/v5/server/store"
)

type tinkConfiguration struct {
	keysetFilePath string
	store          store.Store
	clients        []types.EncryptionClient
}

func newTink(c *cli.Command, s store.Store) types.EncryptionServiceBuilder {
	filepath := c.String(tinkKeysetFilepathConfigFlag)
	return &tinkConfiguration{filepath, s, nil}
}

func (c tinkConfiguration) WithClients(clients []types.EncryptionClient) types.EncryptionServiceBuilder {
	c.clients = clients
	return c
}

func (c tinkConfiguration) Build() (types.EncryptionService, error) {
	svc := &tinkEncryptionService{
		keysetFilePath:    c.keysetFilePath,
		primaryKeyID:      "",
		encryption:        nil,
		store:             c.store,
		keysetFileWatcher: nil,
		clients:           c.clients,
	}

	if err := svc.initClients(); err != nil {
		return nil, fmt.Errorf(errTemplateFailedInitializingClients, err)
	}

	if err := svc.loadKeyset(); err != nil {
		return nil, fmt.Errorf(errTemplateTinkFailedLoadingKeyset, err)
	}

	err := svc.validateKeyset()
	switch {
	case errors.Is(err, errEncryptionNotEnabled):
		err = svc.enable()
	case errors.Is(err, errEncryptionKeyRotated):
		err = svc.rotate()
	case err == nil:
		// Keyset validated successfully, but check if actual data is encrypted.
		// This handles the case where encryption was configured but data wasn't encrypted
		// (e.g., due to a failed/interrupted previous encryption attempt).
		if checkErr := svc.checkPlaintextData(); errors.Is(checkErr, errEncryptionNotEnabled) {
			err = svc.enable()
		} else if checkErr != nil {
			err = checkErr
		}
	}

	if err != nil {
		return nil, fmt.Errorf(errTemplateTinkFailedValidatingKeyset, err)
	}

	if err := svc.initFileWatcher(); err != nil {
		return nil, fmt.Errorf(errTemplateTinkFailedInitializeFileWatcher, err)
	}
	return svc, nil
}

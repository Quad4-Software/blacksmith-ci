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
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/rs/zerolog/log"
	"github.com/tink-crypto/tink-go/v2/aead"
	insecure_clear_text_keyset "github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyset"

	"codefloe.com/crowci/crow/v5/server/store/types"
)

func (svc *tinkEncryptionService) loadKeyset() error {
	log.Info().Msgf(logTemplateTinkLoadingKeyset, svc.keysetFilePath)
	file, err := os.Open(svc.keysetFilePath)
	if err != nil {
		return fmt.Errorf(errTemplateTinkFailedOpeningKeyset, err)
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			log.Error().Err(err).Msgf(logTemplateTinkFailedClosingKeysetFile, svc.keysetFilePath)
		}
	}(file)

	jsonKeyset := keyset.NewJSONReader(file)
	keysetHandle, err := insecure_clear_text_keyset.Read(jsonKeyset)
	if err != nil {
		return fmt.Errorf(errTemplateTinkFailedReadingKeyset, err)
	}
	svc.primaryKeyID = strconv.FormatUint(uint64(keysetHandle.KeysetInfo().PrimaryKeyId), 10)

	encryptionInstance, err := aead.New(keysetHandle)
	if err != nil {
		return fmt.Errorf(errTemplateTinkFailedInitializingAEAD, err)
	}
	svc.encryption = encryptionInstance
	return nil
}

func (svc *tinkEncryptionService) validateKeyset() error {
	ciphertextSample, err := svc.store.ServerConfigGet(ciphertextSampleConfigKey)
	if errors.Is(err, types.RecordNotExist) {
		return errEncryptionNotEnabled
	} else if err != nil {
		return fmt.Errorf(errTemplateFailedLoadingServerConfig, err)
	}

	plaintext, err := svc.Decrypt(ciphertextSample, keyIDAssociatedData)
	if plaintext != svc.primaryKeyID {
		return errEncryptionKeyRotated
	} else if err != nil {
		return fmt.Errorf(errTemplateFailedValidatingKey, err)
	}
	return nil
}

// checkPlaintextData checks if any data in the database is still plaintext
// (not encrypted). This can happen when encryption was previously configured
// but the actual data encryption failed or was interrupted.
// Returns errEncryptionNotEnabled if plaintext data is found that needs encryption.
func (svc *tinkEncryptionService) checkPlaintextData() error {
	// Check secrets
	if needsEncryption, err := svc.checkSecretsForPlaintext(); err != nil {
		return err
	} else if needsEncryption {
		return errEncryptionNotEnabled
	}

	// Check registries
	if needsEncryption, err := svc.checkRegistriesForPlaintext(); err != nil {
		return err
	} else if needsEncryption {
		return errEncryptionNotEnabled
	}

	// Check users
	if needsEncryption, err := svc.checkUsersForPlaintext(); err != nil {
		return err
	} else if needsEncryption {
		return errEncryptionNotEnabled
	}

	// Check forges
	if needsEncryption, err := svc.checkForgesForPlaintext(); err != nil {
		return err
	} else if needsEncryption {
		return errEncryptionNotEnabled
	}

	log.Info().Msg("all existing data is properly encrypted")
	return nil
}

func (svc *tinkEncryptionService) checkSecretsForPlaintext() (bool, error) {
	secrets, err := svc.store.SecretListAll()
	if err != nil {
		return false, fmt.Errorf("failed to check secrets for plaintext data: %w", err)
	}

	for _, secret := range secrets {
		if secret.Value == "" {
			continue
		}
		// Try to base64 decode - if it fails, the value is plaintext
		if _, err := base64.StdEncoding.DecodeString(secret.Value); err != nil {
			log.Warn().Msgf("detected plaintext secret (id=%d), encryption will be triggered", secret.ID)
			return true, nil
		}
		// Also verify it can actually be decrypted
		if _, err := svc.Decrypt(secret.Value, strconv.Itoa(int(secret.ID))); err != nil {
			log.Warn().Msgf("detected unencrypted secret (id=%d), encryption will be triggered", secret.ID)
			return true, nil
		}
	}
	return false, nil
}

func (svc *tinkEncryptionService) checkRegistriesForPlaintext() (bool, error) {
	registries, err := svc.store.RegistryListAll()
	if err != nil {
		return false, fmt.Errorf("failed to check registries for plaintext data: %w", err)
	}

	for _, registry := range registries {
		if registry.Password == "" {
			continue
		}
		// Try to base64 decode - if it fails, the value is plaintext
		if _, err := base64.StdEncoding.DecodeString(registry.Password); err != nil {
			log.Warn().Msgf("detected plaintext registry (id=%d), encryption will be triggered", registry.ID)
			return true, nil
		}
		// Also verify it can actually be decrypted
		if _, err := svc.Decrypt(registry.Password, strconv.FormatInt(registry.ID, 10)); err != nil {
			log.Warn().Msgf("detected unencrypted registry (id=%d), encryption will be triggered", registry.ID)
			return true, nil
		}
	}
	return false, nil
}

func (svc *tinkEncryptionService) checkUsersForPlaintext() (bool, error) {
	users, err := svc.store.UserListAll()
	if err != nil {
		return false, fmt.Errorf("failed to check users for plaintext data: %w", err)
	}

	for _, user := range users {
		associatedData := strconv.FormatInt(user.ID, 10)

		// Check access token (using deprecated fields for backward compatibility)
		accessToken := user.AccessToken
		if accessToken != "" {
			if _, err := base64.StdEncoding.DecodeString(accessToken); err != nil {
				log.Warn().Msgf("detected plaintext user access token (id=%d), encryption will be triggered", user.ID)
				return true, nil
			}
			if _, err := svc.Decrypt(accessToken, associatedData+":access"); err != nil {
				log.Warn().Msgf("detected unencrypted user access token (id=%d), encryption will be triggered", user.ID)
				return true, nil
			}
		}

		// Check refresh token (using deprecated fields for backward compatibility)
		refreshToken := user.RefreshToken
		if refreshToken != "" {
			if _, err := base64.StdEncoding.DecodeString(refreshToken); err != nil {
				log.Warn().Msgf("detected plaintext user refresh token (id=%d), encryption will be triggered", user.ID)
				return true, nil
			}
			if _, err := svc.Decrypt(refreshToken, associatedData+":refresh"); err != nil {
				log.Warn().Msgf("detected unencrypted user refresh token (id=%d), encryption will be triggered", user.ID)
				return true, nil
			}
		}
	}
	return false, nil
}

func (svc *tinkEncryptionService) checkForgesForPlaintext() (bool, error) {
	forges, err := svc.store.ForgeListAll()
	if err != nil {
		return false, fmt.Errorf("failed to check forges for plaintext data: %w", err)
	}

	for _, forge := range forges {
		if forge.ClientSecret == "" {
			continue
		}
		// Try to base64 decode - if it fails, the value is plaintext
		if _, err := base64.StdEncoding.DecodeString(forge.ClientSecret); err != nil {
			log.Warn().Msgf("detected plaintext forge client secret (id=%d), encryption will be triggered", forge.ID)
			return true, nil
		}
		// Also verify it can actually be decrypted
		if _, err := svc.Decrypt(forge.ClientSecret, strconv.FormatInt(forge.ID, 10)); err != nil {
			log.Warn().Msgf("detected unencrypted forge client secret (id=%d), encryption will be triggered", forge.ID)
			return true, nil
		}
	}
	return false, nil
}

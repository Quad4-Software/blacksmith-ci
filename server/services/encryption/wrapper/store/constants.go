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

package store

const (
	// Secret encryption messages.
	errMessageTemplateFailedToEnable        = "failed enabling secret store encryption: %w"
	errMessageTemplateFailedToMigrate       = "failed migrating secret store encryption: %w"
	errMessageTemplateFailedToEncryptSecret = "failed to encrypt secret id=%d: %w"
	errMessageTemplateFailedToDecryptSecret = "failed to decrypt secret id=%d: %w"
	errMessageTemplateStorageError          = "Storage error: could not update secret in DB"

	errMessageTemplateFailedToRollbackSecretCreation = "failed creating secret: %w. Also failed deleting temporary secret record from store: %s"

	errMessageInitSeveralTimes = "attempt to init encrypted storage more than once"

	logMessageEnablingSecretsEncryption         = "Encrypting all secrets in database"
	logMessageEnablingSecretsEncryptionSuccess  = "All secrets are encrypted"
	logMessageMigratingSecretsEncryption        = "Migrating encryption keys"
	logMessageMigratingSecretsEncryptionSuccess = "Secrets encryption migrated successfully"

	// Registry encryption messages.
	errMessageTemplateFailedToEnableRegistry           = "failed enabling registry store encryption: %w"
	errMessageTemplateFailedToMigrateRegistry          = "failed migrating registry store encryption: %w"
	errMessageTemplateFailedToEncryptRegistry          = "failed to encrypt registry id=%d: %w"
	errMessageTemplateFailedToDecryptRegistry          = "failed to decrypt registry id=%d: %w"
	errMessageTemplateRegistryStorageError             = "Storage error: could not update registry in DB"
	errMessageTemplateFailedToRollbackRegistryCreation = "failed creating registry: %w. Also failed deleting temporary registry record from store: %s"

	logMessageEnablingRegistriesEncryption         = "Encrypting all registry passwords in database"
	logMessageEnablingRegistriesEncryptionSuccess  = "All registry passwords are encrypted"
	logMessageMigratingRegistriesEncryption        = "Migrating registry encryption keys"
	logMessageMigratingRegistriesEncryptionSuccess = "Registry encryption migrated successfully"

	// User encryption messages.
	errMessageTemplateFailedToEnableUser  = "failed enabling user store encryption: %w"
	errMessageTemplateFailedToMigrateUser = "failed migrating user store encryption: %w"
	errMessageTemplateFailedToEncryptUser = "failed to encrypt user id=%d: %w"
	errMessageTemplateFailedToDecryptUser = "failed to decrypt user id=%d: %w"
	errMessageTemplateUserStorageError    = "Storage error: could not update user in DB"

	logMessageEnablingUsersEncryption         = "Encrypting all user OAuth tokens in database"
	logMessageEnablingUsersEncryptionSuccess  = "All user OAuth tokens are encrypted"
	logMessageMigratingUsersEncryption        = "Migrating user encryption keys"
	logMessageMigratingUsersEncryptionSuccess = "User encryption migrated successfully"

	// Forge encryption messages.
	errMessageTemplateFailedToEnableForge  = "failed enabling forge store encryption: %w"
	errMessageTemplateFailedToMigrateForge = "failed migrating forge store encryption: %w"
	errMessageTemplateFailedToEncryptForge = "failed to encrypt forge id=%d: %w"
	errMessageTemplateFailedToDecryptForge = "failed to decrypt forge id=%d: %w"
	errMessageTemplateForgeStorageError    = "Storage error: could not update forge in DB"

	logMessageEnablingForgesEncryption         = "Encrypting all forge client secrets in database"
	logMessageEnablingForgesEncryptionSuccess  = "All forge client secrets are encrypted"
	logMessageMigratingForgesEncryption        = "Migrating forge encryption keys"
	logMessageMigratingForgesEncryptionSuccess = "Forge encryption migrated successfully"
)

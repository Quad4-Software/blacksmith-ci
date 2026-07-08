// Copyright 2024 Woodpecker Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package services

import (
	"crypto"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/server/forge"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/services/config"
	"codefloe.com/crowci/crow/v6/server/services/environment"
	"codefloe.com/crowci/crow/v6/server/services/registry"
	"codefloe.com/crowci/crow/v6/server/services/secret"
	"codefloe.com/crowci/crow/v6/server/store"
)

const forgeCacheTTL = 10 * time.Minute

type SetupForge func(forge *model.Forge) (forge.Forge, error)

type Manager interface {
	SignaturePublicKey() crypto.PublicKey
	SecretServiceFromRepo(repo *model.Repo) secret.Service
	SecretService() secret.Service
	RegistryServiceFromRepo(repo *model.Repo) registry.Service
	RegistryService() registry.Service
	ConfigServiceFromRepo(repo *model.Repo) config.Service
	EnvironmentService() environment.Service
	ForgeFromRepo(repo *model.Repo) (forge.Forge, error)
	ForgeFromUser(user *model.User) (forge.Forge, error)
	ForgeByID(forgeID int64) (forge.Forge, error)
	// Store returns the store used by the manager. This is the encrypted store
	// when encryption is configured, otherwise the base store.
	Store() store.Store
}

type manager struct {
	signaturePrivateKey crypto.PrivateKey
	signaturePublicKey  crypto.PublicKey
	store               store.Store
	secret              secret.Service
	registry            registry.Service
	config              config.Service
	environment         environment.Service
	forgeCache          *ttlcache.Cache[int64, forge.Forge]
	setupForge          SetupForge
}

func NewManager(c *cli.Command, baseStore store.Store, setupForge SetupForge) (Manager, error) {
	// Setup encrypted store wrapper for secrets, registries, users, and forges.
	// This is a no-op if encryption is not configured.
	encryptedStore, err := setupEncryptedStore(c, baseStore)
	if err != nil {
		return nil, err
	}

	signaturePrivateKey, signaturePublicKey, err := setupSignatureKeys(encryptedStore)
	if err != nil {
		return nil, err
	}

	err = setupForgeService(c, encryptedStore)
	if err != nil {
		return nil, err
	}

	configService, err := setupConfigService(c, signaturePrivateKey)
	if err != nil {
		return nil, err
	}

	return &manager{
		signaturePrivateKey: signaturePrivateKey,
		signaturePublicKey:  signaturePublicKey,
		store:               encryptedStore,
		secret:              setupSecretService(encryptedStore),
		registry:            setupRegistryService(encryptedStore, c.String("docker-config")),
		config:              configService,
		environment:         environment.Parse(c.StringSlice("environment")),
		forgeCache:          ttlcache.New(ttlcache.WithDisableTouchOnHit[int64, forge.Forge]()),
		setupForge:          setupForge,
	}, nil
}

func (m *manager) SignaturePublicKey() crypto.PublicKey {
	return m.signaturePublicKey
}

func (m *manager) Store() store.Store {
	return m.store
}

func (m *manager) SecretServiceFromRepo(_ *model.Repo) secret.Service {
	return m.SecretService()
}

func (m *manager) SecretService() secret.Service {
	return m.secret
}

func (m *manager) RegistryServiceFromRepo(_ *model.Repo) registry.Service {
	return m.RegistryService()
}

func (m *manager) RegistryService() registry.Service {
	return m.registry
}

func (m *manager) ConfigServiceFromRepo(_ *model.Repo) config.Service {
	// TODO: decide based on repo property which config service to use
	return m.config
}

func (m *manager) EnvironmentService() environment.Service {
	return m.environment
}

func (m *manager) ForgeFromRepo(repo *model.Repo) (forge.Forge, error) {
	return m.ForgeByID(repo.ForgeID)
}

func (m *manager) ForgeFromUser(user *model.User) (forge.Forge, error) {
	return m.ForgeByID(user.ForgeID)
}

func (m *manager) ForgeByID(id int64) (forge.Forge, error) {
	item := m.forgeCache.Get(id)
	if item != nil && !item.IsExpired() {
		return item.Value(), nil
	}

	forgeModel, err := m.store.ForgeGet(id)
	if err != nil {
		return nil, err
	}

	forge, err := m.setupForge(forgeModel)
	if err != nil {
		return nil, err
	}

	m.forgeCache.Set(id, forge, forgeCacheTTL)

	return forge, nil
}

// Copyright 2026 Crow Authors
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

package integrations

import (
	"fmt"

	"codefloe.com/crowci/crow/v5/server/model"
)

// DefaultFactory is the standard provider factory implementation.
type DefaultFactory struct {
	// Factory functions are injected to avoid import cycles
	vaultFactory         func(*model.Integration) (Provider, error)
	azureKeyVaultFactory func(*model.Integration) (Provider, error)
	infisicalFactory     func(*model.Integration) (Provider, error)
}

// NewDefaultFactory creates a new DefaultFactory.
// Factory functions should be provided by the respective provider packages.
func NewDefaultFactory(
	vaultFactory func(*model.Integration) (Provider, error),
	azureKeyVaultFactory func(*model.Integration) (Provider, error),
	infisicalFactory func(*model.Integration) (Provider, error),
) *DefaultFactory {
	return &DefaultFactory{
		vaultFactory:         vaultFactory,
		azureKeyVaultFactory: azureKeyVaultFactory,
		infisicalFactory:     infisicalFactory,
	}
}

// Create creates a new provider instance for the given integration.
func (f *DefaultFactory) Create(integration *model.Integration) (Provider, error) {
	if !integration.IsActive {
		return nil, fmt.Errorf("%w: integration '%s' is not active", ErrProviderNotConfigured, integration.Name)
	}

	switch integration.Type {
	case model.IntegrationTypeVault:
		if f.vaultFactory == nil {
			return nil, fmt.Errorf("%w: vault provider not configured", ErrProviderNotSupported)
		}
		return f.vaultFactory(integration)

	case model.IntegrationTypeAzureKeyVault:
		if f.azureKeyVaultFactory == nil {
			return nil, fmt.Errorf("%w: azure key vault provider not configured", ErrProviderNotSupported)
		}
		return f.azureKeyVaultFactory(integration)

	case model.IntegrationTypeInfisical:
		if f.infisicalFactory == nil {
			return nil, fmt.Errorf("%w: infisical provider not configured", ErrProviderNotSupported)
		}
		return f.infisicalFactory(integration)

	// Future integration types:
	// case model.IntegrationTypeAWS:
	//     return aws.NewProvider(integration)
	// case model.IntegrationTypeGCP:
	//     return gcp.NewProvider(integration)

	default:
		return nil, fmt.Errorf("%w: type '%s'", ErrProviderNotSupported, integration.Type)
	}
}

// SupportedTypes returns the list of integration types this factory supports.
func (f *DefaultFactory) SupportedTypes() []model.IntegrationType {
	types := make([]model.IntegrationType, 0, 3)

	if f.vaultFactory != nil {
		types = append(types, model.IntegrationTypeVault)
	}
	if f.azureKeyVaultFactory != nil {
		types = append(types, model.IntegrationTypeAzureKeyVault)
	}
	if f.infisicalFactory != nil {
		types = append(types, model.IntegrationTypeInfisical)
	}

	return types
}

// globalFactory is the singleton factory instance.
var globalFactory ProviderFactory

// SetGlobalFactory sets the global provider factory.
// This should be called during server initialization.
func SetGlobalFactory(factory ProviderFactory) {
	globalFactory = factory
}

// GetFactory returns the global provider factory.
// Returns nil if SetGlobalFactory has not been called.
func GetFactory() ProviderFactory {
	return globalFactory
}

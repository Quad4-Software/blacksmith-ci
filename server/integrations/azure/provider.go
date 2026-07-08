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

// Package azure provides an Azure Key Vault secret store provider.
package azure

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/integrations"
	"codefloe.com/crowci/crow/v6/server/model"
)

// secretsClient is a narrow interface over azsecrets.Client for testability.
type secretsClient interface {
	GetSecret(ctx context.Context, name string, version string, options *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error)
}

// Provider implements the integrations.Provider interface for Azure Key Vault.
// Azure Key Vault secrets are single-valued (name → value), so the Provider
// interface's `path` parameter maps to the secret name and `key` is ignored.
// Version pinning is not supported; the latest version is always fetched.
type Provider struct {
	integration *model.Integration
	config      *model.AzureKeyVaultConfig
	client      secretsClient
}

// NewProvider creates a new Azure Key Vault provider from an integration configuration.
func NewProvider(integration *model.Integration) (integrations.Provider, error) {
	config, err := integration.GetAzureKeyVaultConfig()
	if err != nil {
		return nil, errors.Join(integrations.ErrProviderNotConfigured, err)
	}

	config = config.WithDefaults()

	cred, err := azidentity.NewClientSecretCredential(config.TenantID, config.ClientID, config.ClientSecret, nil)
	if err != nil {
		return nil, errors.Join(integrations.ErrProviderNotConfigured, fmt.Errorf("failed to create Azure credential: %w", err))
	}

	client, err := azsecrets.NewClient(config.VaultURL, cred, nil)
	if err != nil {
		return nil, errors.Join(integrations.ErrProviderConnectionFail, fmt.Errorf("failed to create Azure Key Vault client: %w", err))
	}

	return &Provider{
		integration: integration,
		config:      config,
		client:      client,
	}, nil
}

// newProviderWithClient creates a provider with an injected client (for testing).
func newProviderWithClient(integration *model.Integration, client secretsClient) (*Provider, error) {
	config, err := integration.GetAzureKeyVaultConfig()
	if err != nil {
		return nil, errors.Join(integrations.ErrProviderNotConfigured, err)
	}

	return &Provider{
		integration: integration,
		config:      config.WithDefaults(),
		client:      client,
	}, nil
}

// Type returns the integration type.
func (p *Provider) Type() model.IntegrationType {
	return model.IntegrationTypeAzureKeyVault
}

// GetSecret retrieves a single secret value from Azure Key Vault.
// The `path` parameter is used as the secret name. The `key` parameter is
// ignored since AKV secrets are single-valued. The `version` parameter is
// ignored; the latest version is always fetched.
func (p *Provider) GetSecret(ctx context.Context, path, key string, version int) (string, error) {
	if key != "" && key != "value" {
		log.Debug().
			Int64("integration_id", p.integration.ID).
			Str("path", path).
			Str("key", key).
			Msg("Azure Key Vault secrets are single-valued; 'key' parameter is ignored")
	}

	resp, err := p.client.GetSecret(ctx, path, "", nil)
	if err != nil {
		return "", mapAzureError(err, path)
	}

	if resp.Value == nil {
		return "", fmt.Errorf("%w: secret '%s' has no value", integrations.ErrSecretNotFound, path)
	}

	return *resp.Value, nil
}

// GetSecretsBatch retrieves multiple secrets. Azure Key Vault has no batch API,
// so secrets are fetched sequentially.
func (p *Provider) GetSecretsBatch(ctx context.Context, requests []integrations.SecretRequest) (map[string]string, error) {
	results := make(map[string]string, len(requests))

	for _, req := range requests {
		value, err := p.GetSecret(ctx, req.Path, req.Key, req.Version)
		if err != nil {
			return nil, err
		}
		results[req.ID] = value
	}

	return results, nil
}

// TestConnection validates the connection and authentication by attempting
// to read a non-existent secret. A 404 response means the connection and
// credentials work; any other error indicates a problem.
func (p *Provider) TestConnection(ctx context.Context) error {
	_, err := p.client.GetSecret(ctx, "crow-connection-test", "", nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound {
			// 404 means we authenticated and reached the vault successfully
			return nil
		}
		return errors.Join(integrations.ErrProviderConnectionFail, fmt.Errorf("connection test failed: %w", err))
	}

	// Secret exists (unlikely but fine) — connection works
	return nil
}

// Close releases any resources held by the provider.
// Azure ClientSecretCredential has no persistent token to revoke.
func (p *Provider) Close() error {
	return nil
}

// mapAzureError maps Azure SDK errors to integration error types.
func mapAzureError(err error, secretName string) error {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		switch respErr.StatusCode {
		case http.StatusNotFound:
			return fmt.Errorf("%w: secret '%s' not found in Azure Key Vault", integrations.ErrSecretNotFound, secretName)
		case http.StatusUnauthorized, http.StatusForbidden:
			return fmt.Errorf("%w: access denied for secret '%s' — check Service Principal permissions (Key Vault Secrets User role required)", integrations.ErrAuthenticationFailed, secretName)
		}
	}
	return errors.Join(integrations.ErrProviderConnectionFail, fmt.Errorf("failed to read secret '%s': %w", secretName, err))
}

// NewProviderFunc returns a function that creates Azure Key Vault providers.
// This is used to inject the factory into the integrations package.
func NewProviderFunc() func(*model.Integration) (integrations.Provider, error) {
	return NewProvider
}

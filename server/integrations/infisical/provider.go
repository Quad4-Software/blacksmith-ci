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

// Package infisical provides an Infisical secret store provider.
package infisical

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	infisical "github.com/infisical/go-sdk"
	sdkErrors "github.com/infisical/go-sdk/packages/errors"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/integrations"
	"codefloe.com/crowci/crow/v6/server/model"
)

// secretsClient is a narrow interface over infisical.SecretsInterface for testability.
type secretsClient interface {
	Retrieve(options infisical.RetrieveSecretOptions) (infisical.Secret, error)
}

// Provider implements the integrations.Provider interface for Infisical.
// Infisical secrets are scoped by project, environment, and path. The integration
// config holds the project ID, environment, and secret path. The Provider
// interface's `path` parameter maps to the Infisical secret key name;
// `key` and `version` are ignored.
type Provider struct {
	integration *model.Integration
	config      *model.InfisicalConfig
	secrets     secretsClient
}

// NewProvider creates a new Infisical provider from an integration configuration.
func NewProvider(integration *model.Integration) (integrations.Provider, error) {
	config, err := integration.GetInfisicalConfig()
	if err != nil {
		return nil, errors.Join(integrations.ErrProviderNotConfigured, err)
	}

	config = config.WithDefaults()

	client := infisical.NewInfisicalClient(context.Background(), infisical.Config{
		SiteUrl:    config.SiteURL,
		SilentMode: true,
	})

	_, err = client.Auth().UniversalAuthLogin(config.ClientID, config.ClientSecret)
	if err != nil {
		return nil, wrapAuthError(integration, config, err)
	}

	log.Debug().
		Int64("integration_id", integration.ID).
		Str("integration_name", integration.Name).
		Str("site_url", config.SiteURL).
		Msg("Infisical authentication successful")

	return &Provider{
		integration: integration,
		config:      config,
		secrets:     client.Secrets(),
	}, nil
}

// newProviderWithClient creates a provider with an injected client (for testing).
func newProviderWithClient(integration *model.Integration, secrets secretsClient) (*Provider, error) {
	config, err := integration.GetInfisicalConfig()
	if err != nil {
		return nil, errors.Join(integrations.ErrProviderNotConfigured, err)
	}

	return &Provider{
		integration: integration,
		config:      config.WithDefaults(),
		secrets:     secrets,
	}, nil
}

// Type returns the integration type.
func (p *Provider) Type() model.IntegrationType {
	return model.IntegrationTypeInfisical
}

// GetSecret retrieves a single secret value from Infisical.
// The `path` parameter is used as the Infisical secret key name.
// The `key` and `version` parameters are ignored.
func (p *Provider) GetSecret(ctx context.Context, path, key string, version int) (string, error) {
	if key != "" && key != "value" {
		log.Debug().
			Int64("integration_id", p.integration.ID).
			Str("path", path).
			Str("key", key).
			Msg("Infisical secrets are single-valued; 'key' parameter is ignored")
	}

	secret, err := p.secrets.Retrieve(infisical.RetrieveSecretOptions{
		SecretKey:   path,
		ProjectID:   p.config.ProjectID,
		Environment: p.config.Environment,
		SecretPath:  p.config.SecretPath,
	})
	if err != nil {
		return "", mapInfisicalError(err, path)
	}

	return secret.SecretValue, nil
}

// GetSecretsBatch retrieves multiple secrets. Infisical has no batch-get API,
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

// TestConnection validates the connection by attempting to retrieve a
// non-existent secret. A 404 response means the connection and
// credentials work; any other error indicates a problem.
func (p *Provider) TestConnection(ctx context.Context) error {
	_, err := p.secrets.Retrieve(infisical.RetrieveSecretOptions{
		SecretKey:   "crow-connection-test",
		ProjectID:   p.config.ProjectID,
		Environment: p.config.Environment,
		SecretPath:  p.config.SecretPath,
	})
	if err != nil {
		var apiErr *sdkErrors.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			// 404 means we authenticated and reached the server successfully
			return nil
		}
		return errors.Join(integrations.ErrProviderConnectionFail, fmt.Errorf("connection test failed: %w", err))
	}

	// Secret exists (unlikely but fine) — connection works
	return nil
}

// Close releases any resources held by the provider.
// The Infisical SDK manages token lifecycle internally.
func (p *Provider) Close() error {
	return nil
}

// mapInfisicalError maps Infisical SDK errors from secret reads to integration error types.
func mapInfisicalError(err error, secretName string) error {
	if integrations.IsTimeoutError(err) {
		return fmt.Errorf("%w: timed out reading secret '%s' from Infisical: %w",
			integrations.ErrProviderTimeout, secretName, err)
	}
	var apiErr *sdkErrors.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusNotFound:
			return fmt.Errorf("%w: secret '%s' not found in Infisical", integrations.ErrSecretNotFound, secretName)
		case http.StatusUnauthorized, http.StatusForbidden:
			return fmt.Errorf("%w: access denied for secret '%s'", integrations.ErrAuthenticationFailed, secretName)
		}
	}
	return errors.Join(integrations.ErrProviderConnectionFail, fmt.Errorf("failed to read secret '%s': %w", secretName, err))
}

// wrapAuthError classifies an error from the Infisical UniversalAuthLogin call:
//   - timeouts (context deadline / net.Error.Timeout) → ErrProviderTimeout
//   - Infisical APIError 4xx → ErrAuthenticationFailed (credentials hint)
//   - Infisical APIError 5xx → ErrProviderConnectionFail (server hint)
//   - everything else (network, DNS, TLS) → ErrProviderConnectionFail
//
// This replaces the previous behavior of wrapping every login error as
// ErrAuthenticationFailed, which misled users into thinking timeouts or
// network failures were credentials problems.
func wrapAuthError(integration *model.Integration, config *model.InfisicalConfig, err error) error {
	if integrations.IsTimeoutError(err) {
		return fmt.Errorf("%w: integration '%s' (id=%d) timed out connecting to Infisical at %s. "+
			"This usually means Infisical is unreachable, overloaded, or the network is slow: %w",
			integrations.ErrProviderTimeout, integration.Name, integration.ID, config.SiteURL, err)
	}

	var apiErr *sdkErrors.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
			return fmt.Errorf("%w: integration '%s' (id=%d) was rejected by Infisical at %s (HTTP %d). "+
				"The client_id or client_secret is incorrect, expired, or the Machine Identity is misconfigured. "+
				"Please verify the integration credentials in settings: %w",
				integrations.ErrAuthenticationFailed, integration.Name, integration.ID,
				config.SiteURL, apiErr.StatusCode, err)
		}
		return fmt.Errorf("%w: integration '%s' (id=%d) - Infisical at %s returned HTTP %d. "+
			"The Infisical server appears unhealthy: %w",
			integrations.ErrProviderConnectionFail, integration.Name, integration.ID,
			config.SiteURL, apiErr.StatusCode, err)
	}

	return fmt.Errorf("%w: integration '%s' (id=%d) could not reach Infisical at %s: %w",
		integrations.ErrProviderConnectionFail, integration.Name, integration.ID,
		config.SiteURL, err)
}

// NewProviderFunc returns a function that creates Infisical providers.
// This is used to inject the factory into the integrations package.
func NewProviderFunc() func(*model.Integration) (integrations.Provider, error) {
	return NewProvider
}

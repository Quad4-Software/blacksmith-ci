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

// Package vault provides a HashiCorp Vault/OpenBao secret store provider.
package vault

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/integrations"
	"codefloe.com/crowci/crow/v6/server/model"
)

const (
	// CloseTimeout is the timeout for closing/revoking tokens.
	closeTimeout = 5 * time.Second
	// TokenRefreshRatio is the percentage of TTL at which to refresh tokens (80 out of 100).
	tokenRefreshRatio   = 80
	tokenRefreshDivisor = 100
)

// Provider implements the integrations.Provider interface for HashiCorp Vault/OpenBao.
type Provider struct {
	integration *model.Integration
	config      *model.VaultConfig
	client      *vault.Client
	token       string
	tokenExpiry time.Time
	mu          sync.RWMutex
}

// NewProvider creates a new Vault provider from an integration configuration.
func NewProvider(integration *model.Integration) (integrations.Provider, error) {
	config, err := integration.GetVaultConfig()
	if err != nil {
		return nil, errors.Join(integrations.ErrProviderNotConfigured, err)
	}

	// Apply defaults
	config = config.WithDefaults()

	// Create Vault client configuration
	vaultConfig := vault.DefaultConfig()
	vaultConfig.Address = config.Address
	vaultConfig.Timeout = config.RequestTimeout()

	// Configure TLS
	if config.TLSConfig != nil {
		tlsConfig, err := buildTLSConfig(config.TLSConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to configure TLS: %w", err)
		}

		transport, ok := vaultConfig.HttpClient.Transport.(*http.Transport)
		if !ok {
			return nil, fmt.Errorf("unexpected transport type: %T", vaultConfig.HttpClient.Transport)
		}
		clonedTransport := transport.Clone()
		clonedTransport.TLSClientConfig = tlsConfig
		vaultConfig.HttpClient.Transport = clonedTransport
	}

	// Create Vault client
	client, err := vault.NewClient(vaultConfig)
	if err != nil {
		return nil, errors.Join(integrations.ErrProviderConnectionFail, err)
	}

	// Set namespace if provided (Vault Enterprise feature)
	if config.Namespace != "" {
		client.SetNamespace(config.Namespace)
	}

	provider := &Provider{
		integration: integration,
		config:      config,
		client:      client,
	}

	// Authenticate immediately to validate credentials
	if err := provider.authenticate(context.Background()); err != nil {
		return nil, err
	}

	return provider, nil
}

// Type returns the integration type.
func (p *Provider) Type() model.IntegrationType {
	return model.IntegrationTypeVault
}

// GetSecret retrieves a single secret value from Vault.
func (p *Provider) GetSecret(ctx context.Context, path, key string, version int) (string, error) {
	if err := p.ensureToken(ctx); err != nil {
		return "", err
	}

	// Build the full path for KV v2 secrets engine
	fullPath := fmt.Sprintf("%s/data/%s", p.config.MountPath, path)

	// Prepare request options
	var secret *vault.Secret
	var err error

	if version > 0 {
		// Read specific version
		secret, err = p.client.Logical().ReadWithDataWithContext(ctx, fullPath, map[string][]string{
			"version": {fmt.Sprintf("%d", version)},
		})
	} else {
		// Read latest version
		secret, err = p.client.Logical().ReadWithContext(ctx, fullPath)
	}

	if err != nil {
		return "", p.wrapReadError(err, path)
	}

	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("%w: no secret at path '%s'", integrations.ErrSecretNotFound, path)
	}

	// KV v2 nests the actual data under "data" key
	data, ok := secret.Data["data"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("%w: invalid secret format at path '%s'", integrations.ErrSecretNotFound, path)
	}

	// Get the specific key
	value, ok := data[key]
	if !ok {
		return "", fmt.Errorf("%w: key '%s' not found in secret at '%s'", integrations.ErrSecretKeyNotFound, key, path)
	}

	// Convert to string
	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%w: key '%s' at '%s' is not a string", integrations.ErrSecretKeyNotFound, key, path)
	}

	return strValue, nil
}

// GetSecretsBatch retrieves multiple secrets efficiently.
// Optimizes by grouping requests by path to minimize Vault API calls.
func (p *Provider) GetSecretsBatch(ctx context.Context, requests []integrations.SecretRequest) (map[string]string, error) {
	if err := p.ensureToken(ctx); err != nil {
		return nil, err
	}

	results := make(map[string]string, len(requests))

	// Group requests by path for efficient fetching
	pathGroups := make(map[string][]integrations.SecretRequest)
	for _, req := range requests {
		pathKey := fmt.Sprintf("%s:v%d", req.Path, req.Version)
		pathGroups[pathKey] = append(pathGroups[pathKey], req)
	}

	// Fetch each unique path once
	for _, reqs := range pathGroups {
		if len(reqs) == 0 {
			continue
		}

		// Use the first request's path and version (all should be same in group)
		firstReq := reqs[0]

		// Build the full path
		fullPath := fmt.Sprintf("%s/data/%s", p.config.MountPath, firstReq.Path)

		// Read the secret
		var secret *vault.Secret
		var err error

		if firstReq.Version > 0 {
			secret, err = p.client.Logical().ReadWithDataWithContext(ctx, fullPath, map[string][]string{
				"version": {fmt.Sprintf("%d", firstReq.Version)},
			})
		} else {
			secret, err = p.client.Logical().ReadWithContext(ctx, fullPath)
		}

		if err != nil {
			return nil, p.wrapReadError(err, firstReq.Path)
		}

		if secret == nil || secret.Data == nil {
			return nil, fmt.Errorf("%w: no secret at path '%s'", integrations.ErrSecretNotFound, firstReq.Path)
		}

		// Extract data
		data, ok := secret.Data["data"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: invalid secret format at path '%s'", integrations.ErrSecretNotFound, firstReq.Path)
		}

		// Extract each requested key
		for _, req := range reqs {
			value, ok := data[req.Key]
			if !ok {
				return nil, fmt.Errorf("%w: key '%s' not found in secret at '%s'", integrations.ErrSecretKeyNotFound, req.Key, req.Path)
			}

			strValue, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("%w: key '%s' at '%s' is not a string", integrations.ErrSecretKeyNotFound, req.Key, req.Path)
			}

			results[req.ID] = strValue
		}
	}

	return results, nil
}

// TestConnection validates the connection and authentication.
func (p *Provider) TestConnection(ctx context.Context) error {
	if err := p.ensureToken(ctx); err != nil {
		return err
	}

	// Try to look up our own token to verify everything works
	_, err := p.client.Auth().Token().LookupSelfWithContext(ctx)
	if err != nil {
		return errors.Join(integrations.ErrProviderConnectionFail, fmt.Errorf("token lookup failed: %w", err))
	}

	return nil
}

// Close releases resources and revokes the token.
func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Revoke our token to clean up
	if p.token != "" {
		ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
		defer cancel()

		if err := p.client.Auth().Token().RevokeSelfWithContext(ctx, ""); err != nil {
			log.Warn().Err(err).
				Int64("integration_id", p.integration.ID).
				Msg("Failed to revoke Vault token during close")
		}

		p.token = ""
		p.tokenExpiry = time.Time{}
	}

	return nil
}

// authenticate performs AppRole authentication with Vault.
func (p *Provider) authenticate(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Prepare AppRole login data
	loginData := map[string]any{
		"role_id":   p.config.RoleID,
		"secret_id": p.config.SecretID,
	}

	// Perform AppRole login
	resp, err := p.client.Logical().WriteWithContext(ctx, p.config.AuthPath+"/login", loginData)
	if err != nil {
		return p.wrapAuthError(err)
	}

	if resp == nil || resp.Auth == nil {
		return fmt.Errorf("%w: integration '%s' (id=%d) - AppRole login returned no auth info",
			integrations.ErrAuthenticationFailed, p.integration.Name, p.integration.ID)
	}

	// Set the token
	p.token = resp.Auth.ClientToken
	p.client.SetToken(p.token)

	// Calculate token expiry (with some buffer for safety)
	tokenTTL := time.Duration(resp.Auth.LeaseDuration) * time.Second
	if tokenTTL > 0 {
		// Refresh at 80% of TTL
		p.tokenExpiry = time.Now().Add(tokenTTL * tokenRefreshRatio / tokenRefreshDivisor)
	} else {
		// Default to 1 hour if no TTL provided
		p.tokenExpiry = time.Now().Add(1 * time.Hour)
	}

	log.Debug().
		Int64("integration_id", p.integration.ID).
		Str("integration_name", p.integration.Name).
		Time("token_expiry", p.tokenExpiry).
		Msg("Vault authentication successful")

	return nil
}

// ensureToken ensures we have a valid token, refreshing if necessary.
func (p *Provider) ensureToken(ctx context.Context) error {
	p.mu.RLock()
	needsRefresh := time.Now().After(p.tokenExpiry)
	p.mu.RUnlock()

	if needsRefresh {
		return p.authenticate(ctx)
	}

	return nil
}

// wrapAuthError classifies an error from the AppRole login call and returns
// a user-friendly error wrapping the most specific sentinel:
//   - timeouts (context deadline / net.Error.Timeout) → ErrProviderTimeout
//   - HTTP 4xx responses from Vault → ErrAuthenticationFailed (credentials hint)
//   - HTTP 5xx responses from Vault → ErrProviderConnectionFail (server hint)
//   - everything else (network, DNS, TLS) → ErrProviderConnectionFail
func (p *Provider) wrapAuthError(err error) error {
	if integrations.IsTimeoutError(err) {
		return fmt.Errorf("%w: integration '%s' (id=%d) timed out connecting to Vault at %s after %s. "+
			"This usually means Vault is unreachable, overloaded, or the network is slow. "+
			"Increase 'timeout_seconds' on the integration if Vault is healthy but slow: %w",
			integrations.ErrProviderTimeout, p.integration.Name, p.integration.ID,
			p.config.Address, p.config.RequestTimeout(), err)
	}

	var respErr *vault.ResponseError
	if errors.As(err, &respErr) {
		if respErr.StatusCode >= 400 && respErr.StatusCode < 500 {
			return fmt.Errorf("%w: integration '%s' (id=%d) was rejected by Vault at %s (HTTP %d). "+
				"The role_id or secret_id is incorrect, expired, or the AppRole is misconfigured. "+
				"Please verify the integration credentials in settings: %w",
				integrations.ErrAuthenticationFailed, p.integration.Name, p.integration.ID,
				p.config.Address, respErr.StatusCode, err)
		}
		return fmt.Errorf("%w: integration '%s' (id=%d) - Vault at %s returned HTTP %d. "+
			"The Vault server appears unhealthy: %w",
			integrations.ErrProviderConnectionFail, p.integration.Name, p.integration.ID,
			p.config.Address, respErr.StatusCode, err)
	}

	return fmt.Errorf("%w: integration '%s' (id=%d) could not reach Vault at %s: %w",
		integrations.ErrProviderConnectionFail, p.integration.Name, p.integration.ID,
		p.config.Address, err)
}

// wrapReadError classifies an error from a secret read call.
func (p *Provider) wrapReadError(err error, path string) error {
	if integrations.IsTimeoutError(err) {
		return fmt.Errorf("%w: timed out reading secret at '%s' from Vault at %s after %s: %w",
			integrations.ErrProviderTimeout, path, p.config.Address, p.config.RequestTimeout(), err)
	}
	return errors.Join(integrations.ErrProviderConnectionFail,
		fmt.Errorf("failed to read secret at %s: %w", path, err))
}

// buildTLSConfig creates a TLS configuration from VaultTLSConfig.
func buildTLSConfig(config *model.VaultTLSConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: config.Insecure, //nolint:gosec
		ServerName:         config.TLSServerName,
		MinVersion:         tls.VersionTLS12,
	}

	// Add CA certificate if provided
	if config.CACert != "" {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM([]byte(config.CACert)) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	// Add client certificate if provided
	if config.ClientCert != "" && config.ClientKey != "" {
		cert, err := tls.X509KeyPair([]byte(config.ClientCert), []byte(config.ClientKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// NewProviderFunc returns a function that creates Vault providers.
// This is used to inject the factory into the integrations package.
func NewProviderFunc() func(*model.Integration) (integrations.Provider, error) {
	return NewProvider
}

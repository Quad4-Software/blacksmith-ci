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

// Package integrations provides interfaces and implementations for external
// secret store integrations such as HashiCorp Vault, AWS Secrets Manager, etc.
package integrations

import (
	"context"
	"errors"

	"codefloe.com/crowci/crow/v6/server/model"
)

// Common provider errors.
var (
	ErrProviderNotSupported   = errors.New("integration provider type not supported")
	ErrProviderNotConfigured  = errors.New("integration provider not properly configured")
	ErrProviderConnectionFail = errors.New("failed to connect to secret store")
	ErrProviderTimeout        = errors.New("secret store request timed out")
	ErrSecretNotFound         = errors.New("secret not found")
	ErrSecretKeyNotFound      = errors.New("secret key not found")
	ErrAuthenticationFailed   = errors.New("authentication failed")
)

// Provider defines the interface for external secret store providers.
// Each secret store type (Vault, AWS, Azure, etc.) implements this interface.
type Provider interface {
	// Type returns the integration type this provider handles.
	Type() model.IntegrationType

	// GetSecret retrieves a single secret value.
	// path: The path to the secret in the store (e.g., "database/prod/mysql")
	// key: The specific key within the secret (e.g., "password")
	// version: The secret version (0 = latest)
	GetSecret(ctx context.Context, path, key string, version int) (string, error)

	// GetSecretsBatch retrieves multiple secrets efficiently.
	// Implementations should optimize for batch operations when possible.
	GetSecretsBatch(ctx context.Context, requests []SecretRequest) (map[string]string, error)

	// TestConnection validates that the provider can connect and authenticate.
	// This is used when creating/updating integrations to verify credentials.
	TestConnection(ctx context.Context) error

	// Close releases any resources held by the provider.
	// This should revoke any temporary tokens and close connections.
	Close() error
}

// SecretRequest represents a single secret fetch request.
type SecretRequest struct {
	// ID is a unique identifier for this request (used as map key in response)
	ID string

	// Path is the secret path in the store
	Path string

	// Key is the specific key within the secret
	Key string

	// Version is the secret version (0 = latest)
	Version int
}

// NewSecretRequest creates a new SecretRequest from an ExternalSecretRef.
func NewSecretRequest(ref *model.ExternalSecretRef) SecretRequest {
	return SecretRequest{
		ID:      ref.CacheKey(),
		Path:    ref.Path,
		Key:     ref.Key,
		Version: ref.Version,
	}
}

// ProviderFactory creates provider instances from integration configurations.
type ProviderFactory interface {
	// Create creates a new provider instance for the given integration.
	// The integration's Config field is parsed and used to configure the provider.
	Create(integration *model.Integration) (Provider, error)

	// SupportedTypes returns the list of integration types this factory supports.
	SupportedTypes() []model.IntegrationType
}

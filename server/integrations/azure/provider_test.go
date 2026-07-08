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

package azure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/integrations"
	"codefloe.com/crowci/crow/v6/server/model"
)

// mockSecretsClient is a test double for the secretsClient interface.
type mockSecretsClient struct {
	getSecretFunc func(ctx context.Context, name, version string, options *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error)
}

func (m *mockSecretsClient) GetSecret(ctx context.Context, name, version string, options *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
	return m.getSecretFunc(ctx, name, version, options)
}

func validAzureConfig() model.AzureKeyVaultConfig {
	return model.AzureKeyVaultConfig{
		VaultURL:     "https://my-vault.vault.azure.net",
		TenantID:     "tenant-123",
		ClientID:     "client-456",
		ClientSecret: "secret-789",
	}
}

func validAzureIntegration(t *testing.T) *model.Integration {
	t.Helper()

	config := validAzureConfig()
	configJSON, err := json.Marshal(config)
	assert.NoError(t, err)
	return &model.Integration{
		ID:     1,
		Name:   "test-azure",
		Type:   model.IntegrationTypeAzureKeyVault,
		Config: string(configJSON),
	}
}

func TestNewProvider_InvalidJSON(t *testing.T) {
	integration := &model.Integration{
		Type:   model.IntegrationTypeAzureKeyVault,
		Config: "not json",
	}

	_, err := NewProvider(integration)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrProviderNotConfigured))
}

func TestNewProvider_WrongType(t *testing.T) {
	integration := &model.Integration{
		Type:   "other",
		Config: `{"vault_url":"https://test.vault.azure.net","tenant_id":"t","client_id":"c","client_secret":"s"}`,
	}

	_, err := NewProvider(integration)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrProviderNotConfigured))
}

func TestProvider_Type(t *testing.T) {
	provider := &Provider{}
	assert.Equal(t, model.IntegrationTypeAzureKeyVault, provider.Type())
}

func TestProvider_GetSecret(t *testing.T) {
	secretValue := "my-secret-value"
	mock := &mockSecretsClient{
		getSecretFunc: func(_ context.Context, name, version string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
			assert.Equal(t, "db-password", name)
			assert.Equal(t, "", version)
			return azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{
					Value: &secretValue,
				},
			}, nil
		},
	}

	provider, err := newProviderWithClient(validAzureIntegration(t), mock)
	assert.NoError(t, err)

	value, err := provider.GetSecret(context.Background(), "db-password", "value", 0)
	assert.NoError(t, err)
	assert.Equal(t, "my-secret-value", value)
}

func TestProvider_GetSecret_KeyIgnored(t *testing.T) {
	secretValue := "the-value"
	mock := &mockSecretsClient{
		getSecretFunc: func(_ context.Context, name, _ string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
			return azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{Value: &secretValue},
			}, nil
		},
	}

	provider, err := newProviderWithClient(validAzureIntegration(t), mock)
	assert.NoError(t, err)

	// key="some-key" should be ignored, not cause an error
	value, err := provider.GetSecret(context.Background(), "my-secret", "some-key", 0)
	assert.NoError(t, err)
	assert.Equal(t, "the-value", value)
}

func TestProvider_GetSecret_NotFound(t *testing.T) {
	mock := &mockSecretsClient{
		getSecretFunc: func(_ context.Context, _, _ string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
			return azsecrets.GetSecretResponse{}, &azcore.ResponseError{
				StatusCode: http.StatusNotFound,
			}
		},
	}

	provider, err := newProviderWithClient(validAzureIntegration(t), mock)
	assert.NoError(t, err)

	_, err = provider.GetSecret(context.Background(), "missing-secret", "value", 0)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrSecretNotFound))
}

func TestProvider_GetSecret_Unauthorized(t *testing.T) {
	mock := &mockSecretsClient{
		getSecretFunc: func(_ context.Context, _, _ string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
			return azsecrets.GetSecretResponse{}, &azcore.ResponseError{
				StatusCode: http.StatusForbidden,
			}
		},
	}

	provider, err := newProviderWithClient(validAzureIntegration(t), mock)
	assert.NoError(t, err)

	_, err = provider.GetSecret(context.Background(), "restricted-secret", "value", 0)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrAuthenticationFailed))
}

func TestProvider_GetSecret_NilValue(t *testing.T) {
	mock := &mockSecretsClient{
		getSecretFunc: func(_ context.Context, _, _ string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
			return azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{Value: nil},
			}, nil
		},
	}

	provider, err := newProviderWithClient(validAzureIntegration(t), mock)
	assert.NoError(t, err)

	_, err = provider.GetSecret(context.Background(), "empty-secret", "value", 0)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrSecretNotFound))
}

func TestProvider_GetSecretsBatch(t *testing.T) {
	secrets := map[string]string{
		"secret-a": "value-a",
		"secret-b": "value-b",
	}
	mock := &mockSecretsClient{
		getSecretFunc: func(_ context.Context, name, _ string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
			val, ok := secrets[name]
			if !ok {
				return azsecrets.GetSecretResponse{}, &azcore.ResponseError{StatusCode: http.StatusNotFound}
			}
			return azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{Value: &val},
			}, nil
		},
	}

	provider, err := newProviderWithClient(validAzureIntegration(t), mock)
	assert.NoError(t, err)

	results, err := provider.GetSecretsBatch(context.Background(), []integrations.SecretRequest{
		{ID: "req-a", Path: "secret-a", Key: "value"},
		{ID: "req-b", Path: "secret-b", Key: "value"},
	})
	assert.NoError(t, err)
	assert.Equal(t, "value-a", results["req-a"])
	assert.Equal(t, "value-b", results["req-b"])
}

func TestProvider_GetSecretsBatch_PartialFailure(t *testing.T) {
	mock := &mockSecretsClient{
		getSecretFunc: func(_ context.Context, name, _ string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
			if name == "exists" {
				val := "ok"
				return azsecrets.GetSecretResponse{
					Secret: azsecrets.Secret{Value: &val},
				}, nil
			}
			return azsecrets.GetSecretResponse{}, &azcore.ResponseError{StatusCode: http.StatusNotFound}
		},
	}

	provider, err := newProviderWithClient(validAzureIntegration(t), mock)
	assert.NoError(t, err)

	_, err = provider.GetSecretsBatch(context.Background(), []integrations.SecretRequest{
		{ID: "req-1", Path: "exists", Key: "value"},
		{ID: "req-2", Path: "missing", Key: "value"},
	})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrSecretNotFound))
}

func TestProvider_TestConnection_Success(t *testing.T) {
	mock := &mockSecretsClient{
		getSecretFunc: func(_ context.Context, name, _ string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
			assert.Equal(t, "crow-connection-test", name)
			return azsecrets.GetSecretResponse{}, &azcore.ResponseError{
				StatusCode: http.StatusNotFound,
			}
		},
	}

	provider, err := newProviderWithClient(validAzureIntegration(t), mock)
	assert.NoError(t, err)

	err = provider.TestConnection(context.Background())
	assert.NoError(t, err)
}

func TestProvider_TestConnection_AuthFailed(t *testing.T) {
	mock := &mockSecretsClient{
		getSecretFunc: func(_ context.Context, _, _ string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
			return azsecrets.GetSecretResponse{}, &azcore.ResponseError{
				StatusCode: http.StatusUnauthorized,
			}
		},
	}

	provider, err := newProviderWithClient(validAzureIntegration(t), mock)
	assert.NoError(t, err)

	err = provider.TestConnection(context.Background())
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrProviderConnectionFail))
}

func TestProvider_TestConnection_SecretExists(t *testing.T) {
	val := "unexpected"
	mock := &mockSecretsClient{
		getSecretFunc: func(_ context.Context, _, _ string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
			return azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{Value: &val},
			}, nil
		},
	}

	provider, err := newProviderWithClient(validAzureIntegration(t), mock)
	assert.NoError(t, err)

	// If the test secret exists, connection still works
	err = provider.TestConnection(context.Background())
	assert.NoError(t, err)
}

func TestProvider_Close(t *testing.T) {
	provider := &Provider{}
	assert.NoError(t, provider.Close())
}

func TestNewProviderFunc(t *testing.T) {
	fn := NewProviderFunc()
	assert.NotNil(t, fn)

	// Should return error for invalid config
	integration := &model.Integration{
		Type:   model.IntegrationTypeAzureKeyVault,
		Config: "invalid json",
	}
	_, err := fn(integration)
	assert.Error(t, err)
}

func TestAzureKeyVaultConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      model.AzureKeyVaultConfig
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid config",
			config:  validAzureConfig(),
			wantErr: false,
		},
		{
			name: "missing vault_url",
			config: model.AzureKeyVaultConfig{
				TenantID: "t", ClientID: "c", ClientSecret: "s",
			},
			wantErr:     true,
			errContains: "vault_url is required",
		},
		{
			name: "http vault_url",
			config: model.AzureKeyVaultConfig{
				VaultURL: "http://insecure.vault.azure.net", TenantID: "t", ClientID: "c", ClientSecret: "s",
			},
			wantErr:     true,
			errContains: "must use https",
		},
		{
			name: "missing tenant_id",
			config: model.AzureKeyVaultConfig{
				VaultURL: "https://test.vault.azure.net", ClientID: "c", ClientSecret: "s",
			},
			wantErr:     true,
			errContains: "tenant_id is required",
		},
		{
			name: "missing client_id",
			config: model.AzureKeyVaultConfig{
				VaultURL: "https://test.vault.azure.net", TenantID: "t", ClientSecret: "s",
			},
			wantErr:     true,
			errContains: "client_id is required",
		},
		{
			name: "missing client_secret",
			config: model.AzureKeyVaultConfig{
				VaultURL: "https://test.vault.azure.net", TenantID: "t", ClientID: "c",
			},
			wantErr:     true,
			errContains: "client_secret is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAzureKeyVaultConfig_Redacted(t *testing.T) {
	config := validAzureConfig()
	redacted := config.Redacted()

	assert.Equal(t, config.VaultURL, redacted.VaultURL)
	assert.Equal(t, config.TenantID, redacted.TenantID)
	assert.NotEqual(t, config.ClientID, redacted.ClientID)
	assert.Equal(t, "***", redacted.ClientSecret)
}

func TestMapAzureError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantError error
	}{
		{
			name:      "404 maps to ErrSecretNotFound",
			err:       &azcore.ResponseError{StatusCode: http.StatusNotFound},
			wantError: integrations.ErrSecretNotFound,
		},
		{
			name:      "401 maps to ErrAuthenticationFailed",
			err:       &azcore.ResponseError{StatusCode: http.StatusUnauthorized},
			wantError: integrations.ErrAuthenticationFailed,
		},
		{
			name:      "403 maps to ErrAuthenticationFailed",
			err:       &azcore.ResponseError{StatusCode: http.StatusForbidden},
			wantError: integrations.ErrAuthenticationFailed,
		},
		{
			name:      "500 maps to ErrProviderConnectionFail",
			err:       &azcore.ResponseError{StatusCode: http.StatusInternalServerError},
			wantError: integrations.ErrProviderConnectionFail,
		},
		{
			name:      "generic error maps to ErrProviderConnectionFail",
			err:       fmt.Errorf("network timeout"),
			wantError: integrations.ErrProviderConnectionFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapAzureError(tt.err, "test-secret")
			assert.True(t, errors.Is(result, tt.wantError))
		})
	}
}

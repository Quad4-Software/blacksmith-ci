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

package infisical

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	infisical "github.com/infisical/go-sdk"
	sdkErrors "github.com/infisical/go-sdk/packages/errors"
	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/integrations"
	"codefloe.com/crowci/crow/v6/server/model"
)

// mockSecretsClient is a test double for the secretsClient interface.
type mockSecretsClient struct {
	retrieveFunc func(options infisical.RetrieveSecretOptions) (infisical.Secret, error)
}

func (m *mockSecretsClient) Retrieve(options infisical.RetrieveSecretOptions) (infisical.Secret, error) {
	return m.retrieveFunc(options)
}

func validInfisicalConfig() model.InfisicalConfig {
	return model.InfisicalConfig{
		SiteURL:      "https://app.infisical.com",
		ClientID:     "client-id-123",
		ClientSecret: "client-secret-456",
		ProjectID:    "proj-789",
		Environment:  "prod",
		SecretPath:   "/",
	}
}

func validInfisicalIntegration(t *testing.T) *model.Integration {
	t.Helper()

	config := validInfisicalConfig()
	configJSON, err := json.Marshal(config)
	assert.NoError(t, err)
	return &model.Integration{
		ID:     1,
		Name:   "test-infisical",
		Type:   model.IntegrationTypeInfisical,
		Config: string(configJSON),
	}
}

func TestNewProvider_InvalidJSON(t *testing.T) {
	integration := &model.Integration{
		Type:   model.IntegrationTypeInfisical,
		Config: "not json",
	}

	_, err := NewProvider(integration)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrProviderNotConfigured))
}

func TestNewProvider_WrongType(t *testing.T) {
	integration := &model.Integration{
		Type:   "other",
		Config: `{"site_url":"https://app.infisical.com","client_id":"c","client_secret":"s","project_id":"p","environment":"dev"}`,
	}

	_, err := NewProvider(integration)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrProviderNotConfigured))
}

func TestProvider_Type(t *testing.T) {
	provider := &Provider{}
	assert.Equal(t, model.IntegrationTypeInfisical, provider.Type())
}

func TestProvider_GetSecret(t *testing.T) {
	mock := &mockSecretsClient{
		retrieveFunc: func(options infisical.RetrieveSecretOptions) (infisical.Secret, error) {
			assert.Equal(t, "DATABASE_PASSWORD", options.SecretKey)
			assert.Equal(t, "proj-789", options.ProjectID)
			assert.Equal(t, "prod", options.Environment)
			assert.Equal(t, "/", options.SecretPath)
			return infisical.Secret{SecretValue: "super-secret"}, nil
		},
	}

	provider, err := newProviderWithClient(validInfisicalIntegration(t), mock)
	assert.NoError(t, err)

	value, err := provider.GetSecret(context.Background(), "DATABASE_PASSWORD", "value", 0)
	assert.NoError(t, err)
	assert.Equal(t, "super-secret", value)
}

func TestProvider_GetSecret_KeyIgnored(t *testing.T) {
	mock := &mockSecretsClient{
		retrieveFunc: func(_ infisical.RetrieveSecretOptions) (infisical.Secret, error) {
			return infisical.Secret{SecretValue: "the-value"}, nil
		},
	}

	provider, err := newProviderWithClient(validInfisicalIntegration(t), mock)
	assert.NoError(t, err)

	// key="some-key" should be ignored, not cause an error
	value, err := provider.GetSecret(context.Background(), "MY_SECRET", "some-key", 0)
	assert.NoError(t, err)
	assert.Equal(t, "the-value", value)
}

func TestProvider_GetSecret_NotFound(t *testing.T) {
	mock := &mockSecretsClient{
		retrieveFunc: func(_ infisical.RetrieveSecretOptions) (infisical.Secret, error) {
			return infisical.Secret{}, &sdkErrors.APIError{StatusCode: http.StatusNotFound}
		},
	}

	provider, err := newProviderWithClient(validInfisicalIntegration(t), mock)
	assert.NoError(t, err)

	_, err = provider.GetSecret(context.Background(), "MISSING", "value", 0)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrSecretNotFound))
}

func TestProvider_GetSecret_Unauthorized(t *testing.T) {
	mock := &mockSecretsClient{
		retrieveFunc: func(_ infisical.RetrieveSecretOptions) (infisical.Secret, error) {
			return infisical.Secret{}, &sdkErrors.APIError{StatusCode: http.StatusForbidden}
		},
	}

	provider, err := newProviderWithClient(validInfisicalIntegration(t), mock)
	assert.NoError(t, err)

	_, err = provider.GetSecret(context.Background(), "RESTRICTED", "value", 0)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrAuthenticationFailed))
}

func TestProvider_GetSecretsBatch(t *testing.T) {
	secrets := map[string]string{
		"SECRET_A": "value-a",
		"SECRET_B": "value-b",
	}
	mock := &mockSecretsClient{
		retrieveFunc: func(options infisical.RetrieveSecretOptions) (infisical.Secret, error) {
			val, ok := secrets[options.SecretKey]
			if !ok {
				return infisical.Secret{}, &sdkErrors.APIError{StatusCode: http.StatusNotFound}
			}
			return infisical.Secret{SecretValue: val}, nil
		},
	}

	provider, err := newProviderWithClient(validInfisicalIntegration(t), mock)
	assert.NoError(t, err)

	results, err := provider.GetSecretsBatch(context.Background(), []integrations.SecretRequest{
		{ID: "req-a", Path: "SECRET_A", Key: "value"},
		{ID: "req-b", Path: "SECRET_B", Key: "value"},
	})
	assert.NoError(t, err)
	assert.Equal(t, "value-a", results["req-a"])
	assert.Equal(t, "value-b", results["req-b"])
}

func TestProvider_GetSecretsBatch_PartialFailure(t *testing.T) {
	mock := &mockSecretsClient{
		retrieveFunc: func(options infisical.RetrieveSecretOptions) (infisical.Secret, error) {
			if options.SecretKey == "EXISTS" {
				return infisical.Secret{SecretValue: "ok"}, nil
			}
			return infisical.Secret{}, &sdkErrors.APIError{StatusCode: http.StatusNotFound}
		},
	}

	provider, err := newProviderWithClient(validInfisicalIntegration(t), mock)
	assert.NoError(t, err)

	_, err = provider.GetSecretsBatch(context.Background(), []integrations.SecretRequest{
		{ID: "req-1", Path: "EXISTS", Key: "value"},
		{ID: "req-2", Path: "MISSING", Key: "value"},
	})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrSecretNotFound))
}

func TestProvider_TestConnection_Success(t *testing.T) {
	mock := &mockSecretsClient{
		retrieveFunc: func(options infisical.RetrieveSecretOptions) (infisical.Secret, error) {
			assert.Equal(t, "crow-connection-test", options.SecretKey)
			return infisical.Secret{}, &sdkErrors.APIError{StatusCode: http.StatusNotFound}
		},
	}

	provider, err := newProviderWithClient(validInfisicalIntegration(t), mock)
	assert.NoError(t, err)

	err = provider.TestConnection(context.Background())
	assert.NoError(t, err)
}

func TestProvider_TestConnection_AuthFailed(t *testing.T) {
	mock := &mockSecretsClient{
		retrieveFunc: func(_ infisical.RetrieveSecretOptions) (infisical.Secret, error) {
			return infisical.Secret{}, fmt.Errorf("connection refused")
		},
	}

	provider, err := newProviderWithClient(validInfisicalIntegration(t), mock)
	assert.NoError(t, err)

	err = provider.TestConnection(context.Background())
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrProviderConnectionFail))
}

func TestProvider_TestConnection_SecretExists(t *testing.T) {
	mock := &mockSecretsClient{
		retrieveFunc: func(_ infisical.RetrieveSecretOptions) (infisical.Secret, error) {
			return infisical.Secret{SecretValue: "unexpected"}, nil
		},
	}

	provider, err := newProviderWithClient(validInfisicalIntegration(t), mock)
	assert.NoError(t, err)

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

	integration := &model.Integration{
		Type:   model.IntegrationTypeInfisical,
		Config: "invalid json",
	}
	_, err := fn(integration)
	assert.Error(t, err)
}

func TestInfisicalConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      model.InfisicalConfig
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid config",
			config:  validInfisicalConfig(),
			wantErr: false,
		},
		{
			name: "missing site_url",
			config: model.InfisicalConfig{
				ClientID: "c", ClientSecret: "s", ProjectID: "p", Environment: "dev",
			},
			wantErr:     true,
			errContains: "site_url is required",
		},
		{
			name: "invalid site_url scheme",
			config: model.InfisicalConfig{
				SiteURL: "ftp://example.com", ClientID: "c", ClientSecret: "s", ProjectID: "p", Environment: "dev",
			},
			wantErr:     true,
			errContains: "must use http or https",
		},
		{
			name: "missing client_id",
			config: model.InfisicalConfig{
				SiteURL: "https://app.infisical.com", ClientSecret: "s", ProjectID: "p", Environment: "dev",
			},
			wantErr:     true,
			errContains: "client_id is required",
		},
		{
			name: "missing client_secret",
			config: model.InfisicalConfig{
				SiteURL: "https://app.infisical.com", ClientID: "c", ProjectID: "p", Environment: "dev",
			},
			wantErr:     true,
			errContains: "client_secret is required",
		},
		{
			name: "missing project_id",
			config: model.InfisicalConfig{
				SiteURL: "https://app.infisical.com", ClientID: "c", ClientSecret: "s", Environment: "dev",
			},
			wantErr:     true,
			errContains: "project_id is required",
		},
		{
			name: "missing environment",
			config: model.InfisicalConfig{
				SiteURL: "https://app.infisical.com", ClientID: "c", ClientSecret: "s", ProjectID: "p",
			},
			wantErr:     true,
			errContains: "environment is required",
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

func TestInfisicalConfig_Redacted(t *testing.T) {
	config := validInfisicalConfig()
	redacted := config.Redacted()

	assert.Equal(t, config.SiteURL, redacted.SiteURL)
	assert.NotEqual(t, config.ClientID, redacted.ClientID)
	assert.Equal(t, "***", redacted.ClientSecret)
	assert.Equal(t, config.ProjectID, redacted.ProjectID)
	assert.Equal(t, config.Environment, redacted.Environment)
	assert.Equal(t, config.SecretPath, redacted.SecretPath)
}

func TestInfisicalConfig_WithDefaults(t *testing.T) {
	config := &model.InfisicalConfig{
		ClientID:     "c",
		ClientSecret: "s",
		ProjectID:    "p",
		Environment:  "dev",
	}

	result := config.WithDefaults()
	assert.Equal(t, "https://app.infisical.com", result.SiteURL)
	assert.Equal(t, "/", result.SecretPath)
}

func TestMapInfisicalError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantError error
	}{
		{
			name:      "context deadline maps to ErrProviderTimeout",
			err:       context.DeadlineExceeded,
			wantError: integrations.ErrProviderTimeout,
		},
		{
			name:      "404 maps to ErrSecretNotFound",
			err:       &sdkErrors.APIError{StatusCode: http.StatusNotFound},
			wantError: integrations.ErrSecretNotFound,
		},
		{
			name:      "401 maps to ErrAuthenticationFailed",
			err:       &sdkErrors.APIError{StatusCode: http.StatusUnauthorized},
			wantError: integrations.ErrAuthenticationFailed,
		},
		{
			name:      "403 maps to ErrAuthenticationFailed",
			err:       &sdkErrors.APIError{StatusCode: http.StatusForbidden},
			wantError: integrations.ErrAuthenticationFailed,
		},
		{
			name:      "500 maps to ErrProviderConnectionFail",
			err:       &sdkErrors.APIError{StatusCode: http.StatusInternalServerError},
			wantError: integrations.ErrProviderConnectionFail,
		},
		{
			name:      "generic error maps to ErrProviderConnectionFail",
			err:       fmt.Errorf("connection refused"),
			wantError: integrations.ErrProviderConnectionFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapInfisicalError(tt.err, "test-secret")
			assert.True(t, errors.Is(result, tt.wantError))
		})
	}
}

func TestWrapAuthError(t *testing.T) {
	integration := &model.Integration{ID: 42, Name: "test-infisical"}
	config := &model.InfisicalConfig{SiteURL: "https://app.infisical.com"}

	t.Run("timeout classifies as ErrProviderTimeout", func(t *testing.T) {
		err := wrapAuthError(integration, config, context.DeadlineExceeded)
		assert.ErrorIs(t, err, integrations.ErrProviderTimeout)
		assert.NotErrorIs(t, err, integrations.ErrAuthenticationFailed,
			"timeouts must NOT be reported as authentication failures")
		assert.Contains(t, err.Error(), "timed out")
	})

	t.Run("Infisical 4xx classifies as ErrAuthenticationFailed", func(t *testing.T) {
		apiErr := &sdkErrors.APIError{StatusCode: http.StatusUnauthorized}
		err := wrapAuthError(integration, config, apiErr)
		assert.ErrorIs(t, err, integrations.ErrAuthenticationFailed)
		assert.NotErrorIs(t, err, integrations.ErrProviderTimeout)
		assert.Contains(t, err.Error(), "client_id or client_secret")
		assert.Contains(t, err.Error(), "HTTP 401")
	})

	t.Run("Infisical 5xx classifies as ErrProviderConnectionFail", func(t *testing.T) {
		apiErr := &sdkErrors.APIError{StatusCode: http.StatusServiceUnavailable}
		err := wrapAuthError(integration, config, apiErr)
		assert.ErrorIs(t, err, integrations.ErrProviderConnectionFail)
		assert.NotErrorIs(t, err, integrations.ErrAuthenticationFailed,
			"5xx must NOT be reported as bad credentials")
		assert.Contains(t, err.Error(), "HTTP 503")
		assert.Contains(t, err.Error(), "unhealthy")
	})

	t.Run("generic network error classifies as ErrProviderConnectionFail", func(t *testing.T) {
		err := wrapAuthError(integration, config, errors.New("dial tcp: connection refused"))
		assert.ErrorIs(t, err, integrations.ErrProviderConnectionFail)
		assert.NotErrorIs(t, err, integrations.ErrAuthenticationFailed)
		assert.NotErrorIs(t, err, integrations.ErrProviderTimeout)
	})
}

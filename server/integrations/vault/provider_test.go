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

package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/server/integrations"
	"codefloe.com/crowci/crow/v5/server/model"
)

func TestNewProvider_InvalidJSON(t *testing.T) {
	integration := &model.Integration{
		Type:   model.IntegrationTypeVault,
		Config: "not json",
	}

	_, err := NewProvider(integration)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrProviderNotConfigured))
}

func TestNewProvider_WrongType(t *testing.T) {
	integration := &model.Integration{
		Type:   "other",
		Config: `{"address":"https://vault.example.com","role_id":"role","secret_id":"secret"}`,
	}

	_, err := NewProvider(integration)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, integrations.ErrProviderNotConfigured))
}

func TestNewProvider_InvalidTLS(t *testing.T) {
	tests := []struct {
		name        string
		tlsConfig   *model.VaultTLSConfig
		errContains string
	}{
		{
			name: "invalid ca cert",
			tlsConfig: &model.VaultTLSConfig{
				CACert: "not a valid certificate",
			},
			errContains: "failed to parse CA certificate",
		},
		{
			name: "invalid client cert pair",
			tlsConfig: &model.VaultTLSConfig{
				ClientCert: "not a valid certificate",
				ClientKey:  "not a valid key",
			},
			errContains: "failed to parse client certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := model.VaultConfig{
				Address:   "https://vault.example.com",
				RoleID:    "test-role",
				SecretID:  "test-secret",
				MountPath: "secret",
				AuthPath:  "auth/approle",
				TLSConfig: tt.tlsConfig,
			}
			configJSON, _ := json.Marshal(config)

			integration := &model.Integration{
				Type:   model.IntegrationTypeVault,
				Config: string(configJSON),
			}

			_, err := NewProvider(integration)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestBuildTLSConfig(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		config, err := buildTLSConfig(&model.VaultTLSConfig{})
		assert.NoError(t, err)
		assert.NotNil(t, config)
	})

	t.Run("insecure mode", func(t *testing.T) {
		tlsConfig := &model.VaultTLSConfig{
			Insecure: true,
		}
		config, err := buildTLSConfig(tlsConfig)
		assert.NoError(t, err)
		assert.True(t, config.InsecureSkipVerify)
	})

	t.Run("server name", func(t *testing.T) {
		tlsConfig := &model.VaultTLSConfig{
			TLSServerName: "vault.example.com",
		}
		config, err := buildTLSConfig(tlsConfig)
		assert.NoError(t, err)
		assert.Equal(t, "vault.example.com", config.ServerName)
	})

	t.Run("invalid ca cert", func(t *testing.T) {
		tlsConfig := &model.VaultTLSConfig{
			CACert: "not a valid certificate",
		}
		_, err := buildTLSConfig(tlsConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse CA certificate")
	})

	t.Run("invalid client cert pair", func(t *testing.T) {
		tlsConfig := &model.VaultTLSConfig{
			ClientCert: "not a certificate",
			ClientKey:  "not a key",
		}
		_, err := buildTLSConfig(tlsConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse client certificate")
	})

	t.Run("min tls version", func(t *testing.T) {
		config, err := buildTLSConfig(&model.VaultTLSConfig{})
		assert.NoError(t, err)
		// Should require TLS 1.2 minimum (0x0303)
		assert.Equal(t, uint16(0x0303), config.MinVersion)
	})
}

func TestProvider_Type(t *testing.T) {
	provider := &Provider{}
	assert.Equal(t, model.IntegrationTypeVault, provider.Type())
}

func TestNewProviderFunc(t *testing.T) {
	fn := NewProviderFunc()
	assert.NotNil(t, fn)

	// Test that the function returns error for invalid config
	integration := &model.Integration{
		Type:   model.IntegrationTypeVault,
		Config: "invalid json",
	}
	_, err := fn(integration)
	assert.Error(t, err)
}

func TestProvider_Close_NoToken(t *testing.T) {
	provider := &Provider{
		integration: &model.Integration{
			ID:   1,
			Name: "test",
		},
		token: "", // no token
	}

	// Close should not error when there's no token
	err := provider.Close()
	assert.NoError(t, err)
}

func TestNewSecretRequest(t *testing.T) {
	ref := &model.ExternalSecretRef{
		IntegrationID: 1,
		Path:          "secret/data/test",
		Key:           "api_key",
		Version:       5,
	}

	req := integrations.NewSecretRequest(ref)

	assert.Equal(t, ref.CacheKey(), req.ID)
	assert.Equal(t, ref.Path, req.Path)
	assert.Equal(t, ref.Key, req.Key)
	assert.Equal(t, ref.Version, req.Version)
}

func TestNewSecretRequest_NoVersion(t *testing.T) {
	ref := &model.ExternalSecretRef{
		IntegrationID: 1,
		Path:          "secret/data/test",
		Key:           "api_key",
		Version:       0,
	}

	req := integrations.NewSecretRequest(ref)

	assert.Equal(t, ref.CacheKey(), req.ID)
	assert.Equal(t, 0, req.Version)
}

func TestVaultConfig_WithDefaults(t *testing.T) {
	config := &model.VaultConfig{
		Address:  "https://vault.example.com",
		RoleID:   "role",
		SecretID: "secret",
	}

	result := config.WithDefaults()

	assert.Equal(t, "secret", result.MountPath)
	assert.Equal(t, "auth/approle", result.AuthPath)
}

func TestExternalSecretRef_Validate(t *testing.T) {
	tests := []struct {
		name        string
		ref         model.ExternalSecretRef
		wantErr     bool
		errContains string
	}{
		{
			name: "valid ref",
			ref: model.ExternalSecretRef{
				IntegrationID: 1,
				Path:          "secret/test",
				Key:           "api_key",
			},
			wantErr: false,
		},
		{
			name: "missing integration_id",
			ref: model.ExternalSecretRef{
				IntegrationID: 0,
				Path:          "secret/test",
				Key:           "api_key",
			},
			wantErr:     true,
			errContains: "integration_id is required",
		},
		{
			name: "missing path",
			ref: model.ExternalSecretRef{
				IntegrationID: 1,
				Path:          "",
				Key:           "api_key",
			},
			wantErr:     true,
			errContains: "path is required",
		},
		{
			name: "missing key",
			ref: model.ExternalSecretRef{
				IntegrationID: 1,
				Path:          "secret/test",
				Key:           "",
			},
			wantErr:     true,
			errContains: "key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ref.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsTimeoutError(t *testing.T) {
	t.Run("context deadline exceeded", func(t *testing.T) {
		assert.True(t, isTimeoutError(context.DeadlineExceeded))
	})

	t.Run("wrapped context deadline exceeded", func(t *testing.T) {
		wrapped := fmt.Errorf("doing thing: %w", context.DeadlineExceeded)
		assert.True(t, isTimeoutError(wrapped))
	})

	t.Run("net.Error timeout", func(t *testing.T) {
		urlErr := &url.Error{
			Op:  "Post",
			URL: "https://vault.example.com/v1/auth/approle/login",
			Err: &timeoutNetError{},
		}
		assert.True(t, isTimeoutError(urlErr))
	})

	t.Run("non-timeout error", func(t *testing.T) {
		assert.False(t, isTimeoutError(errors.New("connection refused")))
	})

	t.Run("nil", func(t *testing.T) {
		assert.False(t, isTimeoutError(nil))
	})
}

// timeoutNetError implements net.Error and reports Timeout()=true.
type timeoutNetError struct{}

func (*timeoutNetError) Error() string   { return "i/o timeout" }
func (*timeoutNetError) Timeout() bool   { return true }
func (*timeoutNetError) Temporary() bool { return true }

func TestProvider_WrapAuthError(t *testing.T) {
	p := &Provider{
		integration: &model.Integration{ID: 42, Name: "test-vault"},
		config: (&model.VaultConfig{
			Address: "https://vault.example.com",
		}).WithDefaults(),
	}

	t.Run("timeout classifies as ErrProviderTimeout", func(t *testing.T) {
		err := p.wrapAuthError(context.DeadlineExceeded)
		assert.ErrorIs(t, err, integrations.ErrProviderTimeout)
		assert.NotErrorIs(t, err, integrations.ErrAuthenticationFailed,
			"timeouts must NOT be reported as authentication failures")
		assert.Contains(t, err.Error(), "timed out")
		assert.Contains(t, err.Error(), "timeout_seconds")
	})

	t.Run("Vault 4xx classifies as ErrAuthenticationFailed", func(t *testing.T) {
		respErr := &vault.ResponseError{
			HTTPMethod: http.MethodPost,
			URL:        "https://vault.example.com/v1/auth/approle/login",
			StatusCode: http.StatusBadRequest,
			Errors:     []string{"invalid role or secret ID"},
		}
		err := p.wrapAuthError(respErr)
		assert.ErrorIs(t, err, integrations.ErrAuthenticationFailed)
		assert.NotErrorIs(t, err, integrations.ErrProviderTimeout)
		assert.Contains(t, err.Error(), "role_id or secret_id")
		assert.Contains(t, err.Error(), "HTTP 400")
	})

	t.Run("Vault 5xx classifies as ErrProviderConnectionFail", func(t *testing.T) {
		respErr := &vault.ResponseError{
			HTTPMethod: http.MethodPost,
			URL:        "https://vault.example.com/v1/auth/approle/login",
			StatusCode: http.StatusServiceUnavailable,
			Errors:     []string{"sealed"},
		}
		err := p.wrapAuthError(respErr)
		assert.ErrorIs(t, err, integrations.ErrProviderConnectionFail)
		assert.NotErrorIs(t, err, integrations.ErrAuthenticationFailed,
			"5xx must NOT be reported as bad credentials")
		assert.Contains(t, err.Error(), "HTTP 503")
		assert.Contains(t, err.Error(), "unhealthy")
	})

	t.Run("generic network error classifies as ErrProviderConnectionFail", func(t *testing.T) {
		err := p.wrapAuthError(errors.New("connection refused"))
		assert.ErrorIs(t, err, integrations.ErrProviderConnectionFail)
		assert.NotErrorIs(t, err, integrations.ErrAuthenticationFailed)
		assert.NotErrorIs(t, err, integrations.ErrProviderTimeout)
	})
}

func TestProvider_WrapReadError(t *testing.T) {
	p := &Provider{
		integration: &model.Integration{ID: 1, Name: "test"},
		config: (&model.VaultConfig{
			Address: "https://vault.example.com",
		}).WithDefaults(),
	}

	t.Run("timeout classifies as ErrProviderTimeout", func(t *testing.T) {
		err := p.wrapReadError(context.DeadlineExceeded, "secret/foo")
		assert.ErrorIs(t, err, integrations.ErrProviderTimeout)
		assert.Contains(t, err.Error(), "secret/foo")
	})

	t.Run("non-timeout falls back to connection error", func(t *testing.T) {
		err := p.wrapReadError(errors.New("eof"), "secret/foo")
		assert.ErrorIs(t, err, integrations.ErrProviderConnectionFail)
		assert.Contains(t, err.Error(), "secret/foo")
	})
}

// TestNewProvider_VaultRejectsCredentials exercises the auth-error path end-to-end
// against a fake Vault that returns HTTP 400 and verifies the resulting error
// is classified as a credentials failure rather than a generic connection error.
// Timeout classification is covered by direct unit tests on isTimeoutError and
// wrapAuthError above to avoid the slow Vault-SDK retry loop on a hanging server.
func TestNewProvider_VaultRejectsCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":["invalid role or secret ID"]}`))
	}))
	defer srv.Close()

	config := model.VaultConfig{
		Address:        srv.URL,
		RoleID:         "role",
		SecretID:       "secret",
		MountPath:      "secret",
		AuthPath:       "auth/approle",
		TimeoutSeconds: 5,
	}
	cfgJSON, err := json.Marshal(config)
	assert.NoError(t, err)

	integration := &model.Integration{
		ID:     7,
		Name:   "rejecting-vault",
		Type:   model.IntegrationTypeVault,
		Config: string(cfgJSON),
	}

	start := time.Now()
	_, err = NewProvider(integration)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.ErrorIs(t, err, integrations.ErrAuthenticationFailed,
		"4xx must be classified as ErrAuthenticationFailed, got: %v", err)
	assert.NotErrorIs(t, err, integrations.ErrProviderTimeout)
	assert.Contains(t, err.Error(), "role_id or secret_id")
	// Sanity: real auth-rejection should be near-instant, not waiting for retries.
	assert.Less(t, elapsed, 30*time.Second, "rejection should not block on retries")
}

func TestExternalSecretRef_CacheKey(t *testing.T) {
	t.Run("without version", func(t *testing.T) {
		ref := &model.ExternalSecretRef{
			IntegrationID: 1,
			Path:          "secret/test",
			Key:           "api_key",
			Version:       0,
		}
		assert.Equal(t, "1:secret/test:api_key", ref.CacheKey())
	})

	t.Run("with version", func(t *testing.T) {
		ref := &model.ExternalSecretRef{
			IntegrationID: 2,
			Path:          "secret/prod",
			Key:           "db_password",
			Version:       5,
		}
		assert.Equal(t, "2:secret/prod:db_password:v5", ref.CacheKey())
	})
}

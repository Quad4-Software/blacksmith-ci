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

package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIntegrationType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		itype    IntegrationType
		expected bool
	}{
		{name: "valid vault type", itype: IntegrationTypeVault, expected: true},
		{name: "invalid type", itype: "invalid", expected: false},
		{name: "empty type", itype: "", expected: false},
		{name: "case sensitive", itype: "VAULT", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.itype.IsValid())
		})
	}
}

func TestIntegration_Validate(t *testing.T) {
	validVaultConfig := `{"address":"https://vault.example.com","role_id":"test-role","secret_id":"test-secret","mount_path":"secret","auth_path":"auth/approle"}`

	tests := []struct {
		name        string
		integration Integration
		wantErr     bool
		errContains string
	}{
		{
			name: "valid integration",
			integration: Integration{
				Name:   "test-integration",
				Type:   IntegrationTypeVault,
				Config: validVaultConfig,
			},
			wantErr: false,
		},
		{
			name: "empty name",
			integration: Integration{
				Name:   "",
				Type:   IntegrationTypeVault,
				Config: validVaultConfig,
			},
			wantErr:     true,
			errContains: "name is required",
		},
		{
			name: "name too long",
			integration: Integration{
				Name:   strings.Repeat("a", 256),
				Type:   IntegrationTypeVault,
				Config: validVaultConfig,
			},
			wantErr:     true,
			errContains: "name too long",
		},
		{
			name: "invalid type",
			integration: Integration{
				Name:   "test",
				Type:   "invalid",
				Config: validVaultConfig,
			},
			wantErr:     true,
			errContains: "unsupported type",
		},
		{
			name: "empty config",
			integration: Integration{
				Name:   "test",
				Type:   IntegrationTypeVault,
				Config: "",
			},
			wantErr:     true,
			errContains: "configuration is required",
		},
		{
			name: "invalid json config",
			integration: Integration{
				Name:   "test",
				Type:   IntegrationTypeVault,
				Config: "not json",
			},
			wantErr: true,
		},
		{
			name: "vault config missing address",
			integration: Integration{
				Name:   "test",
				Type:   IntegrationTypeVault,
				Config: `{"role_id":"test","secret_id":"test"}`,
			},
			wantErr:     true,
			errContains: "address is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.integration.Validate()
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

func TestIntegration_HasAccess(t *testing.T) {
	tests := []struct {
		name        string
		integration Integration
		userID      int64
		orgID       int64
		repoID      int64
		expected    bool
	}{
		{
			name: "owner in user allowlist has access",
			integration: Integration{
				UserID:         1,
				AllowedUserIDs: []int64{1}, // owner explicitly allowlisted
			},
			userID:   1,
			expected: true,
		},
		{
			name: "owner without allowlist has no access",
			integration: Integration{
				UserID: 1,
				// no allowlists configured
			},
			userID:   1,
			expected: false,
		},
		{
			name: "user in allowlist",
			integration: Integration{
				UserID:         1,
				AllowedUserIDs: []int64{2, 3},
			},
			userID:   2,
			expected: true,
		},
		{
			name: "org in allowlist",
			integration: Integration{
				UserID:        1,
				AllowedOrgIDs: []int64{10, 20},
			},
			userID:   2,
			orgID:    10,
			expected: true,
		},
		{
			name: "repo in allowlist",
			integration: Integration{
				UserID:         1,
				AllowedRepoIDs: []int64{100, 200},
			},
			userID:   2,
			repoID:   100,
			expected: true,
		},
		{
			name: "no access - not in any allowlist",
			integration: Integration{
				UserID:         1,
				AllowedUserIDs: []int64{3},
				AllowedOrgIDs:  []int64{10},
				AllowedRepoIDs: []int64{100},
			},
			userID:   2,
			orgID:    20,
			repoID:   200,
			expected: false,
		},
		{
			name: "empty allowlists - no one has access",
			integration: Integration{
				UserID: 1,
			},
			userID:   2,
			expected: false,
		},
		{
			name: "owner's org in allowlist grants access",
			integration: Integration{
				UserID:        1,
				AllowedOrgIDs: []int64{10},
			},
			userID:   1,
			orgID:    10,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.integration.HasAccess(tt.userID, tt.orgID, tt.repoID))
		})
	}
}

func TestIntegration_Copy(t *testing.T) {
	original := &Integration{
		ID:             1,
		UserID:         2,
		Name:           "test",
		Type:           IntegrationTypeVault,
		Description:    "description",
		Config:         `{"address":"https://vault.example.com"}`,
		AllowedUserIDs: []int64{1, 2},
		AllowedOrgIDs:  []int64{3, 4},
		AllowedRepoIDs: []int64{5, 6},
		IsActive:       true,
	}

	copied := original.Copy()

	assert.Equal(t, original.ID, copied.ID)
	assert.Equal(t, original.Name, copied.Name)
	assert.Equal(t, original.Type, copied.Type)
	assert.Equal(t, original.Description, copied.Description)
	assert.Equal(t, original.IsActive, copied.IsActive)

	// Config should be intentionally omitted for security
	assert.Empty(t, copied.Config)

	// Allowlists should be copied (shallow copy is ok for this use case)
	assert.Equal(t, original.AllowedUserIDs, copied.AllowedUserIDs)
	assert.Equal(t, original.AllowedOrgIDs, copied.AllowedOrgIDs)
	assert.Equal(t, original.AllowedRepoIDs, copied.AllowedRepoIDs)
}

func TestIntegration_CopyWithRedactedConfig(t *testing.T) {
	config := VaultConfig{
		Address:   "https://vault.example.com",
		RoleID:    "12345678-1234-1234-1234-123456789012",
		SecretID:  "secret-value-that-should-be-hidden",
		MountPath: "secret",
		AuthPath:  "auth/approle",
	}
	configJSON, err := json.Marshal(config)
	assert.NoError(t, err)

	original := &Integration{
		ID:     1,
		Name:   "test",
		Type:   IntegrationTypeVault,
		Config: string(configJSON),
	}

	redacted := original.CopyWithRedactedConfig()

	// Config should be redacted
	var redactedConfig VaultConfig
	err = json.Unmarshal([]byte(redacted.Config), &redactedConfig)
	assert.NoError(t, err)

	// SecretID should be fully redacted
	assert.Equal(t, "***", redactedConfig.SecretID)

	// RoleID should be abbreviated
	assert.Contains(t, redactedConfig.RoleID, "...")

	// Address should not be changed
	assert.Equal(t, config.Address, redactedConfig.Address)
}

func TestIntegration_GetVaultConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := VaultConfig{
			Address:   "https://vault.example.com",
			RoleID:    "test-role",
			SecretID:  "test-secret",
			MountPath: "secret",
			AuthPath:  "auth/approle",
		}
		configJSON, err := json.Marshal(config)
		assert.NoError(t, err)

		integration := &Integration{
			Type:   IntegrationTypeVault,
			Config: string(configJSON),
		}

		result, err := integration.GetVaultConfig()
		assert.NoError(t, err)
		assert.Equal(t, config.Address, result.Address)
		assert.Equal(t, config.RoleID, result.RoleID)
	})

	t.Run("wrong type", func(t *testing.T) {
		integration := &Integration{
			Type:   "other",
			Config: "{}",
		}

		_, err := integration.GetVaultConfig()
		assert.Error(t, err)
	})

	t.Run("invalid json", func(t *testing.T) {
		integration := &Integration{
			Type:   IntegrationTypeVault,
			Config: "not json",
		}

		_, err := integration.GetVaultConfig()
		assert.Error(t, err)
	})
}

func TestVaultConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      VaultConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config",
			config: VaultConfig{
				Address:   "https://vault.example.com",
				RoleID:    "test-role",
				SecretID:  "test-secret",
				MountPath: "secret",
				AuthPath:  "auth/approle",
			},
			wantErr: false,
		},
		{
			name: "missing address",
			config: VaultConfig{
				RoleID:   "test-role",
				SecretID: "test-secret",
			},
			wantErr:     true,
			errContains: "address is required",
		},
		{
			name: "invalid address scheme",
			config: VaultConfig{
				Address:  "ftp://vault.example.com",
				RoleID:   "test-role",
				SecretID: "test-secret",
			},
			wantErr:     true,
			errContains: "http or https",
		},
		{
			name: "missing role_id",
			config: VaultConfig{
				Address:  "https://vault.example.com",
				SecretID: "test-secret",
			},
			wantErr:     true,
			errContains: "role_id is required",
		},
		{
			name: "missing secret_id",
			config: VaultConfig{
				Address: "https://vault.example.com",
				RoleID:  "test-role",
			},
			wantErr:     true,
			errContains: "secret_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
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

func TestVaultConfig_WithDefaults(t *testing.T) {
	config := VaultConfig{
		Address:  "https://vault.example.com",
		RoleID:   "test-role",
		SecretID: "test-secret",
	}

	result := config.WithDefaults()

	assert.Equal(t, "secret", result.MountPath)
	assert.Equal(t, "auth/approle", result.AuthPath)
	assert.Equal(t, config.Address, result.Address)
	assert.Equal(t, DefaultVaultTimeoutSeconds, result.TimeoutSeconds)
}

func TestVaultConfig_WithDefaults_KeepsExplicitTimeout(t *testing.T) {
	config := VaultConfig{
		Address:        "https://vault.example.com",
		RoleID:         "r",
		SecretID:       "s",
		TimeoutSeconds: 120,
	}

	result := config.WithDefaults()

	assert.Equal(t, 120, result.TimeoutSeconds, "explicit timeout must not be overwritten by default")
}

func TestVaultConfig_RequestTimeout(t *testing.T) {
	t.Run("unset uses default", func(t *testing.T) {
		c := &VaultConfig{}
		assert.Equal(t, time.Duration(DefaultVaultTimeoutSeconds)*time.Second, c.RequestTimeout())
	})

	t.Run("zero uses default", func(t *testing.T) {
		c := &VaultConfig{TimeoutSeconds: 0}
		assert.Equal(t, time.Duration(DefaultVaultTimeoutSeconds)*time.Second, c.RequestTimeout())
	})

	t.Run("negative uses default", func(t *testing.T) {
		c := &VaultConfig{TimeoutSeconds: -5}
		assert.Equal(t, time.Duration(DefaultVaultTimeoutSeconds)*time.Second, c.RequestTimeout())
	})

	t.Run("explicit value is honored", func(t *testing.T) {
		c := &VaultConfig{TimeoutSeconds: 90}
		assert.Equal(t, 90*time.Second, c.RequestTimeout())
	})
}

func TestVaultConfig_Redacted(t *testing.T) {
	config := &VaultConfig{
		Address:   "https://vault.example.com",
		Namespace: "test-ns",
		RoleID:    "12345678-1234-1234-1234-123456789012",
		SecretID:  "secret-value",
		MountPath: "secret",
		AuthPath:  "auth/approle",
		TLSConfig: &VaultTLSConfig{
			CACert:        "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
			ClientCert:    "-----BEGIN CERTIFICATE-----\nclient\n-----END CERTIFICATE-----",
			ClientKey:     "-----BEGIN RSA PRIVATE KEY-----\nkey\n-----END RSA PRIVATE KEY-----",
			TLSServerName: "vault.example.com",
			Insecure:      false,
		},
	}

	redacted := config.Redacted()

	// SecretID should be fully redacted
	assert.Equal(t, "***", redacted.SecretID)

	// RoleID should be abbreviated
	assert.Contains(t, redacted.RoleID, "...")
	assert.True(t, len(redacted.RoleID) < len(config.RoleID))

	// Non-sensitive fields should be preserved
	assert.Equal(t, config.Address, redacted.Address)
	assert.Equal(t, config.Namespace, redacted.Namespace)
	assert.Equal(t, config.MountPath, redacted.MountPath)
	assert.Equal(t, config.AuthPath, redacted.AuthPath)

	// TLS sensitive fields should be redacted
	assert.Equal(t, "***", redacted.TLSConfig.ClientKey)
	assert.Equal(t, "[certificate present]", redacted.TLSConfig.CACert)
	assert.Equal(t, "[certificate present]", redacted.TLSConfig.ClientCert)
}

func TestIntegration_BeforeInsert(t *testing.T) {
	integration := &Integration{}

	integration.BeforeInsert()

	assert.NotNil(t, integration.AllowedUserIDs)
	assert.NotNil(t, integration.AllowedOrgIDs)
	assert.NotNil(t, integration.AllowedRepoIDs)
	assert.Empty(t, integration.AllowedUserIDs)
	assert.Empty(t, integration.AllowedOrgIDs)
	assert.Empty(t, integration.AllowedRepoIDs)
}

func TestExternalSecretRef_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ref     ExternalSecretRef
		wantErr bool
	}{
		{"by id", ExternalSecretRef{IntegrationID: 1, Path: "p", Key: "k"}, false},
		{"by name", ExternalSecretRef{IntegrationName: "vault", Path: "p", Key: "k"}, false},
		{"neither id nor name", ExternalSecretRef{Path: "p", Key: "k"}, true},
		{"missing path", ExternalSecretRef{IntegrationName: "vault", Key: "k"}, true},
		{"missing key", ExternalSecretRef{IntegrationName: "vault", Path: "p"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ref.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExternalSecretRef_CacheKey(t *testing.T) {
	// ID- and name-based references must not collide, and the key must stay
	// stable so the fetch round-trip can map results back to their reference.
	idRef := ExternalSecretRef{IntegrationID: 1, Path: "db", Key: "value"}
	nameRef := ExternalSecretRef{IntegrationName: "vault", Path: "db", Key: "value"}

	assert.Equal(t, "1:db:value", idRef.CacheKey())
	assert.Equal(t, "name:vault:db:value", nameRef.CacheKey())
	assert.NotEqual(t, idRef.CacheKey(), nameRef.CacheKey())

	versioned := ExternalSecretRef{IntegrationName: "vault", Path: "db", Key: "value", Version: 3}
	assert.Equal(t, "name:vault:db:value:v3", versioned.CacheKey())
}

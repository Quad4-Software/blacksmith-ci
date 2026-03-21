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
	"errors"
	"fmt"
	"net/url"

	"github.com/uptrace/bun"
)

// Integration errors.
var (
	ErrIntegrationNameInvalid   = errors.New("invalid integration name")
	ErrIntegrationTypeInvalid   = errors.New("invalid integration type")
	ErrIntegrationConfigInvalid = errors.New("invalid integration configuration")
	ErrIntegrationAccessDenied  = errors.New("access denied to integration")
	ErrIntegrationNotFound      = errors.New("integration not found")
	ErrIntegrationInactive      = errors.New("integration is not active")
)

// IntegrationType represents the type of external integration.
type IntegrationType string

const (
	IntegrationTypeVault IntegrationType = "vault"
	// Future integration types.
	// IntegrationTypeAWS   IntegrationType = "aws_secrets_manager".
	// IntegrationTypeAzure IntegrationType = "azure_key_vault".
	// IntegrationTypeGCP   IntegrationType = "gcp_secret_manager".
)

const (
	// MaxIntegrationNameLength is the maximum allowed length for integration names.
	maxIntegrationNameLength = 255
	// AbbreviateSecretMinLength is the minimum length required for secret abbreviation.
	abbreviateSecretMinLength = 8
)

// ValidIntegrationTypes lists all supported integration types.
var ValidIntegrationTypes = []IntegrationType{
	IntegrationTypeVault,
}

// IsValid checks if the integration type is valid.
func (t IntegrationType) IsValid() bool {
	for _, valid := range ValidIntegrationTypes {
		if t == valid {
			return true
		}
	}
	return false
}

// IntegrationStore persists integration information to storage.
type IntegrationStore interface {
	IntegrationFind(int64) (*Integration, error)
	IntegrationFindByName(userID int64, name string) (*Integration, error)
	IntegrationList(userID int64, opts *ListOptions) ([]*Integration, error)
	IntegrationListAll() ([]*Integration, error)
	IntegrationListAccessible(userID int64, orgIDs []int64, repoIDs []int64, opts *ListOptions) ([]*Integration, error)
	IntegrationCreate(*Integration) error
	IntegrationUpdate(*Integration) error
	IntegrationDelete(*Integration) error
}

// Integration represents an external secret store integration.
type Integration struct {
	bun.BaseModel `bun:"table:integrations"`

	ID          int64           `json:"id"           bun:"id,pk,autoincrement"`
	UserID      int64           `json:"user_id"      bun:"user_id,notnull"`
	Name        string          `json:"name"         bun:"name,notnull,type:varchar(255)"`
	Type        IntegrationType `json:"type"         bun:"type,notnull,type:varchar(50)"`
	Description string          `json:"description"  bun:"description,type:text"`

	// Configuration (provider-specific, encrypted in storage)
	Config string `json:"config,omitempty" bun:"config,type:text"`

	// Access Control - who can use this integration
	AllowedUserIDs JSONSliceInt64 `json:"allowed_user_ids" bun:"allowed_user_ids,type:text"`
	AllowedOrgIDs  JSONSliceInt64 `json:"allowed_org_ids"  bun:"allowed_org_ids,type:text"`
	AllowedRepoIDs JSONSliceInt64 `json:"allowed_repo_ids" bun:"allowed_repo_ids,type:text"`

	// Status
	IsActive bool `json:"is_active" bun:"is_active,default:true"`

	// Metadata
	CreatedAt int64 `json:"created_at" bun:",nullzero"`
	UpdatedAt int64 `json:"updated_at" bun:",nullzero"`
} //	@name	Integration

// TableName returns the database table name.
func (Integration) TableName() string {
	return "integrations"
}

// BeforeInsert sets defaults before insertion.
func (i *Integration) BeforeInsert() {
	if i.AllowedUserIDs == nil {
		i.AllowedUserIDs = []int64{}
	}
	if i.AllowedOrgIDs == nil {
		i.AllowedOrgIDs = []int64{}
	}
	if i.AllowedRepoIDs == nil {
		i.AllowedRepoIDs = []int64{}
	}
}

// Validate validates the integration fields.
func (i *Integration) Validate() error {
	if len(i.Name) == 0 {
		return fmt.Errorf("%w: name is required", ErrIntegrationNameInvalid)
	}
	if len(i.Name) > maxIntegrationNameLength {
		return fmt.Errorf("%w: name too long (max 255 characters)", ErrIntegrationNameInvalid)
	}

	if !i.Type.IsValid() {
		return fmt.Errorf("%w: unsupported type '%s'", ErrIntegrationTypeInvalid, i.Type)
	}

	if len(i.Config) == 0 {
		return fmt.Errorf("%w: configuration is required", ErrIntegrationConfigInvalid)
	}

	// Validate type-specific configuration
	if i.Type == IntegrationTypeVault {
		var config VaultConfig
		if err := json.Unmarshal([]byte(i.Config), &config); err != nil {
			return errors.Join(ErrIntegrationConfigInvalid, err)
		}
		if err := config.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Copy returns a copy of the integration without sensitive configuration.
func (i *Integration) Copy() *Integration {
	return &Integration{
		ID:             i.ID,
		UserID:         i.UserID,
		Name:           i.Name,
		Type:           i.Type,
		Description:    i.Description,
		AllowedUserIDs: i.AllowedUserIDs,
		AllowedOrgIDs:  i.AllowedOrgIDs,
		AllowedRepoIDs: i.AllowedRepoIDs,
		IsActive:       i.IsActive,
		CreatedAt:      i.CreatedAt,
		UpdatedAt:      i.UpdatedAt,
		// Config intentionally omitted - contains sensitive data
	}
}

// CopyWithRedactedConfig returns a copy with configuration redacted for display.
func (i *Integration) CopyWithRedactedConfig() *Integration {
	copy := i.Copy()

	if i.Type == IntegrationTypeVault {
		var config VaultConfig
		if err := json.Unmarshal([]byte(i.Config), &config); err == nil {
			redacted := config.Redacted()
			if configJSON, err := json.Marshal(redacted); err == nil {
				copy.Config = string(configJSON)
			}
		}
	}

	return copy
}

// HasAccess checks if the given user/org/repo has access to use this integration.
// Access is explicitly granted via allowlists - if no allowlists are configured,
// no one can use the integration (owner can still manage it, but not use it for secrets).
func (i *Integration) HasAccess(userID int64, orgID int64, repoID int64) bool {
	// Check if any allowlists are configured
	hasAllowlists := len(i.AllowedUserIDs) > 0 || len(i.AllowedOrgIDs) > 0 || len(i.AllowedRepoIDs) > 0
	if !hasAllowlists {
		// No allowlists configured - deny all access
		return false
	}

	// Check user allowlist
	for _, allowedUserID := range i.AllowedUserIDs {
		if allowedUserID == userID {
			return true
		}
	}

	// Check org allowlist (if orgID provided)
	if orgID > 0 {
		for _, allowedOrgID := range i.AllowedOrgIDs {
			if allowedOrgID == orgID {
				return true
			}
		}
	}

	// Check repo allowlist (if repoID provided)
	if repoID > 0 {
		for _, allowedRepoID := range i.AllowedRepoIDs {
			if allowedRepoID == repoID {
				return true
			}
		}
	}

	return false
}

// GetVaultConfig parses and returns the Vault configuration.
func (i *Integration) GetVaultConfig() (*VaultConfig, error) {
	if i.Type != IntegrationTypeVault {
		return nil, fmt.Errorf("integration type is not vault")
	}

	var config VaultConfig
	if err := json.Unmarshal([]byte(i.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to parse vault config: %w", err)
	}

	return &config, nil
}

// SetVaultConfig sets the Vault configuration.
func (i *Integration) SetVaultConfig(config *VaultConfig) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal vault config: %w", err)
	}

	i.Type = IntegrationTypeVault
	i.Config = string(configJSON)
	return nil
}

// VaultConfig holds HashiCorp Vault/OpenBao specific configuration.
type VaultConfig struct {
	// Connection
	Address   string `json:"address"`             // Vault server URL (e.g., https://vault.example.com)
	Namespace string `json:"namespace,omitempty"` // Vault Enterprise namespace

	// AppRole Authentication
	RoleID   string `json:"role_id"`   // AppRole Role ID
	SecretID string `json:"secret_id"` // AppRole Secret ID (encrypted in storage)

	// Paths
	MountPath string `json:"mount_path"` // Secrets engine mount path (default: "secret")
	AuthPath  string `json:"auth_path"`  // Auth method path (default: "auth/approle")

	// TLS Configuration
	TLSConfig *VaultTLSConfig `json:"tls_config,omitempty"`
}

// VaultTLSConfig holds TLS configuration for Vault connections.
type VaultTLSConfig struct {
	CACert        string `json:"ca_cert,omitempty"`         // CA certificate (PEM)
	ClientCert    string `json:"client_cert,omitempty"`     // Client certificate (PEM)
	ClientKey     string `json:"client_key,omitempty"`      // Client private key (PEM)
	TLSServerName string `json:"tls_server_name,omitempty"` // Server name for SNI
	Insecure      bool   `json:"insecure,omitempty"`        // Skip TLS verification (for dev only)
}

// Validate validates the Vault configuration.
func (c *VaultConfig) Validate() error {
	if c.Address == "" {
		return fmt.Errorf("%w: vault address is required", ErrIntegrationConfigInvalid)
	}

	// Validate address is a valid URL
	u, err := url.Parse(c.Address)
	if err != nil {
		return errors.Join(ErrIntegrationConfigInvalid, fmt.Errorf("invalid vault address: %w", err))
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: vault address must use http or https scheme", ErrIntegrationConfigInvalid)
	}

	if c.RoleID == "" {
		return fmt.Errorf("%w: role_id is required for AppRole authentication", ErrIntegrationConfigInvalid)
	}

	if c.SecretID == "" {
		return fmt.Errorf("%w: secret_id is required for AppRole authentication", ErrIntegrationConfigInvalid)
	}

	if c.MountPath == "" {
		return fmt.Errorf("%w: mount_path is required", ErrIntegrationConfigInvalid)
	}

	return nil
}

// Redacted returns a copy with sensitive fields redacted.
func (c *VaultConfig) Redacted() *VaultConfig {
	redacted := &VaultConfig{
		Address:   c.Address,
		Namespace: c.Namespace,
		RoleID:    abbreviateSecret(c.RoleID),
		SecretID:  "***", // Always fully redact SecretID
		MountPath: c.MountPath,
		AuthPath:  c.AuthPath,
	}

	if c.TLSConfig != nil {
		redacted.TLSConfig = &VaultTLSConfig{
			TLSServerName: c.TLSConfig.TLSServerName,
			Insecure:      c.TLSConfig.Insecure,
			// Certificates redacted
			CACert:     redactCert(c.TLSConfig.CACert),
			ClientCert: redactCert(c.TLSConfig.ClientCert),
			ClientKey:  "***",
		}
	}

	return redacted
}

// WithDefaults returns a copy with default values applied.
func (c *VaultConfig) WithDefaults() *VaultConfig {
	config := *c

	if config.MountPath == "" {
		config.MountPath = "secret"
	}
	if config.AuthPath == "" {
		config.AuthPath = "auth/approle"
	}

	return &config
}

// abbreviateSecret returns an abbreviated version of a secret string.
func abbreviateSecret(value string) string {
	if len(value) <= abbreviateSecretMinLength {
		return "***"
	}
	return value[:4] + "..." + value[len(value)-4:]
}

// redactCert redacts a certificate, showing only if it's present.
func redactCert(cert string) string {
	if cert == "" {
		return ""
	}
	return "[certificate present]"
}

// ExternalSecretRef represents a reference to an external secret in YAML.
type ExternalSecretRef struct {
	IntegrationID int64  `json:"integration_id" yaml:"integration_id"`
	Path          string `json:"path"           yaml:"path"`
	Key           string `json:"key"            yaml:"key"`
	Version       int    `json:"version"        yaml:"version,omitempty"`
}

// Validate validates the external secret reference.
func (r *ExternalSecretRef) Validate() error {
	if r.IntegrationID <= 0 {
		return fmt.Errorf("integration_id is required")
	}
	if r.Path == "" {
		return fmt.Errorf("path is required")
	}
	if r.Key == "" {
		return fmt.Errorf("key is required")
	}
	return nil
}

// CacheKey returns a unique key for caching this secret.
func (r *ExternalSecretRef) CacheKey() string {
	if r.Version > 0 {
		return fmt.Sprintf("%d:%s:%s:v%d", r.IntegrationID, r.Path, r.Key, r.Version)
	}
	return fmt.Sprintf("%d:%s:%s", r.IntegrationID, r.Path, r.Key)
}

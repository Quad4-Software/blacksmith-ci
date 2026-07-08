package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/pipeline"
)

func TestHandlePipelineError(t *testing.T) {
	tests := []struct {
		err  error
		code int
	}{
		{
			err:  pipeline.ErrFiltered,
			code: http.StatusNoContent,
		},
		{
			err:  &pipeline.ErrNotFound{Msg: "pipeline not found"},
			code: http.StatusNotFound,
		},
		{
			err:  &pipeline.ErrBadRequest{Msg: "bad request error"},
			code: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		r := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(r)
		handlePipelineErr(c, tt.err)
		c.Writer.WriteHeaderNow() // require written header
		assert.Equal(t, tt.code, r.Code)
	}
}

func TestMergeIntegrationConfig(t *testing.T) {
	t.Run("unknown type returns new config unchanged", func(t *testing.T) {
		oldConfig := `{"key":"old"}`
		newConfig := `{"key":"new"}`

		result, err := mergeIntegrationConfig("unknown", oldConfig, newConfig)
		assert.NoError(t, err)
		assert.Equal(t, newConfig, result)
	})

	t.Run("vault preserves secret_id when redacted", func(t *testing.T) {
		oldConfig := model.VaultConfig{
			Address:   "https://vault.example.com",
			RoleID:    "real-role-id",
			SecretID:  "real-secret-id",
			MountPath: "secret",
			AuthPath:  "auth/approle",
		}
		oldJSON, _ := json.Marshal(oldConfig)

		newConfig := model.VaultConfig{
			Address:   "https://vault.example.com",
			RoleID:    "real-role-id",
			SecretID:  "***", // redacted
			MountPath: "secret",
			AuthPath:  "auth/approle",
		}
		newJSON, _ := json.Marshal(newConfig)

		result, err := mergeIntegrationConfig(model.IntegrationTypeVault, string(oldJSON), string(newJSON))
		assert.NoError(t, err)

		var mergedConfig model.VaultConfig
		err = json.Unmarshal([]byte(result), &mergedConfig)
		assert.NoError(t, err)
		assert.Equal(t, "real-secret-id", mergedConfig.SecretID)
	})

	t.Run("vault preserves secret_id when empty", func(t *testing.T) {
		oldConfig := model.VaultConfig{
			Address:   "https://vault.example.com",
			RoleID:    "real-role-id",
			SecretID:  "real-secret-id",
			MountPath: "secret",
			AuthPath:  "auth/approle",
		}
		oldJSON, _ := json.Marshal(oldConfig)

		newConfig := model.VaultConfig{
			Address:   "https://vault.example.com",
			RoleID:    "real-role-id",
			SecretID:  "", // empty
			MountPath: "secret",
			AuthPath:  "auth/approle",
		}
		newJSON, _ := json.Marshal(newConfig)

		result, err := mergeIntegrationConfig(model.IntegrationTypeVault, string(oldJSON), string(newJSON))
		assert.NoError(t, err)

		var mergedConfig model.VaultConfig
		err = json.Unmarshal([]byte(result), &mergedConfig)
		assert.NoError(t, err)
		assert.Equal(t, "real-secret-id", mergedConfig.SecretID)
	})

	t.Run("vault preserves role_id when abbreviated", func(t *testing.T) {
		oldConfig := model.VaultConfig{
			Address:   "https://vault.example.com",
			RoleID:    "12345678-1234-1234-1234-123456789012",
			SecretID:  "secret",
			MountPath: "secret",
			AuthPath:  "auth/approle",
		}
		oldJSON, _ := json.Marshal(oldConfig)

		newConfig := model.VaultConfig{
			Address:   "https://vault.example.com",
			RoleID:    "1234...9012", // abbreviated
			SecretID:  "secret",
			MountPath: "secret",
			AuthPath:  "auth/approle",
		}
		newJSON, _ := json.Marshal(newConfig)

		result, err := mergeIntegrationConfig(model.IntegrationTypeVault, string(oldJSON), string(newJSON))
		assert.NoError(t, err)

		var mergedConfig model.VaultConfig
		err = json.Unmarshal([]byte(result), &mergedConfig)
		assert.NoError(t, err)
		assert.Equal(t, "12345678-1234-1234-1234-123456789012", mergedConfig.RoleID)
	})

	t.Run("vault updates non-sensitive fields", func(t *testing.T) {
		oldConfig := model.VaultConfig{
			Address:   "https://old-vault.example.com",
			RoleID:    "role-id",
			SecretID:  "secret-id",
			MountPath: "old-mount",
			AuthPath:  "auth/approle",
		}
		oldJSON, _ := json.Marshal(oldConfig)

		newConfig := model.VaultConfig{
			Address:   "https://new-vault.example.com",
			RoleID:    "role-id",
			SecretID:  "***",
			MountPath: "new-mount",
			AuthPath:  "auth/new-approle",
		}
		newJSON, _ := json.Marshal(newConfig)

		result, err := mergeIntegrationConfig(model.IntegrationTypeVault, string(oldJSON), string(newJSON))
		assert.NoError(t, err)

		var mergedConfig model.VaultConfig
		err = json.Unmarshal([]byte(result), &mergedConfig)
		assert.NoError(t, err)

		// Non-sensitive fields should be updated
		assert.Equal(t, "https://new-vault.example.com", mergedConfig.Address)
		assert.Equal(t, "new-mount", mergedConfig.MountPath)
		assert.Equal(t, "auth/new-approle", mergedConfig.AuthPath)

		// Sensitive fields should be preserved
		assert.Equal(t, "secret-id", mergedConfig.SecretID)
	})

	t.Run("vault preserves TLS client key when redacted", func(t *testing.T) {
		oldConfig := model.VaultConfig{
			Address:  "https://vault.example.com",
			RoleID:   "role-id",
			SecretID: "secret-id",
			TLSConfig: &model.VaultTLSConfig{
				ClientKey:  "real-private-key",
				ClientCert: "real-cert",
				CACert:     "real-ca-cert",
			},
		}
		oldJSON, _ := json.Marshal(oldConfig)

		newConfig := model.VaultConfig{
			Address:  "https://vault.example.com",
			RoleID:   "role-id",
			SecretID: "***",
			TLSConfig: &model.VaultTLSConfig{
				ClientKey:  "***",
				ClientCert: "[certificate present]",
				CACert:     "[certificate present]",
			},
		}
		newJSON, _ := json.Marshal(newConfig)

		result, err := mergeIntegrationConfig(model.IntegrationTypeVault, string(oldJSON), string(newJSON))
		assert.NoError(t, err)

		var mergedConfig model.VaultConfig
		err = json.Unmarshal([]byte(result), &mergedConfig)
		assert.NoError(t, err)

		assert.Equal(t, "real-private-key", mergedConfig.TLSConfig.ClientKey)
		assert.Equal(t, "real-cert", mergedConfig.TLSConfig.ClientCert)
		assert.Equal(t, "real-ca-cert", mergedConfig.TLSConfig.CACert)
	})

	t.Run("vault updates secret when new value provided", func(t *testing.T) {
		oldConfig := model.VaultConfig{
			Address:  "https://vault.example.com",
			RoleID:   "role-id",
			SecretID: "old-secret-id",
		}
		oldJSON, _ := json.Marshal(oldConfig)

		newConfig := model.VaultConfig{
			Address:  "https://vault.example.com",
			RoleID:   "role-id",
			SecretID: "new-secret-id", // new value provided
		}
		newJSON, _ := json.Marshal(newConfig)

		result, err := mergeIntegrationConfig(model.IntegrationTypeVault, string(oldJSON), string(newJSON))
		assert.NoError(t, err)

		var mergedConfig model.VaultConfig
		err = json.Unmarshal([]byte(result), &mergedConfig)
		assert.NoError(t, err)
		assert.Equal(t, "new-secret-id", mergedConfig.SecretID)
	})

	t.Run("invalid old config returns new config", func(t *testing.T) {
		oldConfig := "not valid json"
		newConfig := `{"address":"https://vault.example.com","role_id":"role","secret_id":"secret"}`

		result, err := mergeIntegrationConfig(model.IntegrationTypeVault, oldConfig, newConfig)
		assert.NoError(t, err)
		assert.Equal(t, newConfig, result)
	})

	t.Run("invalid new config returns error", func(t *testing.T) {
		oldConfig := `{"address":"https://vault.example.com","role_id":"role","secret_id":"secret"}`
		newConfig := "not valid json"

		_, err := mergeIntegrationConfig(model.IntegrationTypeVault, oldConfig, newConfig)
		assert.Error(t, err)
	})

	t.Run("vault preserves credentials when secret_id field is omitted", func(t *testing.T) {
		// This simulates what the frontend sends when only updating allowlist:
		// - secret_id is omitted entirely from JSON (not sent when empty)
		// - role_id is abbreviated (shown in form as "1234...5678")
		oldConfig := `{"address":"https://vault.devxy.io","role_id":"real-role-id-12345","secret_id":"real-secret-id-67890","mount_path":"secret","auth_path":"auth/approle"}`
		newConfig := `{"address":"https://vault.devxy.io","role_id":"real...12345","mount_path":"secret"}`

		result, err := mergeIntegrationConfig(model.IntegrationTypeVault, oldConfig, newConfig)
		assert.NoError(t, err)

		var mergedConfig model.VaultConfig
		err = json.Unmarshal([]byte(result), &mergedConfig)
		assert.NoError(t, err)

		// Both credentials should be preserved
		assert.Equal(t, "real-role-id-12345", mergedConfig.RoleID, "role_id should be preserved when abbreviated")
		assert.Equal(t, "real-secret-id-67890", mergedConfig.SecretID, "secret_id should be preserved when omitted")
		// Other fields should remain
		assert.Equal(t, "https://vault.devxy.io", mergedConfig.Address)
		assert.Equal(t, "secret", mergedConfig.MountPath)
	})
}

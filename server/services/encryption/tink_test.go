// Copyright 2025 Crow Authors
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

package encryption

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyset"
)

// createTestKeyset creates a temporary keyset file for testing.
func createTestKeyset(t *testing.T) string {
	t.Helper()

	// Generate a new keyset handle with AES256-GCM-SIV
	handle, err := keyset.NewHandle(aead.AES256GCMSIVKeyTemplate())
	require.NoError(t, err)

	// Write keyset to temp file
	tmpDir := t.TempDir()
	keysetPath := filepath.Join(tmpDir, "test-keyset.json")

	f, err := os.Create(keysetPath)
	require.NoError(t, err)
	defer f.Close()

	writer := keyset.NewJSONWriter(f)
	err = insecurecleartextkeyset.Write(handle, writer)
	require.NoError(t, err)

	return keysetPath
}

func TestTinkEncryptDecrypt(t *testing.T) {
	keysetPath := createTestKeyset(t)

	svc := &tinkEncryptionService{
		keysetFilePath: keysetPath,
	}

	err := svc.loadKeyset()
	require.NoError(t, err)

	tests := []struct {
		name           string
		plaintext      string
		associatedData string
	}{
		{
			name:           "simple text",
			plaintext:      "hello world",
			associatedData: "1",
		},
		{
			name:           "empty associated data",
			plaintext:      "secret value",
			associatedData: "",
		},
		{
			name:           "unicode text",
			plaintext:      "secret with unicode: \u00e9\u00e0\u00fc\u4e2d\u6587",
			associatedData: "123",
		},
		{
			name:           "long text",
			plaintext:      string(make([]byte, 10000)),
			associatedData: "long-id",
		},
		{
			name:           "special characters",
			plaintext:      "p@ssw0rd!#$%^&*()_+-=[]{}|;':\",./<>?",
			associatedData: "special",
		},
		{
			name:           "multiline text",
			plaintext:      "line1\nline2\nline3",
			associatedData: "multiline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := svc.Encrypt(tt.plaintext, tt.associatedData)
			require.NoError(t, err)
			assert.NotEmpty(t, ciphertext)
			assert.NotEqual(t, tt.plaintext, ciphertext)

			// Decrypt
			decrypted, err := svc.Decrypt(ciphertext, tt.associatedData)
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}

func TestTinkDecryptWithWrongAssociatedData(t *testing.T) {
	keysetPath := createTestKeyset(t)

	svc := &tinkEncryptionService{
		keysetFilePath: keysetPath,
	}

	err := svc.loadKeyset()
	require.NoError(t, err)

	plaintext := "secret"
	ciphertext, err := svc.Encrypt(plaintext, "correct-id")
	require.NoError(t, err)

	// Decrypt with wrong associated data should fail
	_, err = svc.Decrypt(ciphertext, "wrong-id")
	assert.Error(t, err)
}

func TestTinkDecryptTamperedCiphertext(t *testing.T) {
	keysetPath := createTestKeyset(t)

	svc := &tinkEncryptionService{
		keysetFilePath: keysetPath,
	}

	err := svc.loadKeyset()
	require.NoError(t, err)

	plaintext := "secret"
	ciphertext, err := svc.Encrypt(plaintext, "1")
	require.NoError(t, err)

	// Tamper with the ciphertext
	tampered := ciphertext[:len(ciphertext)-5] + "XXXXX"

	_, err = svc.Decrypt(tampered, "1")
	assert.Error(t, err)
}

func TestTinkEncryptProducesUniqueCiphertexts(t *testing.T) {
	keysetPath := createTestKeyset(t)

	svc := &tinkEncryptionService{
		keysetFilePath: keysetPath,
	}

	err := svc.loadKeyset()
	require.NoError(t, err)

	plaintext := "same-secret"
	associatedData := "1"

	// Encrypt the same plaintext multiple times
	ciphertexts := make([]string, 10)
	for i := 0; i < 10; i++ {
		ct, err := svc.Encrypt(plaintext, associatedData)
		require.NoError(t, err)
		ciphertexts[i] = ct
	}

	// All ciphertexts should be unique (due to random nonce)
	seen := make(map[string]bool)
	for _, ct := range ciphertexts {
		assert.False(t, seen[ct], "duplicate ciphertext found")
		seen[ct] = true
	}

	// But all should decrypt to the same value
	for _, ct := range ciphertexts {
		decrypted, err := svc.Decrypt(ct, associatedData)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	}
}

func TestTinkDecryptInvalidBase64(t *testing.T) {
	keysetPath := createTestKeyset(t)

	svc := &tinkEncryptionService{
		keysetFilePath: keysetPath,
	}

	err := svc.loadKeyset()
	require.NoError(t, err)

	_, err = svc.Decrypt("not-valid-base64!!!", "1")
	assert.Error(t, err)
}

func TestTinkLoadKeysetFileNotFound(t *testing.T) {
	svc := &tinkEncryptionService{
		keysetFilePath: "/nonexistent/path/keyset.json",
	}

	err := svc.loadKeyset()
	assert.Error(t, err)
}

func TestTinkLoadKeysetInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	keysetPath := filepath.Join(tmpDir, "invalid-keyset.json")

	err := os.WriteFile(keysetPath, []byte("not valid json"), 0o600)
	require.NoError(t, err)

	svc := &tinkEncryptionService{
		keysetFilePath: keysetPath,
	}

	err = svc.loadKeyset()
	assert.Error(t, err)
}

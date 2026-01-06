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

package settings

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectExternalSecrets(t *testing.T) {
	tests := []struct {
		name         string
		input        any
		expectedRefs int
		checkRefs    func(t *testing.T, collected *CollectedExternalSecrets)
	}{
		{
			name:         "nil input",
			input:        nil,
			expectedRefs: 0,
		},
		{
			name:         "empty map",
			input:        map[string]any{},
			expectedRefs: 0,
		},
		{
			name: "single external secret with ID",
			input: map[string]any{
				"TEST_SECRET": map[string]any{
					"from_secret": map[string]any{
						"integration": 1,
						"path":        "secret/data/test",
						"key":         "api_key",
					},
				},
			},
			expectedRefs: 1,
			checkRefs: func(t *testing.T, collected *CollectedExternalSecrets) {
				ref := collected.Refs[0]
				assert.Equal(t, int64(1), ref.IntegrationID)
				assert.Empty(t, ref.IntegrationName)
				assert.Equal(t, "secret/data/test", ref.Path)
				assert.Equal(t, "api_key", ref.Key)
			},
		},
		{
			name: "single external secret with name",
			input: map[string]any{
				"TEST_SECRET": map[string]any{
					"from_secret": map[string]any{
						"integration": "vault-prod",
						"path":        "secret/data/test",
						"key":         "api_key",
					},
				},
			},
			expectedRefs: 1,
			checkRefs: func(t *testing.T, collected *CollectedExternalSecrets) {
				ref := collected.Refs[0]
				assert.Equal(t, int64(0), ref.IntegrationID)
				assert.Equal(t, "vault-prod", ref.IntegrationName)
				assert.Equal(t, "secret/data/test", ref.Path)
				assert.Equal(t, "api_key", ref.Key)
			},
		},
		{
			name: "internal secret string - should be ignored",
			input: map[string]any{
				"TEST_SECRET": map[string]any{
					"from_secret": "my_internal_secret",
				},
			},
			expectedRefs: 0, // String from_secret is internal, not collected
		},
		{
			name: "multiple external secrets same path",
			input: map[string]any{
				"SECRET1": map[string]any{
					"from_secret": map[string]any{
						"integration": 1,
						"path":        "secret/data/test",
						"key":         "key1",
					},
				},
				"SECRET2": map[string]any{
					"from_secret": map[string]any{
						"integration": 1,
						"path":        "secret/data/test",
						"key":         "key2",
					},
				},
			},
			expectedRefs: 2,
		},
		{
			name: "deduplication of identical refs",
			input: map[string]any{
				"SECRET1": map[string]any{
					"from_secret": map[string]any{
						"integration": 1,
						"path":        "secret/data/test",
						"key":         "key1",
					},
				},
				"SECRET2": map[string]any{
					"from_secret": map[string]any{
						"integration": 1,
						"path":        "secret/data/test",
						"key":         "key1", // same as SECRET1
					},
				},
			},
			expectedRefs: 1, // Should deduplicate
		},
		{
			name: "nested structure",
			input: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"from_secret": map[string]any{
							"integration": 1,
							"path":        "secret/nested",
							"key":         "nested_key",
						},
					},
				},
			},
			expectedRefs: 1,
		},
		{
			name: "array with external secrets",
			input: []any{
				map[string]any{
					"from_secret": map[string]any{
						"integration": 1,
						"path":        "secret/array",
						"key":         "item1",
					},
				},
				map[string]any{
					"from_secret": map[string]any{
						"integration": 2,
						"path":        "secret/array",
						"key":         "item2",
					},
				},
			},
			expectedRefs: 2,
		},
		{
			name: "mixed content with regular values",
			input: map[string]any{
				"REGULAR_VAR": "plain_value",
				"SECRET_VAR": map[string]any{
					"from_secret": map[string]any{
						"integration": 1,
						"path":        "secret/mixed",
						"key":         "secret_key",
					},
				},
				"ANOTHER_REGULAR": 42,
			},
			expectedRefs: 1,
		},
		{
			name: "mixed internal and external secrets",
			input: map[string]any{
				"INTERNAL_SECRET": map[string]any{
					"from_secret": "my_internal_secret", // string = internal
				},
				"EXTERNAL_SECRET": map[string]any{
					"from_secret": map[string]any{ // object = external
						"integration": "vault-prod",
						"path":        "secret/mixed",
						"key":         "external_key",
					},
				},
			},
			expectedRefs: 1, // Only external secret collected
		},
		{
			name: "with version specified",
			input: map[string]any{
				"VERSIONED_SECRET": map[string]any{
					"from_secret": map[string]any{
						"integration": 1,
						"path":        "secret/versioned",
						"key":         "key",
						"version":     5,
					},
				},
			},
			expectedRefs: 1,
			checkRefs: func(t *testing.T, collected *CollectedExternalSecrets) {
				ref := collected.Refs[0]
				assert.Equal(t, 5, ref.Version)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collected := CollectExternalSecrets(tt.input)

			assert.Len(t, collected.Refs, tt.expectedRefs)
			assert.Len(t, collected.RefsByKey, tt.expectedRefs)

			if tt.checkRefs != nil {
				tt.checkRefs(t, collected)
			}
		})
	}
}

func TestCollectedExternalSecrets_HasExternalSecrets(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		collected := &CollectedExternalSecrets{
			Refs:      []ExternalSecretRef{},
			RefsByKey: make(map[string]ExternalSecretRef),
		}
		assert.False(t, collected.HasExternalSecrets())
	})

	t.Run("with refs", func(t *testing.T) {
		collected := &CollectedExternalSecrets{
			Refs: []ExternalSecretRef{
				{IntegrationID: 1, Path: "test", Key: "key"},
			},
			RefsByKey: make(map[string]ExternalSecretRef),
		}
		assert.True(t, collected.HasExternalSecrets())
	})
}

func TestFetchExternalSecrets(t *testing.T) {
	ctx := context.Background()

	t.Run("empty refs", func(t *testing.T) {
		collected := &CollectedExternalSecrets{
			Refs:      []ExternalSecretRef{},
			RefsByKey: make(map[string]ExternalSecretRef),
		}

		fetched, err := FetchExternalSecrets(ctx, collected, nil)
		assert.NoError(t, err)
		assert.Empty(t, fetched.Secrets)
	})

	t.Run("nil fetcher with refs", func(t *testing.T) {
		collected := &CollectedExternalSecrets{
			Refs: []ExternalSecretRef{
				{IntegrationID: 1, Path: "test", Key: "key"},
			},
			RefsByKey: make(map[string]ExternalSecretRef),
		}

		_, err := FetchExternalSecrets(ctx, collected, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "external secret fetcher not configured")
	})

	t.Run("successful fetch", func(t *testing.T) {
		collected := &CollectedExternalSecrets{
			Refs: []ExternalSecretRef{
				{IntegrationID: 1, Path: "secret/test", Key: "api_key", Version: 0},
				{IntegrationID: 1, Path: "secret/test", Key: "api_secret", Version: 0},
			},
			RefsByKey: make(map[string]ExternalSecretRef),
		}
		for _, ref := range collected.Refs {
			collected.RefsByKey[externalSecretKey(ref)] = ref
		}

		fetcher := func(_ context.Context, ref ExternalSecretRef) (string, error) {
			if ref.Key == "api_key" {
				return "key_value_123", nil
			}
			return "secret_value_456", nil
		}

		fetched, err := FetchExternalSecrets(ctx, collected, fetcher)
		assert.NoError(t, err)
		assert.Len(t, fetched.Secrets, 2)
	})

	t.Run("fetch error", func(t *testing.T) {
		collected := &CollectedExternalSecrets{
			Refs: []ExternalSecretRef{
				{IntegrationID: 1, Path: "secret/test", Key: "key"},
			},
			RefsByKey: make(map[string]ExternalSecretRef),
		}

		fetcher := func(_ context.Context, _ ExternalSecretRef) (string, error) {
			return "", fmt.Errorf("vault unreachable")
		}

		_, err := FetchExternalSecrets(ctx, collected, fetcher)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch external secret")
	})
}

func TestInjectExternalSecrets(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		fetched := &FetchedExternalSecrets{Secrets: make(map[string]string)}
		result, err := InjectExternalSecrets(nil, fetched)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("simple string passthrough", func(t *testing.T) {
		fetched := &FetchedExternalSecrets{Secrets: make(map[string]string)}
		result, err := InjectExternalSecrets("plain_value", fetched)
		assert.NoError(t, err)
		assert.Equal(t, "plain_value", result)
	})

	t.Run("inject single external secret", func(t *testing.T) {
		ref := ExternalSecretRef{IntegrationID: 1, Path: "secret/test", Key: "api_key"}
		fetched := &FetchedExternalSecrets{
			Secrets: map[string]string{
				externalSecretKey(ref): "injected_value",
			},
		}

		input := map[string]any{
			"TEST_SECRET": map[string]any{
				"from_secret": map[string]any{
					"integration": int64(1),
					"path":        "secret/test",
					"key":         "api_key",
				},
			},
		}

		result, err := InjectExternalSecrets(input, fetched)
		assert.NoError(t, err)

		resultMap, ok := result.(map[string]any)
		require.True(t, ok, "expected result to be map[string]any")
		assert.Equal(t, "injected_value", resultMap["TEST_SECRET"])
	})

	t.Run("inject external secret with name", func(t *testing.T) {
		ref := ExternalSecretRef{IntegrationName: "vault-prod", Path: "secret/test", Key: "api_key"}
		fetched := &FetchedExternalSecrets{
			Secrets: map[string]string{
				externalSecretKey(ref): "named_injected_value",
			},
		}

		input := map[string]any{
			"TEST_SECRET": map[string]any{
				"from_secret": map[string]any{
					"integration": "vault-prod",
					"path":        "secret/test",
					"key":         "api_key",
				},
			},
		}

		result, err := InjectExternalSecrets(input, fetched)
		assert.NoError(t, err)

		resultMap, ok := result.(map[string]any)
		require.True(t, ok, "expected result to be map[string]any")
		assert.Equal(t, "named_injected_value", resultMap["TEST_SECRET"])
	})

	t.Run("internal secret passthrough", func(t *testing.T) {
		fetched := &FetchedExternalSecrets{Secrets: make(map[string]string)}

		input := map[string]any{
			"INTERNAL_SECRET": map[string]any{
				"from_secret": "my_internal_secret", // string = internal, not processed
			},
		}

		result, err := InjectExternalSecrets(input, fetched)
		assert.NoError(t, err)

		resultMap, ok := result.(map[string]any)
		require.True(t, ok, "expected result to be map[string]any")
		// Internal secret should pass through unchanged
		innerMap, ok := resultMap["INTERNAL_SECRET"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "my_internal_secret", innerMap["from_secret"])
	})

	t.Run("inject multiple secrets", func(t *testing.T) {
		ref1 := ExternalSecretRef{IntegrationID: 1, Path: "secret/test", Key: "key1"}
		ref2 := ExternalSecretRef{IntegrationID: 1, Path: "secret/test", Key: "key2"}
		fetched := &FetchedExternalSecrets{
			Secrets: map[string]string{
				externalSecretKey(ref1): "value1",
				externalSecretKey(ref2): "value2",
			},
		}

		input := map[string]any{
			"SECRET1": map[string]any{
				"from_secret": map[string]any{
					"integration": int64(1),
					"path":        "secret/test",
					"key":         "key1",
				},
			},
			"SECRET2": map[string]any{
				"from_secret": map[string]any{
					"integration": int64(1),
					"path":        "secret/test",
					"key":         "key2",
				},
			},
			"REGULAR": "plain_value",
		}

		result, err := InjectExternalSecrets(input, fetched)
		assert.NoError(t, err)

		resultMap, ok := result.(map[string]any)
		require.True(t, ok, "expected result to be map[string]any")
		assert.Equal(t, "value1", resultMap["SECRET1"])
		assert.Equal(t, "value2", resultMap["SECRET2"])
		assert.Equal(t, "plain_value", resultMap["REGULAR"])
	})

	t.Run("missing secret", func(t *testing.T) {
		fetched := &FetchedExternalSecrets{
			Secrets: make(map[string]string), // empty
		}

		input := map[string]any{
			"TEST_SECRET": map[string]any{
				"from_secret": map[string]any{
					"integration": int64(1),
					"path":        "secret/test",
					"key":         "missing_key",
				},
			},
		}

		_, err := InjectExternalSecrets(input, fetched)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "external secret not found")
	})

	t.Run("inject in array", func(t *testing.T) {
		ref := ExternalSecretRef{IntegrationID: 1, Path: "secret/test", Key: "key"}
		fetched := &FetchedExternalSecrets{
			Secrets: map[string]string{
				externalSecretKey(ref): "array_value",
			},
		}

		input := []any{
			"regular",
			map[string]any{
				"from_secret": map[string]any{
					"integration": int64(1),
					"path":        "secret/test",
					"key":         "key",
				},
			},
		}

		result, err := InjectExternalSecrets(input, fetched)
		assert.NoError(t, err)

		resultArr, ok := result.([]any)
		require.True(t, ok, "expected result to be []any")
		assert.Equal(t, "regular", resultArr[0])
		assert.Equal(t, "array_value", resultArr[1])
	})
}

func TestExternalSecretKey(t *testing.T) {
	tests := []struct {
		name     string
		ref      ExternalSecretRef
		expected string
	}{
		{
			name: "with ID",
			ref: ExternalSecretRef{
				IntegrationID: 1,
				Path:          "secret/test",
				Key:           "api_key",
				Version:       0,
			},
			expected: "id:1:secret/test:api_key:0",
		},
		{
			name: "with name",
			ref: ExternalSecretRef{
				IntegrationName: "vault-prod",
				Path:            "secret/test",
				Key:             "api_key",
				Version:         0,
			},
			expected: "name:vault-prod:secret/test:api_key:0",
		},
		{
			name: "with ID and version",
			ref: ExternalSecretRef{
				IntegrationID: 2,
				Path:          "secret/prod",
				Key:           "db_password",
				Version:       5,
			},
			expected: "id:2:secret/prod:db_password:5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := externalSecretKey(tt.ref)
			assert.Equal(t, tt.expected, key)
		})
	}
}

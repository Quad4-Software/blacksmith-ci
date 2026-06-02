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
	"reflect"
)

// CollectedExternalSecrets holds the result of scanning YAML for external secrets.
type CollectedExternalSecrets struct {
	// Refs are all unique external secret references found
	Refs []ExternalSecretRef
	// RefsByKey maps a unique key to the reference for deduplication
	RefsByKey map[string]ExternalSecretRef
}

// CollectExternalSecrets scans a value (typically YAML settings/environment)
// for external secret references (from_secret with object value) and collects them for batch fetching.
// Note: from_secret with a string value is an internal secret, handled separately.
func CollectExternalSecrets(v any) *CollectedExternalSecrets {
	collected := &CollectedExternalSecrets{
		Refs:      make([]ExternalSecretRef, 0),
		RefsByKey: make(map[string]ExternalSecretRef),
	}

	collectExternalSecretsRecursive(v, collected)
	return collected
}

// collectExternalSecretsRecursive recursively scans for external secret references.
func collectExternalSecretsRecursive(v any, collected *CollectedExternalSecrets) {
	if v == nil {
		return
	}

	t := reflect.TypeOf(v)
	vv := reflect.ValueOf(v)

	switch t.Kind() {
	case reflect.Map:
		if m, ok := v.(map[string]any); ok {
			// Check if this is an external secret reference (from_secret with object value)
			if secretRefI, ok := m["from_secret"]; ok {
				// Only process if it's an object (external secret)
				// String values are internal secrets, handled separately
				if _, isString := secretRefI.(string); !isString {
					ref, err := parseExternalSecretRef(secretRefI)
					if err == nil {
						key := externalSecretKey(ref)
						if _, exists := collected.RefsByKey[key]; !exists {
							collected.RefsByKey[key] = ref
							collected.Refs = append(collected.Refs, ref)
						}
					}
					return
				}
			}

			// Recurse into map values
			for _, val := range m {
				collectExternalSecretsRecursive(val, collected)
			}
		}

	case reflect.Slice, reflect.Array:
		for i := 0; i < vv.Len(); i++ {
			collectExternalSecretsRecursive(vv.Index(i).Interface(), collected)
		}
	}
}

// externalSecretKey creates a unique key for deduplication.
func externalSecretKey(ref ExternalSecretRef) string {
	if ref.IntegrationName != "" {
		return fmt.Sprintf("name:%s:%s:%s:%d", ref.IntegrationName, ref.Path, ref.Key, ref.Version)
	}
	return fmt.Sprintf("id:%d:%s:%s:%d", ref.IntegrationID, ref.Path, ref.Key, ref.Version)
}

// FetchedExternalSecrets holds secrets that have been fetched from external stores.
type FetchedExternalSecrets struct {
	// Secrets maps the unique key to the fetched secret value
	Secrets map[string]string
}

// FetchExternalSecrets fetches all collected external secrets using the provided batch fetcher.
// Returns a map that can be used to resolve references during YAML processing.
// A single batch call lets the fetcher authenticate against each integration only once,
// which avoids per-secret HTTP overhead when a workflow references many external secrets.
func FetchExternalSecrets(ctx context.Context, collected *CollectedExternalSecrets, fetcher ExternalSecretBatchFetcher) (*FetchedExternalSecrets, error) {
	fetched := &FetchedExternalSecrets{
		Secrets: make(map[string]string, len(collected.Refs)),
	}

	if len(collected.Refs) == 0 {
		return fetched, nil
	}

	if fetcher == nil {
		return nil, fmt.Errorf("external secret fetcher not configured but %d external secrets found", len(collected.Refs))
	}

	values, err := fetcher(ctx, collected.Refs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch external secrets: %w", err)
	}

	for _, ref := range collected.Refs {
		value, ok := values[ref]
		if !ok {
			integrationInfo := fmt.Sprintf("ID: %d", ref.IntegrationID)
			if ref.IntegrationName != "" {
				integrationInfo = fmt.Sprintf("Name: %s", ref.IntegrationName)
			}
			return nil, fmt.Errorf("external secret not returned by fetcher:\n\n"+
				"- **Integration:** %s\n"+
				"- **Path:** `%s`\n"+
				"- **Key:** `%s`",
				integrationInfo, ref.Path, ref.Key)
		}

		fetched.Secrets[externalSecretKey(ref)] = value
	}

	return fetched, nil
}

// InjectExternalSecrets replaces external secret references (from_secret with object value)
// with the actual fetched secret values.
func InjectExternalSecrets(v any, fetched *FetchedExternalSecrets) (any, error) {
	return injectExternalSecretsRecursive(v, fetched)
}

// injectExternalSecretsRecursive recursively replaces external secret references.
func injectExternalSecretsRecursive(v any, fetched *FetchedExternalSecrets) (any, error) {
	if v == nil {
		return v, nil
	}

	t := reflect.TypeOf(v)
	vv := reflect.ValueOf(v)

	switch t.Kind() {
	case reflect.Map:
		if m, ok := v.(map[string]any); ok {
			// Check if this is an external secret reference (from_secret with object value)
			if secretRefI, ok := m["from_secret"]; ok {
				// Only process if it's an object (external secret)
				// String values are internal secrets, handled separately
				if _, isString := secretRefI.(string); !isString {
					ref, err := parseExternalSecretRef(secretRefI)
					if err != nil {
						return nil, fmt.Errorf("invalid external secret reference: %w", err)
					}

					key := externalSecretKey(ref)
					value, ok := fetched.Secrets[key]
					if !ok {
						integrationInfo := fmt.Sprintf("id=%d", ref.IntegrationID)
						if ref.IntegrationName != "" {
							integrationInfo = fmt.Sprintf("name=%s", ref.IntegrationName)
						}
						return nil, fmt.Errorf("external secret not found (integration=%s, path=%s, key=%s)",
							integrationInfo, ref.Path, ref.Key)
					}

					return value, nil
				}
			}

			// Recurse into map values
			result := make(map[string]any, len(m))
			for k, val := range m {
				injected, err := injectExternalSecretsRecursive(val, fetched)
				if err != nil {
					return nil, err
				}
				result[k] = injected
			}
			return result, nil
		}

	case reflect.Slice, reflect.Array:
		result := make([]any, vv.Len())
		for i := 0; i < vv.Len(); i++ {
			injected, err := injectExternalSecretsRecursive(vv.Index(i).Interface(), fetched)
			if err != nil {
				return nil, err
			}
			result[i] = injected
		}
		return result, nil
	}

	return v, nil
}

// HasExternalSecrets checks if the collected secrets contain any references.
func (c *CollectedExternalSecrets) HasExternalSecrets() bool {
	return len(c.Refs) > 0
}

// Copyright 2022 Woodpecker Authors
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
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

// ExternalSecretRef represents a reference to an external secret store.
type ExternalSecretRef struct {
	IntegrationID   int64  // Numeric ID (used if IntegrationName is empty)
	IntegrationName string // Name lookup (takes precedence if set)
	Path            string
	Key             string
	Version         int
}

// ExternalSecretFetcher is a function that fetches secrets from external stores.
type ExternalSecretFetcher func(ctx context.Context, ref ExternalSecretRef) (string, error)

// ParamsToEnv uses reflection to convert a map[string]interface to a list
// of environment variables.
func ParamsToEnv(from map[string]any, to map[string]string, prefix string, upper bool, getSecretValue func(name string) (string, error), secretMapping map[string]string) (err error) {
	if to == nil {
		return fmt.Errorf("no map to write to")
	}
	for k, v := range from {
		if v == nil || len(k) == 0 {
			continue
		}
		sanitizedParamKey := sanitizeParamKey(prefix, upper, k)

		secretUsed := false
		wrappedGetSecretValue := func(name string) (string, error) {
			secretUsed = true
			return getSecretValue(name)
		}

		to[sanitizedParamKey], err = sanitizeParamValue(v, wrappedGetSecretValue)
		if err != nil {
			return err
		}

		if secretUsed && secretMapping != nil {
			secretMapping[sanitizedParamKey] = to[sanitizedParamKey]
		}
	}
	return nil
}

// sanitizeParamKey formats the environment variable key.
func sanitizeParamKey(prefix string, upper bool, k string) string {
	r := k
	if upper {
		r = strings.ReplaceAll(strings.ReplaceAll(k, ".", "_"), "-", "_")
		r = strings.ToUpper(r)
	}
	return prefix + r
}

// isComplex indicate if a data type can be turned into string without encoding as json.
func isComplex(t reflect.Kind) bool {
	switch t {
	case reflect.Bool,
		reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return false
	default:
		return true
	}
}

// sanitizeParamValue returns the value of a setting as string prepared to be injected as environment variable.
func sanitizeParamValue(v any, getSecretValue func(name string) (string, error)) (string, error) {
	t := reflect.TypeOf(v)
	vv := reflect.ValueOf(v)

	switch t.Kind() {
	case reflect.Bool:
		return strconv.FormatBool(vv.Bool()), nil

	case reflect.String:
		return vv.String(), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%v", vv.Int()), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%v", vv.Uint()), nil

	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%v", vv.Float()), nil

	case reflect.Map:
		switch v := v.(type) {
		// YAML unmarshalers emit this map interface
		case map[string]any:
			// check if it's a secret and return value if it's the case
			value, isSecret, err := injectSecret(v, getSecretValue)
			if err != nil {
				return "", err
			} else if isSecret {
				return value, nil
			}
		default:
			return "", fmt.Errorf("could not handle: %#v", v)
		}

		return handleComplex(vv.Interface(), getSecretValue)

	case reflect.Slice, reflect.Array:
		if vv.Len() == 0 {
			return "", nil
		}

		allPrimitive := true
		in := make([]string, vv.Len())
		for i := 0; i < vv.Len(); i++ {
			el := vv.Index(i).Interface()
			if el == nil {
				continue
			}
			// If any element is complex, we must serialize the whole slice as JSON
			if isComplex(reflect.TypeOf(el).Kind()) {
				allPrimitive = false
				break
			}
			var err error
			in[i], err = sanitizeParamValue(el, getSecretValue)
			if err != nil {
				return "", err
			}
		}
		if allPrimitive {
			return strings.Join(in, ","), nil
		}
	}

	// handle all elements which are not primitives, string-maps containing secrets or arrays
	return handleComplex(vv.Interface(), getSecretValue)
}

// handleComplex returns JSON strings as values for environment variables.
// It uses yaml.Marshal first to normalize field names (lowercasing),
// then converts to JSON for the final output.
func handleComplex(v any, getSecretValue func(name string) (string, error)) (string, error) {
	v, err := injectSecretRecursive(v, getSecretValue)
	if err != nil {
		return "", err
	}

	// First marshal to YAML to normalize field names (lowercase)
	yamlData, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}

	// Unmarshal back to a generic structure
	var normalized any
	if err := yaml.Unmarshal(yamlData, &normalized); err != nil {
		return "", err
	}

	// Finally marshal to JSON
	out, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SecretFetchers holds callbacks for fetching secrets from various sources.
type SecretFetchers struct {
	// GetInternalSecret fetches secrets from Crow's internal secret store
	GetInternalSecret func(name string) (string, error)
	// GetExternalSecret fetches secrets from external stores (Vault, etc.)
	GetExternalSecret ExternalSecretFetcher
	// Context for external secret operations
	Ctx context.Context
}

// injectSecret probes if a map is a from_secret request for an internal secret.
// If it's an internal secret request (from_secret: "string"), it returns the secret value.
// If from_secret is an object, it's an external secret reference which should be
// processed by InjectExternalSecrets first - this function skips those.
func injectSecret(v map[string]any, getSecretValue func(name string) (string, error)) (string, bool, error) {
	// Check for internal secret reference (from_secret: "string")
	if secretNameI, ok := v["from_secret"]; ok {
		// String value = internal secret
		if secretName, ok := secretNameI.(string); ok {
			secret, err := getSecretValue(secretName)
			if err != nil {
				return "", false, err
			}

			return secret, true, nil
		}
		// Object value = external secret (handled by InjectExternalSecrets)
		// Skip here - if external injection hasn't run, this will be caught later
		return "", false, nil
	}
	return "", false, nil
}

// parseExternalSecretRef parses an external secret reference from YAML.
// Expected format: {integration: 1 or "name", path: "secret/path", key: "keyname", version: 0}.
func parseExternalSecretRef(v any) (ExternalSecretRef, error) {
	ref := ExternalSecretRef{}

	m, ok := v.(map[string]any)
	if !ok {
		return ref, fmt.Errorf("external secret must be an object with integration, path, and key")
	}

	// integration (required) - can be ID (number) or name (string)
	if integrationI, ok := m["integration"]; ok {
		switch integration := integrationI.(type) {
		case string:
			ref.IntegrationName = integration
		case int:
			ref.IntegrationID = int64(integration)
		case int64:
			ref.IntegrationID = integration
		case uint64:
			ref.IntegrationID = int64(integration)
		case uint:
			ref.IntegrationID = int64(integration)
		case float64:
			ref.IntegrationID = int64(integration)
		default:
			return ref, fmt.Errorf("integration must be a string (name) or number (ID)")
		}
	} else {
		return ref, fmt.Errorf("integration is required")
	}

	// path (required)
	if pathI, ok := m["path"]; ok {
		if path, ok := pathI.(string); ok {
			ref.Path = path
		} else {
			return ref, fmt.Errorf("path must be a string")
		}
	} else {
		return ref, fmt.Errorf("path is required")
	}

	// key (required)
	if keyI, ok := m["key"]; ok {
		if key, ok := keyI.(string); ok {
			ref.Key = key
		} else {
			return ref, fmt.Errorf("key must be a string")
		}
	} else {
		return ref, fmt.Errorf("key is required")
	}

	// version (optional, defaults to 0 = latest)
	if versionI, ok := m["version"]; ok {
		switch version := versionI.(type) {
		case int:
			ref.Version = version
		case int64:
			ref.Version = int(version)
		case uint64:
			ref.Version = int(version)
		case uint:
			ref.Version = int(version)
		case float64:
			ref.Version = int(version)
		default:
			return ref, fmt.Errorf("version must be a number")
		}
	}

	return ref, nil
}

// injectSecretRecursive iterates over all types and if they contain elements
// it iterates recursively over them too, using injectSecret internally.
func injectSecretRecursive(v any, getSecretValue func(name string) (string, error)) (any, error) {
	if v == nil {
		return v, nil
	}

	t := reflect.TypeOf(v)
	if !isComplex(t.Kind()) {
		return v, nil
	}

	switch t.Kind() {
	case reflect.Map:
		switch v := v.(type) {
		// YAML unmarshalers emit this map interface
		case map[string]any:
			// handle secrets
			value, isSecret, err := injectSecret(v, getSecretValue)
			if err != nil {
				return nil, err
			} else if isSecret {
				return value, nil
			}

			for key, val := range v {
				v[key], err = injectSecretRecursive(val, getSecretValue)
				if err != nil {
					return nil, err
				}
			}
			return v, nil
		default:
			return v, fmt.Errorf("could not handle: %#v", v)
		}

	case reflect.Array, reflect.Slice:
		vv := reflect.ValueOf(v)
		vl := make([]any, vv.Len())

		for i := 0; i < vv.Len(); i++ {
			v, err := injectSecretRecursive(vv.Index(i).Interface(), getSecretValue)
			if err != nil {
				return nil, err
			}
			vl[i] = v
		}
		return vl, nil

	default:
		return v, nil
	}
}

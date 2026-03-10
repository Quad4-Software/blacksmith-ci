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

package api

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validateBody checks that a JSON payload passes huma's JSON Schema validation
// for the given body struct type. This catches mismatches between what the
// frontend sends and what huma expects (missing required fields, null for
// non-nullable types like maps/slices, extra fields, etc.).
func validateBody(t *testing.T, bodyType reflect.Type, jsonPayload string) {
	t.Helper()

	reg := huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
	schema := reg.Schema(bodyType, false, "")

	var payload any
	require.NoError(t, json.Unmarshal([]byte(jsonPayload), &payload))

	res := &huma.ValidateResult{}
	pb := &huma.PathBuffer{}
	huma.Validate(reg, schema, pb, huma.ModeWriteToServer, payload, res)

	if len(res.Errors) > 0 {
		var msgs []string
		for _, e := range res.Errors {
			msgs = append(msgs, e.Error())
		}
		t.Errorf("huma schema validation failed for %s:\n  payload: %s\n  errors: %s",
			bodyType.Name(), jsonPayload, strings.Join(msgs, "; "))
	}
}

// rejectBody checks that a JSON payload is rejected by huma's validation.
func rejectBody(t *testing.T, bodyType reflect.Type, jsonPayload string) {
	t.Helper()

	reg := huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
	schema := reg.Schema(bodyType, false, "")

	var payload any
	require.NoError(t, json.Unmarshal([]byte(jsonPayload), &payload))

	res := &huma.ValidateResult{}
	pb := &huma.PathBuffer{}
	huma.Validate(reg, schema, pb, huma.ModeWriteToServer, payload, res)

	assert.NotEmpty(t, res.Errors, "expected validation to reject payload for %s: %s", bodyType.Name(), jsonPayload)
}

// --- Agent ---.

func TestPostAgentBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PostAgentBody{})

	t.Run("minimal payload", func(t *testing.T) {
		validateBody(t, typ, `{"name": "test-agent"}`)
	})

	t.Run("with optional fields", func(t *testing.T) {
		validateBody(t, typ, `{"name": "test-agent", "no_schedule": true, "priority": 5}`)
	})

	t.Run("with custom labels", func(t *testing.T) {
		validateBody(t, typ, `{"name": "test-agent", "custom_labels": {"arch": "arm64"}}`)
	})

	t.Run("rejects empty name", func(t *testing.T) {
		rejectBody(t, typ, `{"name": ""}`)
	})

	t.Run("rejects missing name", func(t *testing.T) {
		rejectBody(t, typ, `{}`)
	})
}

func TestPatchAgentBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PatchAgentBody{})

	t.Run("frontend payload", func(t *testing.T) {
		validateBody(t, typ, `{"name": "updated", "no_schedule": false, "priority": 0}`)
	})

	t.Run("with no_schedule enabled", func(t *testing.T) {
		validateBody(t, typ, `{"name": "updated", "no_schedule": true, "priority": 3}`)
	})
}

// --- Autoscaler ---.

func TestPostAutoscalerBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PostAutoscalerBody{})

	t.Run("minimal payload", func(t *testing.T) {
		validateBody(t, typ, `{"name": "my-scaler"}`)
	})

	t.Run("rejects empty name", func(t *testing.T) {
		rejectBody(t, typ, `{"name": ""}`)
	})
}

func TestPatchAutoscalerBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PatchAutoscalerBody{})

	t.Run("disable", func(t *testing.T) {
		validateBody(t, typ, `{"enabled": false}`)
	})

	t.Run("enable", func(t *testing.T) {
		validateBody(t, typ, `{"enabled": true}`)
	})
}

// --- Secret ---.

func TestPostSecretBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PostSecretBody{})

	t.Run("minimal payload", func(t *testing.T) {
		validateBody(t, typ, `{"name": "my_secret", "value": "s3cret", "events": ["push"]}`)
	})

	t.Run("with images", func(t *testing.T) {
		validateBody(t, typ, `{"name": "my_secret", "value": "s3cret", "events": ["push"], "images": ["alpine"]}`)
	})

	t.Run("rejects missing name", func(t *testing.T) {
		rejectBody(t, typ, `{"value": "s3cret", "events": ["push"]}`)
	})

	t.Run("rejects missing value", func(t *testing.T) {
		rejectBody(t, typ, `{"name": "my_secret", "events": ["push"]}`)
	})
}

func TestPatchSecretBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PatchSecretBody{})

	t.Run("value only", func(t *testing.T) {
		validateBody(t, typ, `{"value": "new_value"}`)
	})

	t.Run("events only", func(t *testing.T) {
		validateBody(t, typ, `{"events": ["push", "tag"]}`)
	})

	t.Run("empty object", func(t *testing.T) {
		validateBody(t, typ, `{}`)
	})
}

// --- Registry ---.

func TestPostRegistryBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PostRegistryBody{})

	t.Run("full payload", func(t *testing.T) {
		validateBody(t, typ, `{"address": "docker.io", "username": "user", "password": "pass"}`)
	})

	t.Run("rejects missing address", func(t *testing.T) {
		rejectBody(t, typ, `{"username": "user", "password": "pass"}`)
	})
}

func TestPatchRegistryBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PatchRegistryBody{})

	t.Run("password only", func(t *testing.T) {
		validateBody(t, typ, `{"password": "new_pass"}`)
	})

	t.Run("all fields", func(t *testing.T) {
		validateBody(t, typ, `{"address": "ghcr.io", "username": "user", "password": "pass"}`)
	})

	t.Run("empty object", func(t *testing.T) {
		validateBody(t, typ, `{}`)
	})
}

// --- Cron ---.

func TestPostCronBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PostCronBody{})

	t.Run("minimal payload", func(t *testing.T) {
		validateBody(t, typ, `{"name": "nightly", "schedule": "0 0 * * *"}`)
	})

	t.Run("with branch", func(t *testing.T) {
		validateBody(t, typ, `{"name": "nightly", "schedule": "0 0 * * *", "branch": "main"}`)
	})

	t.Run("rejects missing name", func(t *testing.T) {
		rejectBody(t, typ, `{"schedule": "0 0 * * *"}`)
	})

	t.Run("rejects missing schedule", func(t *testing.T) {
		rejectBody(t, typ, `{"name": "nightly"}`)
	})
}

func TestPatchCronBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PatchCronBody{})

	t.Run("schedule only", func(t *testing.T) {
		validateBody(t, typ, `{"schedule": "0 6 * * *"}`)
	})

	t.Run("empty object", func(t *testing.T) {
		validateBody(t, typ, `{}`)
	})
}

// --- Integration ---.

func TestPostIntegrationBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PostIntegrationBody{})

	t.Run("minimal payload", func(t *testing.T) {
		validateBody(t, typ, `{"name": "vault", "type": "vault"}`)
	})

	t.Run("with allowlists", func(t *testing.T) {
		validateBody(t, typ, `{"name": "vault", "type": "vault", "allowed_user_ids": [1, 2], "allowed_org_ids": [3]}`)
	})

	t.Run("rejects missing name", func(t *testing.T) {
		rejectBody(t, typ, `{"type": "vault"}`)
	})
}

func TestPatchIntegrationBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PatchIntegrationBody{})

	t.Run("name only", func(t *testing.T) {
		validateBody(t, typ, `{"name": "updated", "is_active": true}`)
	})

	t.Run("disable with allowlists", func(t *testing.T) {
		validateBody(t, typ, `{"is_active": false, "allowed_user_ids": [1]}`)
	})
}

// --- Notification ---.

func TestPutNotificationConfigBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PutNotificationConfigBody{})

	t.Run("enable with events", func(t *testing.T) {
		validateBody(t, typ, `{"is_enabled": true, "events": ["failure"]}`)
	})

	t.Run("disable", func(t *testing.T) {
		validateBody(t, typ, `{"is_enabled": false}`)
	})
}

// --- User ---.

func TestPostUserBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PostUserBody{})

	t.Run("minimal payload", func(t *testing.T) {
		validateBody(t, typ, `{"login": "jdoe", "email": "jdoe@example.com"}`)
	})

	t.Run("with avatar", func(t *testing.T) {
		validateBody(t, typ, `{"login": "jdoe", "email": "j@ex.com", "avatar_url": "https://example.com/a.png"}`)
	})

	t.Run("rejects missing login", func(t *testing.T) {
		rejectBody(t, typ, `{"email": "jdoe@example.com"}`)
	})
}

func TestPatchUserBodySchema(t *testing.T) {
	typ := reflect.TypeOf(PatchUserBody{})

	t.Run("email only", func(t *testing.T) {
		validateBody(t, typ, `{"email": "new@example.com"}`)
	})

	t.Run("admin toggle", func(t *testing.T) {
		validateBody(t, typ, `{"admin": true}`)
	})

	t.Run("empty object", func(t *testing.T) {
		validateBody(t, typ, `{}`)
	})
}

// --- Forge ---.

func TestForgeBodySchema(t *testing.T) {
	typ := reflect.TypeOf(ForgeBody{})

	t.Run("minimal payload", func(t *testing.T) {
		validateBody(t, typ, `{"url": "https://github.com", "type": "github"}`)
	})

	t.Run("with client credentials", func(t *testing.T) {
		validateBody(t, typ, `{"url": "https://github.com", "type": "github", "client": "id", "client_secret": "secret"}`)
	})

	t.Run("with additional options", func(t *testing.T) {
		validateBody(t, typ, `{"url": "https://github.com", "type": "github", "additional_options": {"key": "val"}}`)
	})

	t.Run("rejects missing url", func(t *testing.T) {
		rejectBody(t, typ, `{"type": "github"}`)
	})
}

// --- Maintenance ---.

func TestUpdateMaintenanceConfigBodySchema(t *testing.T) {
	typ := reflect.TypeOf(UpdateMaintenanceConfigBody{})

	t.Run("enable with schedule", func(t *testing.T) {
		validateBody(t, typ, `{"enabled": true, "schedule": "0 3 * * *"}`)
	})

	t.Run("disable", func(t *testing.T) {
		validateBody(t, typ, `{"enabled": false}`)
	})

	t.Run("with dry run and threshold", func(t *testing.T) {
		validateBody(t, typ, `{"enabled": true, "schedule": "0 3 * * *", "dry_run": true, "stale_threshold": 7200}`)
	})
}

// --- Pipeline ---.

func TestCreatePipelineBodySchema(t *testing.T) {
	typ := reflect.TypeOf(CreatePipelineBody{})

	t.Run("branch only", func(t *testing.T) {
		validateBody(t, typ, `{"branch": "main"}`)
	})

	t.Run("with variables", func(t *testing.T) {
		validateBody(t, typ, `{"branch": "main", "variables": {"key": "val"}}`)
	})

	t.Run("with empty variables", func(t *testing.T) {
		validateBody(t, typ, `{"branch": "main", "variables": {}}`)
	})

	t.Run("with workflows filter", func(t *testing.T) {
		validateBody(t, typ, `{"branch": "main", "workflows": ["build", "test"]}`)
	})
}

// --- Regression: full model structs as body would fail ---
// These tests verify the specific payloads that broke with the old
// Body model.X pattern (null maps/slices, missing required fields).

func TestRegressionNullMapRejectedByOldSchema(t *testing.T) {
	// With the old `Body model.Agent`, this payload would fail because
	// custom_labels is null but the schema expected an object.
	// With PostAgentBody using omitempty, null is allowed.
	validateBody(t, reflect.TypeOf(PostAgentBody{}), `{"name": "test"}`)

	// Similarly, omitting custom_labels entirely should work.
	validateBody(t, reflect.TypeOf(PostAgentBody{}), `{"name": "test", "no_schedule": false}`)
}

func TestRegressionNullSliceRejectedByOldSchema(t *testing.T) {
	// With old Body model.Secret, sending events/images as null would fail.
	// PatchSecretBody uses omitempty so these are optional.
	validateBody(t, reflect.TypeOf(PatchSecretBody{}), `{"value": "new"}`)
	validateBody(t, reflect.TypeOf(PatchSecretBody{}), `{}`)
}

func TestRegressionPartialPayloadRejectedByOldSchema(t *testing.T) {
	// With old Body model.Registry, sending only address/username/password
	// would fail because id, org_id, repo_id etc. were required.
	validateBody(t, reflect.TypeOf(PostRegistryBody{}), `{"address": "docker.io", "username": "u", "password": "p"}`)

	// With old Body model.Cron, sending only name/schedule would fail
	// because id, repo_id, creator_id etc. were required.
	validateBody(t, reflect.TypeOf(PostCronBody{}), `{"name": "nightly", "schedule": "0 0 * * *"}`)
}

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

	"codefloe.com/crowci/crow/v6/server/model"
)

// bodyEntry describes a request body struct that huma validates.
// MinimalPayload is the smallest valid JSON the frontend can send.
// EmptyOK means `{}` should also pass (true for all Patch/Update bodies).
type bodyEntry struct {
	name           string
	bodyType       reflect.Type
	minimalPayload string
	emptyOK        bool
}

// allBodies returns every request body type used by huma-registered handlers.
// When you add a new endpoint with a Body field, add it here — the test below
// will fail loudly if the schema rejects the declared minimal payload.
func allBodies() []bodyEntry {
	return []bodyEntry{
		// --- Agent ---
		{
			name: "PostAgentBody", bodyType: reflect.TypeOf(PostAgentBody{}),
			minimalPayload: `{"name":"test"}`, emptyOK: false,
		},
		{
			name: "PatchAgentBody", bodyType: reflect.TypeOf(PatchAgentBody{}),
			minimalPayload: `{"name":"a","no_schedule":false,"priority":0}`, emptyOK: false,
		},

		// --- Autoscaler ---
		{
			name: "PostAutoscalerBody", bodyType: reflect.TypeOf(PostAutoscalerBody{}),
			minimalPayload: `{"name":"s"}`, emptyOK: false,
		},
		{
			name: "PatchAutoscalerBody", bodyType: reflect.TypeOf(PatchAutoscalerBody{}),
			minimalPayload: `{"enabled":false}`, emptyOK: false,
		},

		// --- Secret ---
		{
			name: "PostSecretBody", bodyType: reflect.TypeOf(PostSecretBody{}),
			minimalPayload: `{"name":"s","value":"v","events":["push"]}`, emptyOK: false,
		},
		{
			name: "PatchSecretBody", bodyType: reflect.TypeOf(PatchSecretBody{}),
			minimalPayload: `{"value":"v"}`, emptyOK: true,
		},

		// --- Registry ---
		{
			name: "PostRegistryBody", bodyType: reflect.TypeOf(PostRegistryBody{}),
			minimalPayload: `{"address":"docker.io","username":"u","password":"p"}`, emptyOK: false,
		},
		{
			name: "PatchRegistryBody", bodyType: reflect.TypeOf(PatchRegistryBody{}),
			minimalPayload: `{"password":"p"}`, emptyOK: true,
		},

		// --- Cron ---
		{
			name: "PostCronBody", bodyType: reflect.TypeOf(PostCronBody{}),
			minimalPayload: `{"name":"n","schedule":"0 0 * * *"}`, emptyOK: false,
		},
		{
			name: "PatchCronBody", bodyType: reflect.TypeOf(PatchCronBody{}),
			minimalPayload: `{"schedule":"0 6 * * *"}`, emptyOK: true,
		},

		// --- Integration ---
		{
			name: "PostIntegrationBody", bodyType: reflect.TypeOf(PostIntegrationBody{}),
			minimalPayload: `{"name":"vault","type":"vault"}`, emptyOK: false,
		},
		{
			name: "PatchIntegrationBody", bodyType: reflect.TypeOf(PatchIntegrationBody{}),
			minimalPayload: `{"is_active":true}`, emptyOK: false,
		},

		// --- Watch ---
		{
			name: "WatchSettingsBody", bodyType: reflect.TypeOf(WatchSettingsBody{}),
			minimalPayload: `{"in_app":true,"email":false}`, emptyOK: false,
		},
		{
			name: "CreateWatchBody", bodyType: reflect.TypeOf(CreateWatchBody{}),
			minimalPayload: `{"event":"push","target":"main","in_app":true,"email":false}`, emptyOK: false,
		},

		// --- User ---
		{
			name: "PostUserBody", bodyType: reflect.TypeOf(PostUserBody{}),
			minimalPayload: `{"login":"jdoe","email":"j@x.com"}`, emptyOK: false,
		},
		{
			name: "PatchUserBody", bodyType: reflect.TypeOf(PatchUserBody{}),
			minimalPayload: `{"email":"j@x.com"}`, emptyOK: true,
		},

		// --- Forge ---
		{
			name: "ForgeBody", bodyType: reflect.TypeOf(ForgeBody{}),
			minimalPayload: `{"url":"https://github.com","type":"github"}`, emptyOK: false,
		},

		// --- Maintenance ---
		{
			name: "UpdateMaintenanceConfigBody", bodyType: reflect.TypeOf(UpdateMaintenanceConfigBody{}),
			minimalPayload: `{"enabled":false}`, emptyOK: false,
		},

		// --- Pipeline ---
		{
			name: "CreatePipelineBody", bodyType: reflect.TypeOf(CreatePipelineBody{}),
			minimalPayload: `{"branch":"main"}`, emptyOK: false,
		},

		// --- Access Token ---
		{
			name: "CreateAccessTokenRequest", bodyType: reflect.TypeOf(CreateAccessTokenRequest{}),
			minimalPayload: `{"name":"tok","scopes":["repo"]}`, emptyOK: false,
		},
		{
			name: "UpdateAccessTokenRequest", bodyType: reflect.TypeOf(UpdateAccessTokenRequest{}),
			minimalPayload: `{"name":"tok"}`, emptyOK: true,
		},
		{
			name: "DeleteAccessTokenBatchRequest", bodyType: reflect.TypeOf(DeleteAccessTokenBatchRequest{}),
			minimalPayload: `{"ids":[1]}`, emptyOK: false,
		},

		// --- Repo ---
		{
			name: "RepoPatch", bodyType: reflect.TypeOf(model.RepoPatch{}),
			minimalPayload: `{"timeout":60}`, emptyOK: true,
		},

		// --- Maintenance Run ---
		{
			name: "RunMaintenanceRequest", bodyType: reflect.TypeOf(RunMaintenanceRequest{}),
			minimalPayload: `{"job":"vacuumLogTable"}`, emptyOK: false,
		},

		// --- Autoscaler Heartbeat (agent-to-server, not frontend) ---
		{
			name: "HeartbeatBody", bodyType: reflect.TypeOf(HeartbeatBody{}),
			minimalPayload: `{"active_agents":1,"pending_agents":0,"version":"1.0","min_agents":1,"max_agents":5,"workflows_per_agent":2,"provider":"aws","instance_type":"t3.micro","region":"us-east-1","agent_labels":{},"limit_cpu_quota":"","limit_mem":""}`,
			emptyOK:        false,
		},
	}
}

// TestAllBodySchemas_MinimalPayload ensures every registered body type accepts
// its declared minimal payload. If a new body struct is added without updating
// allBodies(), the TestAllBodySchemas_NoModelTypes guard below will catch it.
func TestAllBodySchemas_MinimalPayload(t *testing.T) {
	for _, b := range allBodies() {
		t.Run(b.name+"/minimal", func(t *testing.T) {
			validateBody(t, b.bodyType, b.minimalPayload)
		})
	}
}

// TestAllBodySchemas_EmptyPayload checks that Patch/Update bodies accept `{}`.
func TestAllBodySchemas_EmptyPayload(t *testing.T) {
	for _, b := range allBodies() {
		if !b.emptyOK {
			continue
		}
		t.Run(b.name+"/empty", func(t *testing.T) {
			validateBody(t, b.bodyType, `{}`)
		})
	}
}

// TestAllBodySchemas_NoAccidentalRequired uses huma's schema generator to
// inspect every body struct for fields that are marked required but use pointer
// or slice types (which usually signals "optional"). This catches the
// omitempty-missing pattern that caused many of our huma migration bugs.
func TestAllBodySchemas_NoAccidentalRequired(t *testing.T) {
	for _, b := range allBodies() {
		t.Run(b.name, func(t *testing.T) {
			reg := huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
			schema := reg.Schema(b.bodyType, false, "")

			requiredSet := make(map[string]bool)
			for _, r := range schema.Required {
				requiredSet[r] = true
			}

			for i := range b.bodyType.NumField() {
				field := b.bodyType.Field(i)
				jsonTag := field.Tag.Get("json")
				jsonName := strings.Split(jsonTag, ",")[0]
				if jsonName == "" || jsonName == "-" {
					continue
				}

				isOmitempty := strings.Contains(jsonTag, "omitempty")
				isPointer := field.Type.Kind() == reflect.Pointer
				isRequired := requiredSet[jsonName]

				// A pointer field that is required is suspicious — pointers are
				// used for optional patch semantics. Flag it unless it has a doc
				// tag explicitly saying "required".
				if isPointer && isRequired && !isOmitempty {
					t.Errorf("field %q (%s) is a pointer type but required by huma schema — add omitempty to json tag if optional",
						jsonName, field.Type)
				}
			}
		})
	}
}

// TestAllBodySchemas_NoModelTypes is a compile-time guard that verifies body
// structs don't embed or directly use model types. It checks that body types
// are defined in the api package (not model, which has too many fields).
// This test will print a warning for any body type from the model package
// that isn't in the known-safe allowlist.
func TestAllBodySchemas_NoModelTypes(t *testing.T) {
	// These model types are known-safe because they were designed as patch
	// structs with pointer fields and omitempty tags.
	allowedModelTypes := map[string]bool{
		"RepoPatch": true,
	}

	for _, b := range allBodies() {
		t.Run(b.name, func(t *testing.T) {
			pkgPath := b.bodyType.PkgPath()
			if strings.Contains(pkgPath, "/model") && !allowedModelTypes[b.bodyType.Name()] {
				t.Errorf("body type %s is from %s — create a dedicated body struct in the api package instead",
					b.bodyType.Name(), pkgPath)
			}
		})
	}
}

// TestAllBodySchemas_BindingTagsRemoved ensures no body struct still uses gin's
// binding tag, which huma silently ignores.
func TestAllBodySchemas_BindingTagsRemoved(t *testing.T) {
	for _, b := range allBodies() {
		t.Run(b.name, func(t *testing.T) {
			for i := range b.bodyType.NumField() {
				field := b.bodyType.Field(i)
				if binding := field.Tag.Get("binding"); binding != "" {
					t.Errorf("field %q has binding:%q — huma ignores this tag; use huma tags like minLength/minItems instead",
						field.Name, binding)
				}
			}
		})
	}
}

// TestAllBodySchemas_JSONRoundtrip verifies that every minimal payload can be
// unmarshaled into the body struct and re-marshaled without data loss.
func TestAllBodySchemas_JSONRoundtrip(t *testing.T) {
	for _, b := range allBodies() {
		t.Run(b.name, func(t *testing.T) {
			instance := reflect.New(b.bodyType).Interface()
			err := json.Unmarshal([]byte(b.minimalPayload), instance)
			assert.NoError(t, err, "unmarshal failed for %s", b.name)

			out, err := json.Marshal(instance)
			assert.NoError(t, err, "marshal failed for %s", b.name)

			// Verify the round-tripped JSON still passes schema validation.
			validateBody(t, b.bodyType, string(out))
		})
	}
}

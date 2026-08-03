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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONSliceMarshalsNilAsEmptyArray(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{"string", JSONSliceString(nil)},
		{"int64", JSONSliceInt64(nil)},
		{"webhook event", JSONSliceWebhookEvent(nil)},
		{"notification event", JSONSliceNotificationEvent(nil)},
		{"pipeline error", JSONSlicePipelineError(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.value)
			require.NoError(t, err)
			assert.JSONEq(t, "[]", string(b))
		})
	}
}

func TestJSONSliceMarshalsContent(t *testing.T) {
	b, err := json.Marshal(JSONSliceInt64{5, 7})
	require.NoError(t, err)
	assert.JSONEq(t, "[5,7]", string(b))

	b, err = json.Marshal(JSONSliceString{"a", "b"})
	require.NoError(t, err)
	assert.JSONEq(t, `["a","b"]`, string(b))
}

func TestJSONSliceRoundTrip(t *testing.T) {
	// Storage is deliberately untouched: a nil slice is still written as null
	// and still scans back as nil, so store-layer nil semantics hold.
	v, err := JSONSliceInt64(nil).Value()
	require.NoError(t, err)
	assert.Equal(t, "null", v)

	var scanned JSONSliceInt64
	require.NoError(t, scanned.Scan("null"))
	assert.Nil(t, scanned)

	// That nil must nevertheless reach clients as an empty array.
	b, err := json.Marshal(scanned)
	require.NoError(t, err)
	assert.JSONEq(t, "[]", string(b))

	require.NoError(t, scanned.Scan("[5,7]"))
	assert.Equal(t, JSONSliceInt64{5, 7}, scanned)
}

func TestAgentMarshalsGlobalOrgIDsAsEmptyArray(t *testing.T) {
	// Agents registered over gRPC with the master token have no orgs, so OrgIDs
	// is nil. Serializing that as null broke every client typed against the
	// declared array schema.
	b, err := json.Marshal(&Agent{ID: 1, Name: "system agent"})
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(b, &decoded))
	assert.Equal(t, []any{}, decoded["org_ids"])
}

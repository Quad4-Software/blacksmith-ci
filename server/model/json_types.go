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
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"codefloe.com/crowci/crow/v6/pipeline/errors/types"
)

// jsonValue marshals v to a JSON string for database storage.
func jsonValue(v any) (driver.Value, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("json marshal: %w", err)
	}
	return string(b), nil
}

// jsonScan unmarshals JSON from the database into dst.
func jsonScan(src any, dst any) error {
	if src == nil {
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("json scan: unsupported source type %T", src)
	}
	return json.Unmarshal(b, dst)
}

// jsonMarshalSlice renders a nil slice as an empty JSON array. The API declares
// these fields as arrays, so emitting null breaks clients that take the schema
// at its word and index into the value (a nil OrgIDs used to render the agent
// list unusable). Taking the argument as a plain []T drops the named type, so
// marshaling it does not recurse back into this method.
func jsonMarshalSlice[T any](j []T) ([]byte, error) {
	if j == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(j)
}

// jsonValueSlice stores a slice by its underlying type, bypassing the
// MarshalJSON above. Storage semantics stay as they were: a nil slice is
// written as null and scans back as nil, so nothing in the store layer has to
// care that the API now renders it as [].
func jsonValueSlice[T any](j []T) (driver.Value, error) {
	return jsonValue(j)
}

// JSONSliceString is a []string that serializes to/from JSON in the database.
type JSONSliceString []string

func (j JSONSliceString) Value() (driver.Value, error) { return jsonValueSlice(j) }
func (j *JSONSliceString) Scan(src any) error          { return jsonScan(src, j) }
func (j JSONSliceString) MarshalJSON() ([]byte, error) { return jsonMarshalSlice(j) }

// JSONSliceInt64 is a []int64 that serializes to/from JSON in the database.
type JSONSliceInt64 []int64

func (j JSONSliceInt64) Value() (driver.Value, error) { return jsonValueSlice(j) }
func (j *JSONSliceInt64) Scan(src any) error          { return jsonScan(src, j) }
func (j JSONSliceInt64) MarshalJSON() ([]byte, error) { return jsonMarshalSlice(j) }

// JSONMapStringString is a map[string]string that serializes to/from JSON in the database.
type JSONMapStringString map[string]string

func (j JSONMapStringString) Value() (driver.Value, error) { return jsonValue(j) }
func (j *JSONMapStringString) Scan(src any) error          { return jsonScan(src, j) }

// JSONMapStringAny is a map[string]any that serializes to/from JSON in the database.
type JSONMapStringAny map[string]any

func (j JSONMapStringAny) Value() (driver.Value, error) { return jsonValue(j) }
func (j *JSONMapStringAny) Scan(src any) error          { return jsonScan(src, j) }

// JSONSliceWebhookEvent is a []WebhookEvent that serializes to/from JSON in the database.
type JSONSliceWebhookEvent []WebhookEvent

func (j JSONSliceWebhookEvent) Value() (driver.Value, error) { return jsonValueSlice(j) }
func (j *JSONSliceWebhookEvent) Scan(src any) error          { return jsonScan(src, j) }
func (j JSONSliceWebhookEvent) MarshalJSON() ([]byte, error) { return jsonMarshalSlice(j) }

// JSONSliceNotificationEvent is a []NotificationEvent that serializes to/from JSON in the database.
type JSONSliceNotificationEvent []NotificationEvent

func (j JSONSliceNotificationEvent) Value() (driver.Value, error) { return jsonValueSlice(j) }
func (j *JSONSliceNotificationEvent) Scan(src any) error          { return jsonScan(src, j) }
func (j JSONSliceNotificationEvent) MarshalJSON() ([]byte, error) { return jsonMarshalSlice(j) }

// JSONMapStringStatusValue is a map[string]StatusValue that serializes to/from JSON in the database.
type JSONMapStringStatusValue map[string]StatusValue

func (j JSONMapStringStatusValue) Value() (driver.Value, error) { return jsonValue(j) }
func (j *JSONMapStringStatusValue) Scan(src any) error          { return jsonScan(src, j) }

// JSONSlicePipelineError is a []*types.PipelineError that serializes to/from JSON in the database.
type JSONSlicePipelineError []*types.PipelineError

func (j JSONSlicePipelineError) Value() (driver.Value, error) { return jsonValueSlice(j) }
func (j *JSONSlicePipelineError) Scan(src any) error          { return jsonScan(src, j) }
func (j JSONSlicePipelineError) MarshalJSON() ([]byte, error) { return jsonMarshalSlice(j) }

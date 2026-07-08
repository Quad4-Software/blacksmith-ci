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

// JSONSliceString is a []string that serializes to/from JSON in the database.
type JSONSliceString []string

func (j JSONSliceString) Value() (driver.Value, error) { return jsonValue(j) }
func (j *JSONSliceString) Scan(src any) error          { return jsonScan(src, j) }

// JSONSliceInt64 is a []int64 that serializes to/from JSON in the database.
type JSONSliceInt64 []int64

func (j JSONSliceInt64) Value() (driver.Value, error) { return jsonValue(j) }
func (j *JSONSliceInt64) Scan(src any) error          { return jsonScan(src, j) }

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

func (j JSONSliceWebhookEvent) Value() (driver.Value, error) { return jsonValue(j) }
func (j *JSONSliceWebhookEvent) Scan(src any) error          { return jsonScan(src, j) }

// JSONSliceNotificationEvent is a []NotificationEvent that serializes to/from JSON in the database.
type JSONSliceNotificationEvent []NotificationEvent

func (j JSONSliceNotificationEvent) Value() (driver.Value, error) { return jsonValue(j) }
func (j *JSONSliceNotificationEvent) Scan(src any) error          { return jsonScan(src, j) }

// JSONMapStringStatusValue is a map[string]StatusValue that serializes to/from JSON in the database.
type JSONMapStringStatusValue map[string]StatusValue

func (j JSONMapStringStatusValue) Value() (driver.Value, error) { return jsonValue(j) }
func (j *JSONMapStringStatusValue) Scan(src any) error          { return jsonScan(src, j) }

// JSONSlicePipelineError is a []*types.PipelineError that serializes to/from JSON in the database.
type JSONSlicePipelineError []*types.PipelineError

func (j JSONSlicePipelineError) Value() (driver.Value, error) { return jsonValue(j) }
func (j *JSONSlicePipelineError) Scan(src any) error          { return jsonScan(src, j) }

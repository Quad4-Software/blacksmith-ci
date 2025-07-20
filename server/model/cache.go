// Copyright 2025 Crow CI Authors
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

// CacheEntry represents a distributed cache entry.
type CacheEntry struct {
	Key       string `json:"key" xorm:"pk 'key'"`
	Data      []byte `json:"data" xorm:"LONGBLOB 'data'"`
	ExpiresAt int64  `json:"expires_at" xorm:"'expires_at'"`
	CreatedAt int64  `json:"created_at" xorm:"'created_at'"`
}

// TableName returns the table name for cache entries.
func (CacheEntry) TableName() string {
	return "cache_entries"
}

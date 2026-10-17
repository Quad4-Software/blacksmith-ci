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

import "github.com/uptrace/bun"

// BuildCache is one archive of workspace paths saved by a workflow's cache
// step under a key. Entries are immutable: a key is written once per scope and
// replaced only after it was evicted.
type BuildCache struct {
	bun.BaseModel `bun:"table:build_caches"`

	ID     int64 `json:"id"         bun:"id,pk,autoincrement"`
	RepoID int64 `json:"repo_id"    bun:"repo_id,notnull,unique:build_cache_key"`

	// Scope is the git ref the entry was saved from, e.g. "refs/heads/main".
	// A workflow reads its own scope and the default branch, and writes only
	// its own, so one branch cannot plant a cache another branch restores.
	Scope string `json:"scope"      bun:"scope,type:varchar(255),notnull,unique:build_cache_key"`

	// Key is the user-defined cache key after template rendering.
	Key string `json:"key"        bun:"cache_key,type:varchar(255),notnull,unique:build_cache_key"`

	// Size is the archive size in bytes: the reserved size while the upload is
	// pending, the stored size once committed.
	Size int64 `json:"size"       bun:"size,notnull,default:0"`

	// Committed is false while the archive is being uploaded. Pending entries
	// are never restored and are removed when their upload is abandoned.
	Committed bool  `json:"committed"  bun:"committed,notnull,default:false"`
	Created   int64 `json:"created"    bun:"created,notnull,default:0"`
	LastUsed  int64 `json:"last_used"  bun:"last_used,notnull,default:0"`
} //	@name	BuildCache

// TableName returns the database table name.
func (BuildCache) TableName() string {
	return "build_caches"
}

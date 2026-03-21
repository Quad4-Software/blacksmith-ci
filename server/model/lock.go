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

import "github.com/uptrace/bun"

// DistributedLock represents a distributed lock for coordinating operations
// across multiple server instances in HA deployments.
type DistributedLock struct {
	bun.BaseModel `bun:"table:distributed_locks"`

	ID         int64  `json:"id"          bun:"id,pk,autoincrement"`
	Name       string `json:"name"        bun:"name,unique,notnull"`
	InstanceID string `json:"instance_id" bun:"instance_id,notnull"`
	AcquiredAt int64  `json:"acquired_at" bun:"acquired_at"`
	ExpiresAt  int64  `json:"expires_at"  bun:"expires_at"`
} //	@name	DistributedLock

// TableName returns the table name for DistributedLock.
func (DistributedLock) TableName() string {
	return "distributed_locks"
}

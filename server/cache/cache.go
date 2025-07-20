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

package cache

import (
	"codeberg.org/crowci/crow/v4/server/store"
)

// CacheType represents the type of cache backend to use.
type CacheType string

const (
	// TypeMemory uses in-memory TTL cache (default for single instance).
	TypeMemory CacheType = "memory"

	// TypeDistributed uses database-backed distributed cache (for HA).
	TypeDistributed CacheType = "distributed"
)

// Config holds configuration for creating a cache service.
type Config struct {
	Backend CacheType
	Store   store.Store
}

// NewMembershipServiceWithConfig creates a membership service based on configuration.
func NewMembershipServiceWithConfig(config Config) MembershipService {
	switch config.Backend {
	case TypeDistributed:
		if config.Store == nil {
			// Fallback to memory cache if no store provided
			return NewMembershipService(nil)
		}
		return NewDistributedMembershipService(config.Store)
	default:
		return NewMembershipService(config.Store)
	}
}

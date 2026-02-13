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

package pubsub

import (
	"context"

	"codefloe.com/crowci/crow/v5/server/store"
)

// Publisher defines the interface for pub/sub operations.
type Publisher interface {
	Publish(message Message)
	Subscribe(c context.Context, receiver Receiver)
}

// Config holds the configuration for the pub/sub system.
type Config struct {
	Backend    Type
	Store      store.Store
	InstanceID string
}

// Type represents the pub/sub backend type.
type Type string

const (
	TypeMemory      Type = "memory"
	TypeDistributed Type = "distributed"
)

// New creates a new publisher based on the provided configuration.
func NewPublisher(config Config) Publisher {
	switch config.Backend {
	case TypeDistributed:
		if config.Store != nil && config.InstanceID != "" {
			return NewDistributed(config.Store, config.InstanceID)
		}
		// Fallback to memory if requirements not met
		return New()
	case TypeMemory:
		fallthrough
	default:
		return New()
	}
}

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

package model

import (
	"encoding/base32"
	"time"

	"github.com/tink-crypto/tink-go/v2/subtle/random"
	"github.com/uptrace/bun"
)

// Autoscaler represents an autoscaler instance that provisions agents on demand.
type Autoscaler struct {
	bun.BaseModel `bun:"table:autoscalers"`

	ID                int64               `json:"id"                    bun:"id,pk,autoincrement"`
	Created           int64               `json:"created"               bun:",nullzero"`
	Updated           int64               `json:"updated"               bun:",nullzero"`
	Name              string              `json:"name"                  bun:"name,unique,notnull"`
	Token             string              `json:"-"                     bun:"token"`
	OwnerID           int64               `json:"owner_id"              bun:"owner_id"`
	LastContact       int64               `json:"last_contact"          bun:"last_contact"`
	MinAgents         int32               `json:"min_agents"            bun:"min_agents"`
	MaxAgents         int32               `json:"max_agents"            bun:"max_agents"`
	WorkflowsPerAgent int32               `json:"workflows_per_agent"   bun:"workflows_per_agent"`
	Provider          string              `json:"provider"              bun:"provider,type:varchar(50)"`
	InstanceType      string              `json:"instance_type"         bun:"instance_type,type:varchar(100)"`
	Region            string              `json:"region"                bun:"region,type:varchar(100)"`
	AgentLabels       JSONMapStringString `json:"agent_labels"          bun:"agent_labels,type:text"`
	LimitCPUQuota     string              `json:"limit_cpu_quota"       bun:"limit_cpu_quota,type:varchar(50)"`
	LimitMem          string              `json:"limit_mem"             bun:"limit_mem,type:varchar(50)"`
	Enabled           bool                `json:"enabled"               bun:"enabled,default:true"`
	Version           string              `json:"version"               bun:"version"`
	ActiveAgents      int32               `json:"active_agents"         bun:"active_agents"`
	PendingAgents     int32               `json:"pending_agents"        bun:"pending_agents"`
	PendingSince      int64               `json:"pending_since"         bun:"pending_since"`
	OrgID             int64               `json:"org_id"                bun:"org_id"`
} //	@name	Autoscaler

const (
	// DefaultProvisionTimeoutSeconds is the default time after which pending agents
	// are considered stale (5 minutes).
	DefaultProvisionTimeoutSeconds = 300
)

// AutoscalerWithToken is used for API responses that include the token (only on creation).
type AutoscalerWithToken struct {
	*Autoscaler
	Token string `json:"token"`
} //	@name	AutoscalerWithToken

// AutoscalerListOptions defines filtering options for listing autoscalers.
type AutoscalerListOptions struct {
	ListOptions
	Search   string // Search in name, provider
	Provider string // Filter by provider
	Enabled  *bool  // Filter by enabled status (nil = all)
	OrgID    *int64 // Filter by org_id (nil = all, -1 = global autoscalers only)
	Sort     string // Sort field: id, name, provider, last_contact
	Order    string // Sort order: asc or desc
}

// TableName returns the database table name.
func (Autoscaler) TableName() string {
	return "autoscalers"
}

// IsSystemAutoscaler returns true if the autoscaler is a system-level (global) autoscaler.
func (a *Autoscaler) IsSystemAutoscaler() bool {
	return a.OrgID == IDNotSet
}

// GenerateNewAutoscalerToken generates a new random token for autoscaler authentication.
func GenerateNewAutoscalerToken() string {
	return base32.StdEncoding.EncodeToString(random.GetRandomBytes(32))
}

// CanProvision checks if the autoscaler can provision an agent with the given labels.
// This performs label matching similar to agent label matching logic.
func (a *Autoscaler) CanProvision(requiredLabels map[string]string) bool {
	if !a.Enabled {
		return false
	}

	// Check if at capacity
	if a.ActiveAgents >= a.MaxAgents {
		return false
	}

	// Check if autoscaler's agent labels match required labels
	for key, requiredValue := range requiredLabels {
		autoscalerValue, exists := a.AgentLabels[key]
		if !exists {
			return false
		}

		// Support wildcard matching
		if autoscalerValue != requiredValue && autoscalerValue != "*" {
			return false
		}
	}

	return true
}

// IsHealthy returns true if the autoscaler has sent a heartbeat recently (within 120 seconds).
func (a *Autoscaler) IsHealthy() bool {
	// TODO: Use a configurable threshold
	const autoscalerHealthyThresholdSeconds = 120
	return a.LastContact > 0 && (time.Now().Unix()-a.LastContact) < autoscalerHealthyThresholdSeconds
}

// IsProvisioningStale returns true if agents have been pending longer than the timeout.
func (a *Autoscaler) IsProvisioningStale() bool {
	if a.PendingAgents <= 0 || a.PendingSince <= 0 {
		return false
	}
	return (time.Now().Unix() - a.PendingSince) >= DefaultProvisionTimeoutSeconds
}

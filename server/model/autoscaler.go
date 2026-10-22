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
	"strconv"
	"strings"
	"time"

	"github.com/tink-crypto/tink-go/v2/subtle/random"
	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v6/pipeline"
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
	// PendingAgentIDs are the agents the autoscaler has started that have not
	// connected yet, as reported in its last heartbeat.
	PendingAgentIDs JSONSliceInt64 `json:"pending_agent_ids" bun:"pending_agent_ids,type:text"`
	// ProvisionTimeout is how long, in seconds, the autoscaler expects an agent
	// to take to connect. Zero means DefaultProvisionTimeoutSeconds.
	ProvisionTimeout int64 `json:"provision_timeout" bun:"provision_timeout"`
} //	@name	Autoscaler

const (
	// DefaultProvisionTimeoutSeconds is the time after which pending agents are
	// considered stale (5 minutes), unless the autoscaler reports its own.
	DefaultProvisionTimeoutSeconds = 300
)

// AutoscalerWithToken is used for API responses that include the token (only on creation).
type AutoscalerWithToken struct {
	Autoscaler
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
// The matching mirrors the queue's agent filter (see server/grpc/filter.go) so that a
// pending task is only flagged as "being provisioned" when an agent the autoscaler spins
// up would actually be eligible to run it.
func (a *Autoscaler) CanProvision(requiredLabels map[string]string) bool {
	if !a.Enabled {
		return false
	}

	// Check if at capacity
	if a.ActiveAgents >= a.MaxAgents {
		return false
	}

	for key, requiredValue := range requiredLabels {
		// Empty task labels are ignored, mirroring the queue filter.
		if requiredValue == "" {
			continue
		}

		switch key {
		case pipeline.LabelOrgID:
			// System (global) autoscalers serve every org; org-scoped autoscalers
			// only provision for their own org.
			if !a.IsSystemAutoscaler() && requiredValue != strconv.FormatInt(a.OrgID, 10) {
				return false
			}
			continue
		case pipeline.LabelRepoFullName:
			// Provisioned agents allow all repos by default (the agent repo filter
			// defaults to "*"), so repo scoping never blocks provisioning.
			continue
		}

		// Other reserved crowci/* labels are backend metadata and are not used for
		// matching, mirroring the queue filter.
		if strings.HasPrefix(key, pipeline.InternalLabelPrefix+"/") {
			continue
		}

		// Remaining labels (platform, backend, custom) must be satisfied by the
		// labels the autoscaler's agents advertise.
		autoscalerValue, exists := a.AgentLabels[key]
		if !exists {
			return false
		}
		if autoscalerValue != "*" && autoscalerValue != requiredValue {
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
	return (time.Now().Unix() - a.PendingSince) >= a.ProvisionTimeoutSeconds()
}

// ProvisionTimeoutSeconds returns how long the autoscaler expects an agent to
// take to connect.
func (a *Autoscaler) ProvisionTimeoutSeconds() int64 {
	if a.ProvisionTimeout > 0 {
		return a.ProvisionTimeout
	}
	return DefaultProvisionTimeoutSeconds
}

// Copyright 2021 Woodpecker Authors
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
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"

	"github.com/tink-crypto/tink-go/v2/subtle/random"
	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v5/pipeline"
)

type Agent struct {
	bun.BaseModel `bun:"table:agents"`

	ID                     int64               `json:"id"            bun:"id,pk,autoincrement"`
	Created                int64               `json:"created"       bun:",nullzero"`
	Updated                int64               `json:"updated"       bun:",nullzero"`
	Name                   string              `json:"name"          bun:"name"`
	OwnerID                int64               `json:"owner_id"      bun:"owner_id"`
	Token                  string              `json:"token"                     bun:"token"`
	PersistentID           string              `json:"persistent_id"             bun:"persistent_id,type:varchar(64)"`
	LastContact            int64               `json:"last_contact"              bun:"last_contact"`
	WorkflowPollingHealthy bool                `json:"workflow_polling_healthy"  bun:"workflow_polling_healthy"`
	LastWork               int64               `json:"last_work"                 bun:"last_work"`
	Platform               string              `json:"platform"                  bun:"platform,type:varchar(100)"`
	Backend                string              `json:"backend"       bun:"backend,type:varchar(100)"`
	Hostname               string              `json:"hostname"      bun:"hostname,type:varchar(255)"`
	Capacity               int32               `json:"capacity"      bun:"capacity"`
	Version                string              `json:"version"       bun:"version"`
	BuildDate              string              `json:"build_date"    bun:"build_date"`
	NoSchedule             bool                `json:"no_schedule"   bun:"no_schedule"`
	Priority               int32               `json:"priority"      bun:"priority,default:0"`
	CustomLabels           JSONMapStringString `json:"custom_labels"    bun:"custom_labels,type:text"`
	LimitMem               int64               `json:"limit_mem"        bun:"limit_mem"`
	LimitCPUQuota          int64               `json:"limit_cpu_quota"  bun:"limit_cpu_quota"`
	// OrgID is counted as unset if set to -1, this is done to ensure a new(Agent) still enforce the OrgID check by default
	OrgID int64 `json:"org_id"        bun:"org_id"`
} //	@name	Agent

// AgentWithStats is used for API responses to include runtime stats.
type AgentWithStats struct {
	Agent
	RunningWorkflows int `json:"running_workflows"`
}

const (
	IDNotSet = -1
)

// AgentListOptions defines filtering options for listing agents.
type AgentListOptions struct {
	ListOptions
	Search   string // Search in name, platform, backend, custom labels
	Platform string // Filter by platform
	Backend  string // Filter by backend
	OrgID    *int64 // Filter by org_id (nil = all, -1 = global agents only)
	Sort     string // Sort field: id, name, platform, backend, last_contact
	Order    string // Sort order: asc or desc
}

// TableName returns the database table name.
func (Agent) TableName() string {
	return "agents"
}

func (a *Agent) IsSystemAgent() bool {
	return a.OwnerID == IDNotSet
}

func GenerateNewAgentToken() string {
	return base32.StdEncoding.EncodeToString(random.GetRandomBytes(32))
}

// GeneratePersistentID creates a stable identity hash for an agent.
// For individual token agents, the token alone provides uniqueness.
// For master token agents, the name (hostname) differentiates agents sharing the same token.
// This hash persists across agent restarts and ID changes, allowing metrics aggregation.
func GeneratePersistentID(token, name, platform, backend string) string {
	// Combine token + name + platform + backend for a stable identity
	// - Token ensures different tokens = different agents
	// - Name (hostname) differentiates master token agents on different hosts
	// - Platform + backend provide additional stability
	data := fmt.Sprintf("%s|%s|%s|%s", token, name, platform, backend)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (a *Agent) GetServerLabels() (map[string]string, error) {
	filters := make(map[string]string)

	// enforce filters for user and organization agents
	if a.OrgID != IDNotSet {
		filters[pipeline.LabelOrgID] = fmt.Sprintf("%d", a.OrgID)
	} else {
		filters[pipeline.LabelOrgID] = "*"
	}

	return filters, nil
}

func (a *Agent) CanAccessRepo(repo *Repo) bool {
	// global agent
	if a.OrgID == IDNotSet {
		return true
	}

	// agent has access to the organization
	if a.OrgID == repo.OrgID {
		return true
	}

	return false
}

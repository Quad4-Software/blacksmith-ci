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
	"slices"
	"strconv"
	"strings"

	"github.com/tink-crypto/tink-go/v2/subtle/random"
	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v6/pipeline"
)

type Agent struct {
	bun.BaseModel `bun:"table:agents"`

	ID                     int64               `json:"id"            bun:"id,pk,autoincrement"`
	Created                int64               `json:"created"       bun:",nullzero"`
	Updated                int64               `json:"updated"       bun:",nullzero"`
	Name                   string              `json:"name"          bun:"name"`
	OwnerID                int64               `json:"owner_id"      bun:"owner_id"`
	Token                  string              `json:"-"                         bun:"token"`
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
	// HostCPUs and HostMem are reported by the agent and describe the machine
	// running the workloads, not the machine running the agent process. They are
	// zero for backends that cannot determine them (e.g. Kubernetes).
	HostCPUs int32 `json:"host_cpus" bun:"host_cpus"`
	HostMem  int64 `json:"host_mem"  bun:"host_mem"`
	// ProtocolVersion is the agent protocol version reported on registration.
	// Zero means the agent has not registered since the server started recording it.
	ProtocolVersion int32 `json:"protocol_version" bun:"protocol_version"`
	// RemoteUpdate is how the agent reacts to a restart request, "restart" or
	// empty when it ignores them. Reported by the agent on registration.
	RemoteUpdate string `json:"remote_update" bun:"remote_update,type:varchar(20)"`
	// RestartRequested makes the server stop handing the agent work and ask it
	// to exit once idle. Cleared when the agent registers again. Written only
	// through AgentSetRestartRequested, so a concurrent AgentUpdate from a
	// health report cannot undo a request.
	RestartRequested bool `json:"restart_requested" bun:"restart_requested"`
	// RestartQueued marks the agent for a restart that waits for a free slot
	// under the parallel restart limit. Written only through the restart store
	// methods, like RestartRequested.
	RestartQueued bool `json:"restart_queued" bun:"restart_queued"`
	// AutoRestartVersion is the server release under which the automatic
	// restart policy last asked the agent to restart. The policy asks at most
	// once per server release, so an agent that comes back on the same old
	// release is not restarted in a loop.
	AutoRestartVersion string `json:"-" bun:"auto_restart_version,type:varchar(255)"`
	// OrgIDs scopes the agent to one or more organizations. An empty list means
	// the agent is global and can process tasks from any organization.
	OrgIDs JSONSliceInt64 `json:"org_ids" bun:"org_ids,type:text"`
} //	@name	Agent

// AgentWithStats is used for API responses to include runtime stats.
type AgentWithStats struct {
	Agent
	RunningWorkflows int `json:"running_workflows"`
}

// AgentWithToken is used for API responses that include the token (only on creation).
// Agent.Token is never serialized to JSON, so every other response omits it.
type AgentWithToken struct {
	Agent
	Token string `json:"token"`
} //	@name	AgentWithToken

const (
	IDNotSet = -1
)

// AgentListOptions defines filtering options for listing agents.
type AgentListOptions struct {
	ListOptions
	Search   string // Search in name, platform, backend, custom labels
	Platform string // Filter by platform
	Backend  string // Filter by backend
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

// IsGlobal reports whether the agent can process tasks from any organization.
// IsReservedAgentLabel reports whether a label key uses the crowci/ prefix that
// the server reserves for its own task metadata and agent scoping. Agents must
// not advertise such labels as custom labels: a custom crowci/org-id would let an
// org-scoped agent claim another org's tasks.
func IsReservedAgentLabel(key string) bool {
	return strings.HasPrefix(key, pipeline.InternalLabelPrefix+"/")
}

// StripReservedAgentLabels returns a copy of labels without reserved keys, and
// the sorted keys that were removed.
func StripReservedAgentLabels(labels map[string]string) (map[string]string, []string) {
	if labels == nil {
		return nil, nil
	}
	kept := make(map[string]string, len(labels))
	var dropped []string
	for k, v := range labels {
		if IsReservedAgentLabel(k) {
			dropped = append(dropped, k)
			continue
		}
		kept[k] = v
	}
	slices.Sort(dropped)
	return kept, dropped
}

func (a *Agent) IsGlobal() bool {
	return len(a.OrgIDs) == 0
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

	// enforce filters for organization-scoped agents; global agents match any org
	if a.IsGlobal() {
		filters[pipeline.LabelOrgID] = "*"
	} else {
		ids := make([]string, len(a.OrgIDs))
		for i, id := range a.OrgIDs {
			ids[i] = strconv.FormatInt(id, 10)
		}
		filters[pipeline.LabelOrgID] = strings.Join(ids, "|")
	}

	return filters, nil
}

func (a *Agent) CanAccessRepo(repo *Repo) bool {
	// global agent
	if a.IsGlobal() {
		return true
	}

	// agent scoped to the repo's organization
	return slices.Contains(a.OrgIDs, repo.OrgID)
}

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

// Metrics represents overall system metrics.
type Metrics struct {
	TotalRepos       int64            `json:"total_repos"`
	TotalOrgs        int64            `json:"total_orgs"`
	TotalPipelines   int64            `json:"total_pipelines"`
	AvgBuildTime     float64          `json:"avg_build_time"` // in seconds
	AvgStepsPerFlow  float64          `json:"avg_steps_per_flow"`
	PipelinesByRepo  map[string]int64 `json:"pipelines_by_repo"`  // repo name -> count
	PipelinesByOrg   map[string]int64 `json:"pipelines_by_org"`   // org name -> count
	WorkflowsByAgent map[string]int64 `json:"workflows_by_agent"` // agent name -> count
	RepoDetails      []RepoMetric     `json:"repo_details"`
	OrgDetails       []OrgMetric      `json:"org_details"`
	AgentDetails     []AgentMetric    `json:"agent_details"`
	IsAdminScope     bool             `json:"is_admin_scope"` // true if showing all instance metrics
}

// AdminMetrics is deprecated, use Metrics instead.
// Keeping for backward compatibility during migration.
type AdminMetrics = Metrics

// RepoMetric represents metrics for a specific repository.
type RepoMetric struct {
	RepoID         int64   `json:"repo_id"`
	RepoName       string  `json:"repo_name"`
	RepoFullName   string  `json:"repo_full_name"`
	PipelineCount  int64   `json:"pipeline_count"`
	AvgBuildTime   float64 `json:"avg_build_time"`   // in seconds
	TotalBuildTime float64 `json:"total_build_time"` // in seconds
}

// OrgMetric represents metrics for a specific organization.
type OrgMetric struct {
	OrgID          int64   `json:"org_id"`
	OrgName        string  `json:"org_name"`
	PipelineCount  int64   `json:"pipeline_count"`
	RepoCount      int64   `json:"repo_count"`
	AvgBuildTime   float64 `json:"avg_build_time"`   // in seconds
	TotalBuildTime float64 `json:"total_build_time"` // in seconds
}

// AgentMetric represents metrics for a specific agent.
type AgentMetric struct {
	AgentID       int64   `json:"agent_id"`
	AgentName     string  `json:"agent_name"`
	WorkflowCount int64   `json:"workflow_count"`
	AvgBuildTime  float64 `json:"avg_build_time"` // in seconds
	Platform      string  `json:"platform"`
	Backend       string  `json:"backend"`
}

// MetricsTimeFilter represents time-based filtering for metrics queries.
type MetricsTimeFilter struct {
	After  int64 `json:"after,omitempty"`  // Unix timestamp
	Before int64 `json:"before,omitempty"` // Unix timestamp
}

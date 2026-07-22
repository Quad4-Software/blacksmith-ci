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
	AgentID           int64   `json:"agent_id"`
	AgentPersistentID string  `json:"agent_persistent_id,omitempty"`
	AgentName         string  `json:"agent_name"`
	WorkflowCount     int64   `json:"workflow_count"`
	AvgBuildTime      float64 `json:"avg_build_time"` // in seconds
	Platform          string  `json:"platform"`
	Backend           string  `json:"backend"`
}

// AgentMetricData holds aggregated workflow data for an agent persistent ID.
// Used by store methods to return agent metrics grouped by persistent ID.
type AgentMetricData struct {
	AgentPersistentID string
	AgentName         string
	Platform          string
	Count             int64
}

// OrgMetricData holds aggregated pipeline data for an org, with the display
// name carried straight from the denormalized pipeline rows so it survives org
// deletion and covers personal-account orgs.
type OrgMetricData struct {
	OrgID         int64
	OrgName       string
	PipelineCount int64
	RepoCount     int64
}

// RepoMetricData holds aggregated pipeline data for a repo, with the display
// name carried straight from the denormalized pipeline rows so it survives repo
// deletion.
type RepoMetricData struct {
	RepoID        int64
	RepoFullName  string
	PipelineCount int64
}

// MetricsTimeFilter represents time-based filtering for metrics queries.
type MetricsTimeFilter struct {
	After  int64 `json:"after,omitempty"`  // Unix timestamp
	Before int64 `json:"before,omitempty"` // Unix timestamp
}

// Chart-builder dimensions, metrics, time buckets and units. The API validates
// requests against these and the store branches on them.
const (
	MetricDimensionOrg   = "org"
	MetricDimensionRepo  = "repo"
	MetricDimensionAgent = "agent"
	MetricDimensionTime  = "time"

	MetricPipelineCount  = "pipeline_count"
	MetricWorkflowCount  = "workflow_count"
	MetricAvgBuildTime   = "avg_build_time"
	MetricTotalBuildTime = "total_build_time"
	MetricAvgSteps       = "avg_steps"

	MetricBucketDay   = "day"
	MetricBucketWeek  = "week"
	MetricBucketMonth = "month"

	MetricUnitCount   = "count"
	MetricUnitSeconds = "seconds"
)

// MetricSeries is the generic result of a chart-builder query.
type MetricSeries struct {
	Dimension string        `json:"dimension"`        // org|repo|agent|time
	Metric    string        `json:"metric"`           // pipeline_count|workflow_count|avg_build_time|total_build_time|avg_steps
	Bucket    string        `json:"bucket,omitempty"` // day|week|month (time dimension only)
	Unit      string        `json:"unit"`             // count|seconds
	Points    []MetricPoint `json:"points"`
} //	@name	MetricSeries

// MetricPoint is one bar/point in a MetricSeries.
type MetricPoint struct {
	Key   string  `json:"key"`          // stable sort/identity key (id or bucket label)
	ID    int64   `json:"id,omitempty"` // numeric id for org/repo dimensions
	Label string  `json:"label"`        // display name
	Value float64 `json:"value"`
} //	@name	MetricPoint

// MetricQueryOptions is the validated, access-scoped input to GetMetricSeries.
type MetricQueryOptions struct {
	Dimension string
	Metric    string
	Bucket    string // day|week|month, time dimension only
	After     int64
	Before    int64
	RepoIDs   []int64 // already access-scoped by the API layer
}

// Unit returns the value unit produced by the given metric.
func MetricUnit(metric string) string {
	switch metric {
	case MetricAvgBuildTime, MetricTotalBuildTime:
		return MetricUnitSeconds
	default:
		return MetricUnitCount
	}
}

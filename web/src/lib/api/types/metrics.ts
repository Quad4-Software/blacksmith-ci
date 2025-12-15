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

export interface Metrics {
  total_repos: number;
  total_orgs: number;
  total_pipelines: number;
  avg_build_time: number; // in seconds
  avg_steps_per_flow: number;
  pipelines_by_repo: Record<string, number>; // repo name -> count
  pipelines_by_org: Record<string, number>; // org name -> count
  workflows_by_agent: Record<string, number>; // agent name -> count
  repo_details: RepoMetric[];
  org_details: OrgMetric[];
  agent_details: AgentMetric[];
  is_admin_scope: boolean; // true if showing all instance metrics
}

// Deprecated: use Metrics instead
export type AdminMetrics = Metrics;

export interface RepoMetric {
  repo_id: number;
  repo_name: string;
  repo_full_name: string;
  pipeline_count: number;
  avg_build_time: number; // in seconds
  total_build_time: number; // in seconds
}

export interface OrgMetric {
  org_id: number;
  org_name: string;
  pipeline_count: number;
  repo_count: number;
  avg_build_time: number; // in seconds
  total_build_time: number; // in seconds
}

export interface AgentMetric {
  agent_id: number;
  agent_name: string;
  workflow_count: number;
  avg_build_time: number; // in seconds
  platform: string;
  backend: string;
}

export interface MetricsTimeFilter {
  after?: number; // Unix timestamp
  before?: number; // Unix timestamp
}

export interface MetricsFilter extends MetricsTimeFilter {
  all?: boolean; // Show all instance metrics (admin only)
  repos?: string; // Comma-separated list of repo IDs
}

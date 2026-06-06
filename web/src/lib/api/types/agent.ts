import type { PipelineStatus } from './pipeline';

// AgentWorkflow is a historic workflow processed by an agent, enriched with the
// owning pipeline's repo and number for deep-linking.
export interface AgentWorkflow {
  id: number;
  pipeline_id: number;
  pid: number;
  name: string;
  pipeline_name: string;
  state: PipelineStatus;
  started?: number;
  finished?: number;
  agent_id?: number;
  agent_name?: string;
  error?: string;
  repo_id: number;
  pipeline_number: number;
}

export interface Agent {
  id: number;
  name: string;
  owner_id: number;
  org_id: number;
  token: string;
  created: number;
  updated: number;
  last_contact: number;
  workflow_polling_healthy: boolean;
  platform: string;
  backend: string;
  capacity: number;
  version: string;
  build_date: string;
  no_schedule: boolean;
  priority: number;
  custom_labels: Record<string, string>;
  limit_mem: number;
  limit_cpu_quota: number;
  running_workflows: number;
}

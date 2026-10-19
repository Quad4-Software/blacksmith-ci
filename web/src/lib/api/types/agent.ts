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
  org_ids?: number[];
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
  /** logical cores of the machine running the workloads, 0 when unknown */
  host_cpus: number;
  /** total memory in bytes of the machine running the workloads, 0 when unknown */
  host_mem: number;
  /** agent protocol version reported on registration, 0 when unknown */
  protocol_version: number;
  /** "restart" when the agent drains and exits on a restart request, empty when it ignores them */
  remote_update: string;
  /** an admin asked the agent to restart once its running workflows finish */
  restart_requested: boolean;
  /** a restart waits for a free slot under the server's parallel restart limit */
  restart_queued: boolean;
  running_workflows: number;
}

// AgentWithToken is only returned when an agent is created; no other endpoint exposes the token.
export interface AgentWithToken extends Agent {
  token: string;
}

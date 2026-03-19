export interface Autoscaler {
  id: number;
  created: number;
  updated: number;
  name: string;
  owner_id: number;
  org_id: number;
  last_contact: number;
  min_agents: number;
  max_agents: number;
  workflows_per_agent: number;
  provider: string;
  instance_type: string;
  region: string;
  agent_labels: Record<string, string>;
  limit_cpu_quota: string;
  limit_mem: string;
  enabled: boolean;
  version: string;
  active_agents: number;
  pending_agents: number;
  pending_since: number;
}

export interface AutoscalerWithToken {
  id: number;
  created: number;
  updated: number;
  name: string;
  owner_id: number;
  org_id: number;
  last_contact: number;
  min_agents: number;
  max_agents: number;
  workflows_per_agent: number;
  provider: string;
  instance_type: string;
  region: string;
  agent_labels: Record<string, string>;
  limit_cpu_quota: string;
  limit_mem: string;
  enabled: boolean;
  version: string;
  active_agents: number;
  pending_agents: number;
  pending_since: number;
  token: string;
}

export interface AutoscalerListOptions {
  page?: number;
  perPage?: number;
  search?: string;
  enabled?: boolean;
  sort?: 'id' | 'name' | 'last_contact' | 'status';
  order?: 'asc' | 'desc';
}

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
  no_schedule: boolean;
  priority: number;
  custom_labels: Record<string, string>;
  limit_mem: number;
  limit_cpu_quota: number;
  running_workflows: number;
}

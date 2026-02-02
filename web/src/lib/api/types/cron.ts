export interface Cron {
  id: number;
  repo_id: number;
  name: string;
  branch: string;
  schedule: string;
  next_exec: number;
}

export interface CronWithRepo extends Cron {
  repo_name: string;
  repo_owner: string;
  repo_default_branch: string;
  forge_id?: number;
  forge_name?: string;
  forge_icon?: string;
  forge_type?: string;
}

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
}

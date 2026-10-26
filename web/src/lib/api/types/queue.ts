export interface Task {
  id: number;
  pid: number;
  name: string;
  labels: Record<string, string>;
  dependencies: string[];
  dep_status: Record<string, string>;
  run_on: string[];
  agent_id: number;
  agent_name: string;
  pipeline_id: number;
  pipeline_number: number;
  repo_id: number;
  pipeline_created: number; // Unix timestamp when the pipeline was created
  pipeline_started: number; // Unix timestamp when the pipeline started running (0 if not started)
  stuck_reason?: QueueStuckReason;
}

// blocked: waits on dependencies that left the queue without succeeding
// waited_too_long: waited longer than the server's stuck threshold
export type QueueStuckReason = 'blocked' | 'waited_too_long';

export interface QueueStats {
  worker_count: number;
  pending_count: number;
  waiting_on_deps_count: number;
  running_count: number;
  completed_count: number;
}

export interface QueueInfo {
  pending: Task[];
  waiting_on_deps: Task[];
  running: Task[];
  stats: QueueStats;
  paused: boolean;
  stuck_threshold: number; // seconds, 0 when the age check is disabled
}

export interface QueueCancelTasksResponse {
  canceled: number;
  failed: Record<string, string> | null;
}

export interface StuckWorkflow {
  name: string;
  reason: QueueStuckReason;
}

// A pipeline the current user triggered with workflows stuck in the queue.
export interface StuckPipeline {
  repo_id: number;
  repo_full_name: string;
  pipeline_number: number;
  created: number;
  workflows: StuckWorkflow[];
}

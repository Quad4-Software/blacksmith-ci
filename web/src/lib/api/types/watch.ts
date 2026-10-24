import type { NotificationEvent } from './notification';
import type { Pipeline } from './pipeline';

export interface WatchSettings {
  in_app: boolean;
  email: boolean;
  email_events: NotificationEvent[];
}

export interface CreateWatch extends WatchSettings {
  event: string;
  target: string;
}

export interface PipelineWatch extends WatchSettings {
  id: number;
  repo_id: number;
  event: string;
  target: string;
  last_read_pipeline: number;
  created_at: number;
  updated_at: number;
}

export interface WatchFailingWorkflow {
  // Empty for a pipeline-level (configuration) failure.
  name: string;
  pipeline_number: number;
}

export interface WatchOverviewItem {
  watch: PipelineWatch;
  repo_full_name: string;
  failing: boolean;
  unread: boolean;
  failing_workflows: WatchFailingWorkflow[];
  latest_pipeline?: Pipeline;
}

export interface WatchSuggestion {
  event: string;
  target: string;
}

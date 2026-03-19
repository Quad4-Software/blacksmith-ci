import type { WebhookEvents } from './webhook';

export interface PipelineError<D = unknown> {
  type: PipelineErrorType;
  message: string;
  data?: D;
  is_warning: boolean;
}

export type PipelineErrorType = 'linter' | 'deprecation' | 'compiler' | 'generic' | 'Best practice';

export interface LinterErrorData {
  file?: string;
  field: string;
  line?: number;
  column?: number;
}

export interface DeprecationErrorData {
  file: string;
  field: string;
  docs: string;
}

export interface BadHabitErrorData {
  file?: string;
  field: string;
  docs: string;
}

export interface CompilerErrorData {
  file?: string;
  step?: string;
}

// A pipeline for a repository.
export interface Pipeline {
  id: number;

  // The pipeline number.
  // This number is specified within the context of the repository the pipeline belongs to and is unique within that.
  number: number;

  parent: number;

  event: WebhookEvents;

  //  The current status of the pipeline.
  status: PipelineStatus;

  errors?: PipelineError[];

  // When the pipeline request was received.
  created: number;

  // When the pipeline was updated last time in database.
  updated: number;

  // When the pipeline began execution.
  started: number;

  // When the pipeline was finished.
  finished: number;

  // Where the deployment should go.
  deploy_to: string;

  // The commit for the pipeline.
  commit: string;

  // The branch the commit was pushed to.
  branch: string;

  // The commit message.
  message: string;

  // When the commit was created.
  timestamp: number;

  // The alias for the commit.
  ref: string;

  // The mapping from the local repository to a branch in the forge.
  refspec: string;

  // The clone URL of the forge repository.
  clone_url: string;

  title: string;

  sender: string;

  // The login for the author of the commit.
  author: string;

  // The avatar for the author of the commit.
  author_avatar: string;

  //  email for the author of the commit.
  author_email: string;

  // This url will point to the repository state associated with the pipeline's commit.
  forge_url: string;

  reviewed_by: string;

  reviewed: number;

  // The steps associated with this pipeline.
  // A pipeline will have multiple steps if a matrix pipeline was used or if a rebuild was requested.
  workflows?: PipelineWorkflow[];

  changed_files?: string[];

  // Whether the logs were deleted or not
  logs_deleted: number;

  // Whether this PR is always-approved
  is_always_approved?: boolean;
}

export type PipelineStatus =
  | 'blocked'
  | 'declined'
  | 'error'
  | 'failure'
  | 'killed'
  | 'pending'
  | 'running'
  | 'skipped'
  | 'started'
  | 'success';

export interface PipelineWorkflow {
  id: number;
  pipeline_id: number;
  pid: number;
  name: string;
  pipeline_name: string;
  state: PipelineStatus;
  environ?: Record<string, string>;
  started?: number;
  finished?: number;
  agent_id?: number;
  error?: string;
  depends_on?: string[];
  original_workflow_id: number;
  attempt: number;
  autoscaler_provisioning?: boolean;
  autoscaler_provisioning_stale?: boolean;
  children: PipelineStep[];
}

export interface PipelineStep {
  id: number;
  uuid: string;
  pipeline_id: number;
  pid: number;
  ppid: number;
  name: string;
  state: PipelineStatus;
  exit_code: number;
  started?: number;
  finished?: number;
  error?: string;
  type?: StepType;
  depends_on?: string[];
}

export interface PipelineLog {
  id: number;
  step_id: number;
  time: number;
  line: number;
  data: string; // base64 encoded
  type: number;
}

export type PipelineFeed = Pipeline & {
  repo_id: number;
};

/* eslint-disable no-unused-vars */
export enum StepType {
  Clone = 'clone',
  Service = 'service',
  Plugin = 'plugin',
  Commands = 'commands',
  Cache = 'cache',
}
/* eslint-enable */

// Workflow information returned from GET /repos/:id/configs
export interface ManualWorkflow {
  name: string;
  pipeline_name: string;
  depends_on?: string[];
  has_manual: boolean;
}

// Response from GET /repos/:id/configs?filter=manual
export interface ManualWorkflowsResponse {
  workflows: ManualWorkflow[];
  warnings?: string[];
}

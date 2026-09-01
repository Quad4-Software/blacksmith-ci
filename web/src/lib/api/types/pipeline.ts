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
  // Names the repository "Privileges & Security" option that resolves this
  // error ('security', 'network' or 'volumes'); absent for unrelated errors.
  trusted?: string;
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
  secret_name?: string;
  event?: string;
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

type ActivePipelineStatus = 'blocked' | 'pending' | 'running' | 'started';
type CompletedPipelineStatus = 'declined' | 'error' | 'failure' | 'killed' | 'skipped' | 'success';

export type PipelineStatus = ActivePipelineStatus | CompletedPipelineStatus;

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
  agent_name?: string;
  error?: string;
  depends_on?: string[];
  original_workflow_id: number;
  attempt: number;
  autoscaler_provisioning?: boolean;
  autoscaler_provisioning_stale?: boolean;
  children: PipelineStep[];
  // Workflow approval gate fields (only set when the workflow declares one).
  requires_approval?: boolean;
  approval_users?: string[];
  approval_teams?: string[];
  approval_state?: ApprovalState;
  approval_deadline?: number;
}

// ApprovalState describes the lifecycle of a workflow-level approval gate.
// '' (empty) means the workflow has no gate configured.
export type ApprovalState = '' | 'pending' | 'approved' | 'declined' | 'expired';

// WorkflowApprovalEvent is a single append-only entry in the audit trail for a
// workflow approval gate.
export interface WorkflowApprovalEvent {
  id: number;
  workflow_id: number;
  user_id: number;
  action: 'approved' | 'declined' | 'expired';
  reason?: string;
  created_at: number;
}

// WorkflowApprovalState is the response payload of GET .../workflows/{pid}/approval.
export interface WorkflowApprovalState {
  workflow: PipelineWorkflow;
  events: WorkflowApprovalEvent[];
}

// PendingApprovalItem bundles a workflow gate with the minimal pipeline+repo
// context the UI needs to deep-link back to the pipeline detail page without
// issuing per-item lookups.
export interface PendingApprovalItem {
  workflow: PipelineWorkflow;
  pipeline_number: number;
  repo_id: number;
  repo_full_name: string;
}

// UserPendingApprovalsResponse is the response payload of
// GET /api/v1/user/approvals/pending.
export interface UserPendingApprovalsResponse {
  approvals: PendingApprovalItem[];
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
  original_step_id: number;
  attempt: number;
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

// A user-settable variable surfaced by GET /repos/:id/configs
export interface ManualVariable {
  name: string;
  description?: string;
  options?: string[];
  default?: string;
  required?: boolean;
}

// Workflow information returned from GET /repos/:id/configs
export interface ManualWorkflow {
  name: string;
  pipeline_name: string;
  depends_on?: string[];
  has_manual: boolean;
  variables?: ManualVariable[];
}

// Response from GET /repos/:id/configs?filter=manual
export interface ManualWorkflowsResponse {
  workflows: ManualWorkflow[];
  warnings?: string[];
}

export type ManualWorkflowsProgressStage = 'fetching' | 'preprocessing' | 'parsing' | 'complete' | 'error';

export interface ManualWorkflowsProgress {
  stage: ManualWorkflowsProgressStage;
  current?: number;
  total?: number;
  result?: ManualWorkflowsResponse;
  error?: string;
}

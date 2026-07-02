// Runtime metadata returned by the pipeline metadata endpoint.
// Mirrors the Go `metadata.Metadata` struct served at
// `/api/v1/repos/{repo_id}/pipelines/{number}/metadata`.
// All fields use `omitempty` on the backend, so they are optional here.

export interface PipelineMetadataTrustedConfiguration {
  network?: boolean;
  volumes?: boolean;
  security?: boolean;
}

export interface PipelineMetadataRepo {
  id?: number;
  name?: string;
  owner?: string;
  remote_id?: string;
  forge_url?: string;
  clone_url?: string;
  clone_url_ssh?: string;
  private?: boolean;
  default_branch?: string;
  trusted?: PipelineMetadataTrustedConfiguration;
}

export interface PipelineMetadataAuthor {
  name?: string;
  email?: string;
  avatar?: string;
}

export interface PipelineMetadataCommit {
  sha?: string;
  ref?: string;
  refspec?: string;
  branch?: string;
  message?: string;
  author?: PipelineMetadataAuthor;
  changed_files?: string[];
  labels?: string[];
  is_prerelease?: boolean;
}

export interface PipelineMetadataPipeline {
  number?: number;
  created?: number;
  started?: number;
  finished?: number;
  status?: string;
  event?: string;
  forge_url?: string;
  target?: string;
  task?: string;
  commit?: PipelineMetadataCommit;
  parent?: number;
  cron?: string;
  title?: string;
  author?: string;
  avatar?: string;
}

export interface PipelineMetadataWorkflow {
  name?: string;
  number?: number;
  matrix?: Record<string, string>;
}

export interface PipelineMetadataStep {
  name?: string;
  number?: number;
}

export interface PipelineMetadataSystem {
  name?: string;
  host?: string;
  url?: string;
  arch?: string;
  version?: string;
}

export interface PipelineMetadataForge {
  type?: string;
  url?: string;
}

export interface PipelineMetadata {
  id?: string;
  repo?: PipelineMetadataRepo;
  curr?: PipelineMetadataPipeline;
  prev?: PipelineMetadataPipeline;
  workflow?: PipelineMetadataWorkflow;
  step?: PipelineMetadataStep;
  sys?: PipelineMetadataSystem;
  forge?: PipelineMetadataForge;
}

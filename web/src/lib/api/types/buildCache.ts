export interface BuildCache {
  id: number;
  repo_id: number;
  // Git ref the cache belongs to, e.g. refs/heads/main or refs/pull/12/head.
  scope: string;
  key: string;
  // Archive size in bytes.
  size: number;
  // False while an upload is still in progress.
  committed: boolean;
  created: number;
  last_used: number;
}

export interface RepoCaches {
  // False when caching is disabled on the server.
  enabled: boolean;
  // Total size of all cache entries in bytes.
  size: number;
  entries: BuildCache[];
}

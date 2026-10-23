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
  // 0 for the server's storage, otherwise the bucket the org connected.
  bucket_id?: number;
  created: number;
  last_used: number;
}

export interface RepoCaches {
  // False when caching is disabled on the server.
  enabled: boolean;
  // Total size of all cache entries in bytes.
  size: number;
  entries: BuildCache[];
  // Cache traffic of the last 30 days.
  usage?: BuildCacheUsageCounts;
}

// Cache traffic counters, summed over a period.
export interface BuildCacheUsageCounts {
  // Restores of the requested key.
  hits: number;
  // Restores that fell back to a restore key.
  partial_hits: number;
  misses: number;
  // Restores refused by the monthly egress limit.
  denied: number;
  saves: number;
  // Bytes handed out to restore and saved, respectively.
  egress: number;
  ingress: number;
}

export interface BuildCacheUsageDay extends BuildCacheUsageCounts {
  // Start of the UTC day as a unix timestamp.
  day: number;
}

// Limit values: 0 applies the server default, -1 lifts the limit, positive values are explicit.
export interface BuildCacheSettings {
  org_id: number;
  max_size: number;
  max_egress: number;
  // Seconds an entry is kept without being restored.
  ttl: number;
  updated: number;
}

// Limits in effect; 0 means unlimited, for ttl that entries are kept until evicted by size.
export interface BuildCacheLimits {
  max_size: number;
  max_egress: number;
  ttl: number;
}

export interface BuildCacheBucket {
  id: number;
  org_id: number;
  endpoint: string;
  bucket: string;
  region: string;
  prefix: string;
  access_key: string;
  ssl: boolean;
  path_style: boolean;
  // Limits the owner set for the bucket; 0 is unlimited.
  max_size: number;
  max_egress: number;
  // 0 applies the server default, -1 keeps entries until removed by hand.
  ttl: number;
  created: number;
  updated: number;
}

export interface BuildCacheBucketInput {
  endpoint: string;
  bucket: string;
  region?: string;
  prefix?: string;
  access_key?: string;
  // Empty keeps the secret key of the connected bucket.
  secret_key?: string;
  ssl: boolean;
  path_style: boolean;
  ttl: number;
  // 0 or -1 is unlimited.
  max_size: number;
  max_egress: number;
}

export interface BuildCacheOrgSummary {
  org_id: number;
  // Empty when the org was deleted.
  name: string;
  is_user: boolean;
  // Size, entries and repos in the server's storage.
  size: number;
  entries: number;
  repos: number;
  // Size and entries in the bucket the org connected.
  bucket_size: number;
  bucket_entries: number;
  // The last 30 days.
  usage: BuildCacheUsageCounts;
  // Egress this calendar month from the server's storage and from the own bucket.
  egress_month: number;
  bucket_egress_month: number;
  settings: BuildCacheSettings;
  limits: BuildCacheLimits;
  bucket?: BuildCacheBucket;
}

export interface BuildCacheRepoSummary {
  repo_id: number;
  full_name: string;
  size: number;
  entries: number;
  usage: BuildCacheUsageCounts;
}

export interface BuildCacheOrgDetail extends BuildCacheOrgSummary {
  repo_list: BuildCacheRepoSummary[];
  daily: BuildCacheUsageDay[] | null;
}

export interface BuildCacheOverview {
  defaults: BuildCacheLimits;
  max_repo_size: number;
  max_total_size: number;
  allow_buckets: boolean;
  size: number;
  entries: number;
  bucket_size: number;
  bucket_entries: number;
  usage: BuildCacheUsageCounts;
  egress_month: number;
  orgs: BuildCacheOrgSummary[] | null;
}

export interface AdminCaches {
  enabled: boolean;
  overview?: BuildCacheOverview;
}

export interface OrgCaches {
  enabled: boolean;
  allow_buckets: boolean;
  // Server defaults, applied where the org has no limits or retention of its own.
  defaults: BuildCacheLimits;
  detail?: BuildCacheOrgDetail;
}

export interface BuildCacheCleanupResult {
  removed: number;
  freed: number;
}

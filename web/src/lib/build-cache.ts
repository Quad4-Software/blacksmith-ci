export type CacheScopeKind = 'branch' | 'tag' | 'pull-request' | 'other';

export interface CacheScope {
  kind: CacheScopeKind;
  name: string;
}

// Pull request refs differ per forge: GitHub/Gitea/Forgejo use refs/pull/<n>/head,
// GitLab refs/merge-requests/<n>/head and Bitbucket refs/pull-requests/<n>/from.
const PULL_REQUEST_REF_REGEX = /^refs\/(?:pull|merge-requests|pull-requests)\/(\d+)\/\w+$/;

const BRANCH_PREFIX = 'refs/heads/';
const TAG_PREFIX = 'refs/tags/';

// parseCacheScope turns the git ref a cache is scoped to into a display name.
export function parseCacheScope(scope: string): CacheScope {
  if (scope.startsWith(BRANCH_PREFIX)) {
    return { kind: 'branch', name: scope.slice(BRANCH_PREFIX.length) };
  }
  if (scope.startsWith(TAG_PREFIX)) {
    return { kind: 'tag', name: scope.slice(TAG_PREFIX.length) };
  }
  const pullRequest = PULL_REQUEST_REF_REGEX.exec(scope);
  if (pullRequest) {
    return { kind: 'pull-request', name: `#${pullRequest[1]}` };
  }
  return { kind: 'other', name: scope };
}

const SIZE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB'];

// formatCacheSize renders a byte count with a binary unit, e.g. 1536 -> "1.5 KB".
export function formatCacheSize(bytes: number): string {
  if (bytes <= 0) return '0 B';
  const k = 1024;
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), SIZE_UNITS.length - 1);
  return `${Math.round((bytes / k ** i) * 100) / 100} ${SIZE_UNITS[i]}`;
}

// Restores per counter, as reported by the server.
export interface CacheRestoreCounts {
  hits: number;
  partial_hits: number;
  misses: number;
  denied: number;
}

// cacheRestores is the number of restores, whether they hit or not.
export function cacheRestores(counts: CacheRestoreCounts): number {
  return counts.hits + counts.partial_hits + counts.misses + counts.denied;
}

// cacheHitRate is the share of restores that restored a cache, exact or by restore key, or undefined without restores.
export function cacheHitRate(counts: CacheRestoreCounts): number | undefined {
  const restores = cacheRestores(counts);
  if (restores === 0) return undefined;
  return (counts.hits + counts.partial_hits) / restores;
}

// formatHitRate renders a hit rate as a whole percentage, or a dash without restores.
export function formatHitRate(rate: number | undefined): string {
  if (rate === undefined) return '—';
  return `${Math.round(rate * 100)}%`;
}

const SECONDS_PER_HOUR = 3600;
const SECONDS_PER_DAY = 24 * SECONDS_PER_HOUR;

// formatCacheTTL renders a retention period in seconds as days or hours.
export function formatCacheTTL(seconds: number): string {
  if (seconds % SECONDS_PER_DAY === 0) return `${seconds / SECONDS_PER_DAY}d`;
  if (seconds % SECONDS_PER_HOUR === 0) return `${seconds / SECONDS_PER_HOUR}h`;
  return `${Math.round((seconds / SECONDS_PER_HOUR) * 10) / 10}h`;
}

// LimitMode is how an org's limit relates to the server default.
export type LimitMode = 'default' | 'none' | 'custom';

// limitMode classifies a stored limit: 0 is the server default, -1 lifts the limit.
export function limitMode(value: number): LimitMode {
  if (value === 0) return 'default';
  if (value < 0) return 'none';
  return 'custom';
}

// limitUsage is how much of a limit is used, above 1 when it is exceeded, or undefined when unlimited.
export function limitUsage(used: number, limit: number): number | undefined {
  if (limit <= 0) return undefined;
  return used / limit;
}

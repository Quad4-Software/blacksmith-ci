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

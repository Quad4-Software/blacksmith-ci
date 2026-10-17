import { describe, expect, it } from 'vitest';

import { formatCacheSize, parseCacheScope } from './build-cache';

describe('parseCacheScope', () => {
  it.each([
    ['refs/heads/main', 'branch', 'main'],
    ['refs/heads/feature/caching', 'branch', 'feature/caching'],
    ['refs/tags/v1.2.3', 'tag', 'v1.2.3'],
    ['refs/pull/12/head', 'pull-request', '#12'],
    ['refs/merge-requests/7/head', 'pull-request', '#7'],
    ['refs/pull-requests/3/from', 'pull-request', '#3'],
    ['refs/custom/thing', 'other', 'refs/custom/thing'],
  ])('parses %s', (scope, kind, name) => {
    expect(parseCacheScope(scope)).toEqual({ kind, name });
  });
});

describe('formatCacheSize', () => {
  it.each([
    [0, '0 B'],
    [512, '512 B'],
    [1536, '1.5 KB'],
    [5 * 1024 * 1024, '5 MB'],
    [3.25 * 1024 ** 3, '3.25 GB'],
    [2 * 1024 ** 5, '2048 TB'],
  ])('formats %d bytes', (bytes, expected) => {
    expect(formatCacheSize(bytes)).toBe(expected);
  });
});

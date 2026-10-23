import { describe, expect, it } from 'vitest';

import {
  cacheHitRate,
  cacheRestores,
  formatCacheSize,
  formatCacheTTL,
  formatHitRate,
  limitMode,
  limitUsage,
  parseCacheScope,
} from './build-cache';

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

describe('cache hit rate', () => {
  const counts = { hits: 6, partial_hits: 2, misses: 1, denied: 1 };

  it('counts every restore', () => {
    expect(cacheRestores(counts)).toBe(10);
  });

  it('counts restore key hits as hits', () => {
    expect(cacheHitRate(counts)).toBe(0.8);
    expect(formatHitRate(cacheHitRate(counts))).toBe('80%');
  });

  it('has no rate without restores', () => {
    expect(cacheHitRate({ hits: 0, partial_hits: 0, misses: 0, denied: 0 })).toBeUndefined();
    expect(formatHitRate(undefined)).toBe('—');
  });
});

describe('cache limits', () => {
  it('classifies stored limits', () => {
    expect(limitMode(0)).toBe('default');
    expect(limitMode(-1)).toBe('none');
    expect(limitMode(1024)).toBe('custom');
  });

  it('reports the used share, beyond the limit when exceeded', () => {
    expect(limitUsage(25, 100)).toBe(0.25);
    expect(limitUsage(200, 100)).toBe(2);
    expect(limitUsage(200, 0)).toBeUndefined();
  });

  it('formats retention periods', () => {
    expect(formatCacheTTL(7 * 86400)).toBe('7d');
    expect(formatCacheTTL(3 * 3600)).toBe('3h');
    expect(formatCacheTTL(5400)).toBe('1.5h');
  });
});

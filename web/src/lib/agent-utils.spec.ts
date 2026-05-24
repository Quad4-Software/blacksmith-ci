import { describe, expect, it } from 'vitest';

import { formatBytes, formatCPUQuota } from './agent-utils';

// Identity translator: returns the key unchanged so we can assert on it.
const t = (key: string): string => key;

describe('formatBytes', () => {
  it('returns the unlimited translation when bytes is 0', () => {
    expect(formatBytes(0, t)).toBe('admin.settings.agents.unlimited');
  });

  it('formats bytes below 1 KB as B', () => {
    expect(formatBytes(512, t)).toBe('512 B');
  });

  it('formats kilobytes', () => {
    expect(formatBytes(2048, t)).toBe('2 KB');
  });

  it('formats gigabytes with 2 decimal places of precision', () => {
    // 3 GiB = 3 * 1024^3 = 3221225472
    expect(formatBytes(3 * 1024 ** 3, t)).toBe('3 GB');
  });

  it('rounds non-integer values to 2 decimals', () => {
    // 1.5 MB = 1.5 * 1024^2 = 1572864
    expect(formatBytes(1572864, t)).toBe('1.5 MB');
  });
});

describe('formatCPUQuota', () => {
  it('returns the unlimited translation when quota is 0', () => {
    expect(formatCPUQuota(0, t)).toBe('admin.settings.agents.unlimited');
  });

  it('formats one full core as "1"', () => {
    expect(formatCPUQuota(100000, t)).toBe('1');
  });

  it('formats two full cores as "2"', () => {
    expect(formatCPUQuota(200000, t)).toBe('2');
  });

  it('formats fractional cores below 1 in microseconds', () => {
    expect(formatCPUQuota(50000, t)).toBe('50000µs');
  });
});

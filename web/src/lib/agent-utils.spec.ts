import { describe, expect, it } from 'vitest';

import type { Agent } from '~/lib/api/types';

import { formatAgentCPU, formatAgentMemory, formatBytes, formatCPUQuota, hasAgentResources } from './agent-utils';

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

function agentWith(fields: Partial<Agent>): Agent {
  return {
    backend: 'docker',
    limit_mem: 0,
    limit_cpu_quota: 0,
    host_cpus: 0,
    host_mem: 0,
    ...fields,
  } as Agent;
}

describe('formatAgentMemory', () => {
  it('pairs the per-workflow limit with the host total', () => {
    expect(formatAgentMemory(agentWith({ limit_mem: 4 * 1024 ** 3, host_mem: 64 * 1024 ** 3 }), t)).toBe(
      '4 GB / 64 GB',
    );
  });

  it('shows only the host total when no limit is set', () => {
    expect(formatAgentMemory(agentWith({ host_mem: 64 * 1024 ** 3 }), t)).toBe('64 GB');
  });

  it('falls back to the limit when the host total is unknown', () => {
    expect(formatAgentMemory(agentWith({ limit_mem: 4 * 1024 ** 3 }), t)).toBe('4 GB');
  });

  it('reports unlimited when neither value is known', () => {
    expect(formatAgentMemory(agentWith({}), t)).toBe('admin.settings.agents.unlimited');
  });
});

describe('formatAgentCPU', () => {
  it('pairs the per-workflow quota with the host core count', () => {
    expect(formatAgentCPU(agentWith({ limit_cpu_quota: 200000, host_cpus: 16 }), t)).toBe(
      '2 / 16 admin.settings.agents.cores',
    );
  });

  it('shows only the host core count when no quota is set', () => {
    expect(formatAgentCPU(agentWith({ host_cpus: 16 }), t)).toBe('16 admin.settings.agents.cores');
  });

  it('falls back to the quota when the host core count is unknown', () => {
    expect(formatAgentCPU(agentWith({ limit_cpu_quota: 200000 }), t)).toBe('2');
  });

  it('reports unlimited when neither value is known', () => {
    expect(formatAgentCPU(agentWith({}), t)).toBe('admin.settings.agents.unlimited');
  });
});

describe('hasAgentResources', () => {
  it('is true for docker agents without host values', () => {
    expect(hasAgentResources(agentWith({}))).toBe(true);
  });

  it('is true for non-docker agents that report host values', () => {
    expect(hasAgentResources(agentWith({ backend: 'local', host_cpus: 8 }))).toBe(true);
  });

  it('is false for backends that report nothing', () => {
    expect(hasAgentResources(agentWith({ backend: 'kubernetes' }))).toBe(false);
  });
});

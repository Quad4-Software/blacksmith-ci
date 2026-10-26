import { describe, expect, it, vi } from 'vitest';

import type { StuckPipeline } from '~/lib/api/types';

const getUserStuckPipelines = vi.fn();

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ getUserStuckPipelines }),
}));

const { useStuckPipelines } = await import('./useStuckPipelines');

describe('useStuckPipelines', () => {
  it('loads the stuck pipelines and recognizes them by repo and number', async () => {
    const pipeline: StuckPipeline = {
      repo_id: 3,
      repo_full_name: 'org/repo',
      pipeline_number: 42,
      created: 1,
      workflows: [{ name: 'build', reason: 'waited_too_long' }],
    };
    getUserStuckPipelines.mockResolvedValue([pipeline]);

    const s = useStuckPipelines();
    await s.load();

    expect(getUserStuckPipelines).toHaveBeenCalledWith({ silent: true });
    expect(s.stuck.value).toEqual([pipeline]);
    expect(s.isStuck(3, 42)).toBe(true);
    expect(s.isStuck(3, 43)).toBe(false);
    expect(s.isStuck(4, 42)).toBe(false);
  });
});

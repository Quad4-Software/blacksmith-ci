import { describe, expect, it } from 'vitest';

import type { WatchOverviewItem } from '~/lib/api/types';

import { failedNames } from './watchDisplay';

function item(failing: WatchOverviewItem['failing_workflows']): WatchOverviewItem {
  return {
    watch: { id: 1 } as WatchOverviewItem['watch'],
    repo_full_name: 'o/r',
    failing: true,
    unread: true,
    failing_workflows: failing,
  };
}

describe('failedNames', () => {
  it('lists each failing workflow with the run it last failed in', () => {
    expect(
      failedNames(
        item([
          { name: 'build', pipeline_number: 10 },
          { name: 'test', pipeline_number: 12 },
        ]),
        'config error',
      ),
    ).toBe('build (#10), test (#12)');
  });

  it('labels a pipeline-level failure', () => {
    expect(failedNames(item([{ name: '', pipeline_number: 5 }]), 'config error')).toBe('config error (#5)');
  });

  it('is empty without failing workflows', () => {
    expect(failedNames(item([]), 'config error')).toBe('');
  });
});

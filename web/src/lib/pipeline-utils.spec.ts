import type { PipelineStep } from '~/lib/api/types';
import { describe, expect, it } from 'vitest';
import { duplicateStepNameIndexes } from './pipeline-utils';

function step(pid: number, name: string): PipelineStep {
  return {
    id: pid,
    uuid: `step-${pid}`,
    pipeline_id: 1,
    pid,
    ppid: 0,
    name,
    state: 'success',
    exit_code: 0,
    original_step_id: 0,
    attempt: 1,
  };
}

describe('duplicateStepNameIndexes', () => {
  it('omits steps whose names are unique', () => {
    const result = duplicateStepNameIndexes([step(1, 'clone'), step(2, 'build'), step(3, 'deploy')]);
    expect(result.size).toBe(0);
  });

  it('assigns 1-based occurrence indexes to duplicate names only', () => {
    const result = duplicateStepNameIndexes([
      step(1, 'clone'),
      step(2, 'Crow (apply)'),
      step(3, 'Crow (apply)'),
      step(4, 'lint'),
    ]);
    expect(result.get(2)).toBe(1);
    expect(result.get(3)).toBe(2);
    expect(result.has(1)).toBe(false);
    expect(result.has(4)).toBe(false);
  });

  it('handles three or more occurrences', () => {
    const result = duplicateStepNameIndexes([step(1, 'apply'), step(2, 'apply'), step(3, 'apply')]);
    expect([...result.entries()]).toEqual([
      [1, 1],
      [2, 2],
      [3, 3],
    ]);
  });
});

import type { PipelineStep, PipelineWorkflow } from '~/lib/api/types';
import { describe, expect, it } from 'vitest';
import { classifyMatrixAxes, duplicateStepNameIndexes, matrixValueColor } from './pipeline-utils';

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

function wf(environ: Record<string, string>): PipelineWorkflow {
  return { name: 'build', environ } as unknown as PipelineWorkflow;
}

describe('classifyMatrixAxes', () => {
  it('hoists axes that are constant across every variant', () => {
    const result = classifyMatrixAxes([
      wf({ ARCH: 'amd64', R_VERSION: '4.3' }),
      wf({ ARCH: 'amd64', R_VERSION: '4.4' }),
    ]);
    expect(result.shared).toEqual([{ key: 'ARCH', value: 'amd64' }]);
    expect(result.primary).toBe('R_VERSION');
    expect(result.secondary).toEqual([]);
    expect(result.partial).toEqual([]);
  });

  it('picks the highest-cardinality varying axis as primary', () => {
    const result = classifyMatrixAxes([
      wf({ R_VERSION: '4.3', SPLIT: '1/2' }),
      wf({ R_VERSION: '4.4', SPLIT: '2/2' }),
      wf({ R_VERSION: '4.5', SPLIT: '1/2' }),
    ]);
    // R_VERSION has 3 distinct values, SPLIT has 2 -> R_VERSION wins.
    expect(result.primary).toBe('R_VERSION');
    expect(result.secondary).toEqual(['SPLIT']);
  });

  it('treats keys present on only some variants as partial, never shared', () => {
    const result = classifyMatrixAxes([
      wf({ ARCH: 'amd64', R_VERSION: '4.4' }),
      wf({ ARCH: 'amd64', R_VERSION: '4.5', BIOC: 'devel' }),
    ]);
    expect(result.shared).toEqual([{ key: 'ARCH', value: 'amd64' }]);
    expect(result.primary).toBe('R_VERSION');
    expect(result.partial).toEqual(['BIOC']);
    expect(result.secondary).toContain('BIOC');
  });

  it('keeps first-seen axis order in secondary', () => {
    const result = classifyMatrixAxes([wf({ A: '1', B: '1', C: '1' }), wf({ A: '2', B: '2', C: '2' })]);
    // All three vary with equal cardinality; first-seen (A) is primary, rest keep order.
    expect(result.primary).toBe('A');
    expect(result.secondary).toEqual(['B', 'C']);
  });
});

describe('matrixValueColor', () => {
  it('is deterministic for a given value', () => {
    expect(matrixValueColor('4.5')).toBe(matrixValueColor('4.5'));
  });

  it('returns a hex color from the palette', () => {
    expect(matrixValueColor('amd64')).toMatch(/^#[0-9a-f]{6}$/i);
  });
});

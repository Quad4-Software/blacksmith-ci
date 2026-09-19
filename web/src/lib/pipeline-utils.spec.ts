import type { PipelineStep, PipelineWorkflow } from '~/lib/api/types';
import { describe, expect, it } from 'vitest';
import {
  classifyMatrixAxes,
  duplicateStepNameIndexes,
  getDisplayedAttempts,
  getLatestAttempts,
  getWorkflowAttempts,
  matrixValueColor,
  stepForAttemptSwitch,
} from './pipeline-utils';

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

function workflow(id: number, pipelineName: string, attempt = 1): PipelineWorkflow {
  return {
    id,
    pid: id,
    name: 'python-ubuntu',
    pipeline_name: pipelineName,
    environ: { containerfile: 'python/ubuntu/Containerfile', OS_VERSION: 'noble' },
    attempt,
    children: [],
  } as unknown as PipelineWorkflow;
}

describe('workflow attempts', () => {
  it('keeps identical matrix variants from different pipelines', () => {
    const amd64 = workflow(1, 'build-amd64');
    const arm64 = workflow(2, 'build-arm64');

    expect(getLatestAttempts([amd64, arm64])).toEqual([amd64, arm64]);
  });

  it('only groups reruns from the same pipeline', () => {
    const amd64First = workflow(1, 'build-amd64');
    const amd64Second = workflow(2, 'build-amd64', 2);
    const arm64 = workflow(3, 'build-arm64');

    expect(getLatestAttempts([amd64First, arm64, amd64Second])).toEqual([amd64Second, arm64]);
    expect(getWorkflowAttempts([amd64First, arm64, amd64Second], amd64Second)).toEqual([amd64First, amd64Second]);
  });
});

function attempt(id: number, attemptNumber: number, steps: PipelineStep[], name = 'build-base'): PipelineWorkflow {
  return {
    id,
    pid: id,
    name,
    pipeline_name: name,
    attempt: attemptNumber,
    children: steps,
  } as unknown as PipelineWorkflow;
}

describe('getDisplayedAttempts', () => {
  const first = attempt(1, 1, [step(2, 'clone'), step(3, 'build-ubuntu')]);
  const second = attempt(4, 2, [step(5, 'clone'), step(6, 'build-ubuntu')]);

  it('shows the latest attempt when no step is selected', () => {
    expect(getDisplayedAttempts([first, second], null)).toEqual([second]);
    expect(getDisplayedAttempts([first, second], undefined)).toEqual([second]);
  });

  it('shows the attempt owning the selected step', () => {
    expect(getDisplayedAttempts([first, second], 3)).toEqual([first]);
    expect(getDisplayedAttempts([first, second], 6)).toEqual([second]);
  });

  it('pins only the variant owning the selection', () => {
    const other = attempt(7, 1, [step(8, 'lint')], 'lint');
    const otherRerun = attempt(9, 2, [step(10, 'lint')], 'lint');

    expect(getDisplayedAttempts([first, second, other, otherRerun], 3)).toEqual([first, otherRerun]);
  });

  it('falls back to the latest attempt for an unknown step id', () => {
    expect(getDisplayedAttempts([first, second], 99)).toEqual([second]);
  });
});

describe('stepForAttemptSwitch', () => {
  const target = attempt(4, 2, [step(5, 'clone'), step(6, 'build-ubuntu')]);

  it('keeps the same step across attempts', () => {
    expect(stepForAttemptSwitch(target, 'build-ubuntu')?.pid).toBe(6);
  });

  it('falls back to the first step when the attempt did not run it', () => {
    expect(stepForAttemptSwitch(target, 'deploy')?.pid).toBe(5);
    expect(stepForAttemptSwitch(target, undefined)?.pid).toBe(5);
  });

  it('returns undefined for an attempt without steps', () => {
    expect(stepForAttemptSwitch(attempt(7, 1, []), 'clone')).toBeUndefined();
  });
});

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

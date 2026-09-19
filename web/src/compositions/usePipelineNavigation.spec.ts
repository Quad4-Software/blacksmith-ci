import { describe, expect, it } from 'vitest';
import type { PipelineStep, PipelineWorkflow } from '~/lib/api/types';
import { getRerunFollowStepId } from './usePipelineNavigation';

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

const first = attempt(1, 1, [step(2, 'clone'), step(3, 'build-ubuntu')]);
const second = attempt(4, 2, [step(5, 'clone'), step(6, 'build-ubuntu')]);

describe('getRerunFollowStepId', () => {
  it('does not move the selection on the first call', () => {
    expect(getRerunFollowStepId([first], 3, new Map())).toBeNull();
    // Opening a link to a previous run must stay on that run.
    expect(getRerunFollowStepId([first, second], 3, new Map())).toBeNull();
  });

  it('follows a rerun into the same step of the new attempt', () => {
    const seen = new Map<string, number>();
    getRerunFollowStepId([first], 3, seen);

    expect(getRerunFollowStepId([first, second], 3, seen)).toBe(6);
  });

  it('falls back to the first step when the rerun skipped the selected step', () => {
    const seen = new Map<string, number>();
    getRerunFollowStepId([first], 3, seen);

    const partialRerun = attempt(4, 2, [step(5, 'clone'), step(6, 'lint')]);
    expect(getRerunFollowStepId([first, partialRerun], 3, seen)).toBe(5);
  });

  it('ignores reruns of other workflows', () => {
    const lint = attempt(7, 1, [step(8, 'lint')], 'lint');
    const lintRerun = attempt(9, 2, [step(10, 'lint')], 'lint');
    const seen = new Map<string, number>();
    getRerunFollowStepId([first, lint], 3, seen);

    expect(getRerunFollowStepId([first, lint, lintRerun], 3, seen)).toBeNull();
  });

  it('does nothing while no new attempt appears', () => {
    const seen = new Map<string, number>();
    getRerunFollowStepId([first, second], 6, seen);

    expect(getRerunFollowStepId([first, second], 6, seen)).toBeNull();
  });

  it('stays put when no step is selected', () => {
    const seen = new Map<string, number>();
    getRerunFollowStepId([first], null, seen);

    expect(getRerunFollowStepId([first, second], null, seen)).toBeNull();
  });
});

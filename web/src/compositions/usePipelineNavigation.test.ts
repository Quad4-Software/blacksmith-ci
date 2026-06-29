import { describe, expect, it } from 'vitest';

import type { PipelineWorkflow } from '~/lib/api/types';
import { StepType } from '~/lib/api/types';

import { getAutoFollowStepId } from './usePipelineNavigation';

function step(pid: number, state: string, type?: StepType) {
  return { pid, state, type } as PipelineWorkflow['children'][number];
}

function workflow(pid: number, children: PipelineWorkflow['children']): PipelineWorkflow {
  return { pid, children } as PipelineWorkflow;
}

describe('getAutoFollowStepId', () => {
  it('follows the running step within the current workflow', () => {
    const workflows = [workflow(1, [step(10, 'success'), step(11, 'running'), step(12, 'pending')])];
    // currently on the finished first step -> should advance to the running one
    expect(getAutoFollowStepId(workflows, 10)).toBe(11);
  });

  it('does not follow a running service step (the reported bug)', () => {
    // A service stays "running" for the whole workflow. Auto-follow must skip it
    // and follow the actual running task step instead.
    const workflows = [
      workflow(1, [step(10, 'running', StepType.Service), step(11, 'success'), step(12, 'running', StepType.Commands)]),
    ];
    // on step 11 -> should advance to the running command step 12, not the service 10
    expect(getAutoFollowStepId(workflows, 11)).toBe(12);
  });

  it('does not jump to a service when only the service is running', () => {
    const workflows = [workflow(1, [step(10, 'running', StepType.Service), step(11, 'success'), step(12, 'pending')])];
    // nothing to follow -> stay where we are
    expect(getAutoFollowStepId(workflows, 11)).toBeNull();
  });

  it('returns null when already on the running step', () => {
    const workflows = [workflow(1, [step(10, 'success'), step(11, 'running')])];
    expect(getAutoFollowStepId(workflows, 11)).toBeNull();
  });

  it('finds the first running task step across workflows when nothing is selected', () => {
    const workflows = [
      workflow(1, [step(10, 'success')]),
      workflow(2, [step(20, 'running', StepType.Service), step(21, 'running', StepType.Commands)]),
    ];
    expect(getAutoFollowStepId(workflows, null)).toBe(21);
  });
});

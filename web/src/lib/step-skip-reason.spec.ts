import type { PipelineStatus, PipelineStep, PipelineWorkflow } from '~/lib/api/types';
import { describe, expect, it } from 'vitest';
import { stepSkipReasons } from './step-skip-reason';

function step(pid: number, name: string, state: PipelineStatus, dependsOn: string[] = []): PipelineStep {
  return {
    id: pid,
    uuid: `step-${pid}`,
    pipeline_id: 1,
    pid,
    ppid: 1,
    name,
    state,
    exit_code: 0,
    depends_on: dependsOn,
    original_step_id: 0,
    attempt: 1,
  };
}

function workflow(state: PipelineStatus, children: PipelineStep[]): PipelineWorkflow {
  return {
    id: 1,
    pipeline_id: 1,
    pid: 1,
    name: 'build',
    pipeline_name: 'build',
    state,
    original_workflow_id: 0,
    attempt: 1,
    children,
  };
}

describe('stepSkipReasons', () => {
  it('names the nearest failed step a skipped step depends on', () => {
    const reasons = stepSkipReasons(
      workflow('failure', [
        step(1, 'clone', 'success'),
        step(2, 'test', 'failure', ['clone']),
        step(3, 'package', 'skipped', ['test']),
        step(4, 'deploy', 'skipped', ['package']),
        step(5, 'lint', 'success', ['clone']),
      ]),
      'failure',
    );

    expect(reasons.get(3)).toEqual({ kind: 'dependency_failed', dependency: 'test' });
    expect(reasons.get(4)).toEqual({ kind: 'dependency_failed', dependency: 'test' });
    expect(reasons.has(5)).toBe(false);
  });

  it('does not blame a failed step whose failure is ignored', () => {
    const flaky = { ...step(2, 'flaky', 'failure', ['clone']), failure: 'ignore' };
    const reasons = stepSkipReasons(
      workflow('success', [step(1, 'clone', 'success'), flaky, step(3, 'notify', 'skipped', ['flaky'])]),
      'success',
    );

    expect(reasons.get(3)).toEqual({ kind: 'only_on_failure' });
  });

  it('reports a step that only runs on failure when its lane succeeded', () => {
    const reasons = stepSkipReasons(
      workflow('failure', [
        step(1, 'clone', 'success'),
        step(2, 'build', 'success', ['clone']),
        step(3, 'on-failure', 'skipped', ['build']),
        step(4, 'test', 'failure', ['clone']),
      ]),
      'failure',
    );

    expect(reasons.get(3)).toEqual({ kind: 'only_on_failure' });
  });

  it('reports a cancelled pipeline', () => {
    const reasons = stepSkipReasons(
      workflow('killed', [step(1, 'clone', 'success'), step(2, 'build', 'skipped', ['clone'])]),
      'killed',
    );

    expect(reasons.get(2)).toEqual({ kind: 'cancelled' });
  });

  it('reports a skipped or declined workflow', () => {
    const steps = () => [step(1, 'clone', 'skipped'), step(2, 'build', 'skipped', ['clone'])];

    expect(stepSkipReasons(workflow('skipped', steps()), 'success').get(2)).toEqual({ kind: 'workflow_skipped' });
    expect(stepSkipReasons(workflow('declined', steps()), 'declined').get(2)).toEqual({ kind: 'workflow_declined' });
  });

  it('reports a workflow that stopped without a failed step', () => {
    const reasons = stepSkipReasons(
      workflow('error', [step(1, 'clone', 'skipped'), step(2, 'build', 'skipped', ['clone'])]),
      'error',
    );

    expect(reasons.get(1)).toEqual({ kind: 'workflow_stopped' });
  });

  it('terminates on a dependency cycle', () => {
    const reasons = stepSkipReasons(
      workflow('success', [step(1, 'a', 'skipped', ['b']), step(2, 'b', 'skipped', ['a'])]),
      'success',
    );

    expect(reasons.get(1)).toEqual({ kind: 'only_on_failure' });
  });
});

import { describe, expect, it } from 'vitest';

import type { Pipeline, PipelineStep, PipelineWorkflow } from '~/lib/api/types';

import { mergePipeline, stateRank } from './pipeline-merge';

function step(overrides: Partial<PipelineStep> & { id: number; state: PipelineStep['state'] }): PipelineStep {
  return {
    pipeline_id: 1,
    pid: overrides.id,
    ppid: 0,
    uuid: `step-${overrides.id}`,
    name: `step-${overrides.id}`,
    exit_code: 0,
    original_step_id: 0,
    attempt: 1,
    ...overrides,
  };
}

function workflow(
  overrides: Partial<PipelineWorkflow> & { id: number; state: PipelineWorkflow['state']; children?: PipelineStep[] },
): PipelineWorkflow {
  return {
    pipeline_id: 1,
    pid: overrides.id,
    name: `wf-${overrides.id}`,
    pipeline_name: `wf-${overrides.id}`,
    original_workflow_id: 0,
    attempt: 1,
    children: overrides.children ?? [],
    ...overrides,
  };
}

function pipeline(
  overrides: Partial<Pipeline> & { number: number; status: Pipeline['status']; workflows?: PipelineWorkflow[] },
): Pipeline {
  return {
    id: overrides.number,
    parent: 0,
    event: 'push',
    errors: [],
    created: 0,
    updated: 0,
    started: 0,
    finished: 0,
    deploy_to: '',
    commit: 'deadbeef',
    branch: 'main',
    message: 'msg',
    timestamp: 0,
    ref: 'refs/heads/main',
    refspec: '',
    clone_url: '',
    title: '',
    sender: '',
    author: '',
    author_avatar: '',
    author_email: '',
    forge_url: '',
    reviewed_by: '',
    reviewed: 0,
    logs_deleted: 0,
    ...overrides,
  } as Pipeline;
}

describe('stateRank', () => {
  it('orders the lifecycle: pending/blocked < running/started < terminal', () => {
    expect(stateRank('pending')).toEqual(stateRank('blocked'));
    expect(stateRank('pending')).toBeLessThan(stateRank('running'));
    expect(stateRank('running')).toEqual(stateRank('started'));
    expect(stateRank('running')).toBeLessThan(stateRank('success'));
    expect(stateRank('success')).toEqual(stateRank('failure'));
    expect(stateRank('success')).toEqual(stateRank('skipped'));
    expect(stateRank('success')).toEqual(stateRank('killed'));
    expect(stateRank('success')).toEqual(stateRank('error'));
    expect(stateRank('success')).toEqual(stateRank('declined'));
  });
});

describe('mergePipeline', () => {
  it('returns the incoming snapshot verbatim when nothing is stored yet', () => {
    const incoming = pipeline({ number: 1, status: 'running' });
    expect(mergePipeline(undefined, incoming)).toBe(incoming);
  });

  it('does not regress workflow state when a stale HTTP response arrives after SSE', () => {
    // SSE delivered first: workflow is running with a running step
    const fromSse = pipeline({
      number: 1,
      status: 'running',
      workflows: [
        workflow({
          id: 10,
          state: 'running',
          started: 200,
          children: [step({ id: 100, state: 'running', started: 200 })],
        }),
      ],
    });
    // HTTP response (read just before the agent flipped state) lands afterwards
    const fromHttp = pipeline({
      number: 1,
      status: 'pending',
      workflows: [
        workflow({
          id: 10,
          state: 'pending',
          children: [step({ id: 100, state: 'pending' })],
        }),
      ],
    });

    const merged = mergePipeline(fromSse, fromHttp);
    expect(merged.status).toBe('running');
    expect(merged.workflows![0].state).toBe('running');
    expect(merged.workflows![0].started).toBe(200);
    expect(merged.workflows![0].children[0].state).toBe('running');
  });

  it('does not regress step state when a stale workflow snapshot arrives after step finished', () => {
    // A snapshot where the workflow is still considered running but step #100 already finished.
    // We expect this to be preserved when a stale snapshot showing step running arrives later.
    const fresh = pipeline({
      number: 1,
      status: 'running',
      workflows: [
        workflow({
          id: 10,
          state: 'running',
          children: [
            step({ id: 100, state: 'success', finished: 300 }),
            step({ id: 101, state: 'running', started: 300 }),
          ],
        }),
      ],
    });
    const stale = pipeline({
      number: 1,
      status: 'running',
      workflows: [
        workflow({
          id: 10,
          state: 'running',
          children: [step({ id: 100, state: 'running', started: 200 }), step({ id: 101, state: 'pending' })],
        }),
      ],
    });

    const merged = mergePipeline(fresh, stale);
    expect(merged.workflows![0].children.find((s) => s.id === 100)!.state).toBe('success');
    expect(merged.workflows![0].children.find((s) => s.id === 100)!.finished).toBe(300);
    expect(merged.workflows![0].children.find((s) => s.id === 101)!.state).toBe('running');
  });

  it('moves state forward when the incoming snapshot is more advanced', () => {
    const existing = pipeline({
      number: 1,
      status: 'running',
      workflows: [
        workflow({
          id: 10,
          state: 'running',
          children: [step({ id: 100, state: 'running', started: 200 })],
        }),
      ],
    });
    const incoming = pipeline({
      number: 1,
      status: 'success',
      finished: 400,
      workflows: [
        workflow({
          id: 10,
          state: 'success',
          finished: 400,
          children: [step({ id: 100, state: 'success', started: 200, finished: 400 })],
        }),
      ],
    });

    const merged = mergePipeline(existing, incoming);
    expect(merged.status).toBe('success');
    expect(merged.finished).toBe(400);
    expect(merged.workflows![0].state).toBe('success');
    expect(merged.workflows![0].children[0].state).toBe('success');
    expect(merged.workflows![0].children[0].finished).toBe(400);
  });

  it('keeps workflows from existing that are missing from a stale incoming snapshot', () => {
    // Pipeline has a rerun: existing knows both attempts (wf id 10 and 20). A stale
    // snapshot that predates the rerun only knows about wf 10. We must not drop wf 20.
    const existing = pipeline({
      number: 1,
      status: 'running',
      workflows: [
        workflow({ id: 10, state: 'failure', attempt: 1 }),
        workflow({ id: 20, state: 'running', attempt: 2, original_workflow_id: 10 }),
      ],
    });
    const stale = pipeline({
      number: 1,
      status: 'failure',
      workflows: [workflow({ id: 10, state: 'failure', attempt: 1 })],
    });

    const merged = mergePipeline(existing, stale);
    const ids = merged.workflows!.map((w) => w.id).toSorted((a, b) => a - b);
    expect(ids).toEqual([10, 20]);
    expect(merged.workflows!.find((w) => w.id === 20)!.state).toBe('running');
  });

  it('adopts new workflows from incoming that did not exist locally', () => {
    const existing = pipeline({
      number: 1,
      status: 'running',
      workflows: [workflow({ id: 10, state: 'success' })],
    });
    const incoming = pipeline({
      number: 1,
      status: 'running',
      workflows: [
        workflow({ id: 10, state: 'success' }),
        workflow({ id: 20, state: 'running', attempt: 2, original_workflow_id: 10 }),
      ],
    });

    const merged = mergePipeline(existing, incoming);
    expect(merged.workflows!.map((w) => w.id)).toEqual([10, 20]);
  });

  it('falls back to incoming when no existing workflows are present', () => {
    const existing = pipeline({ number: 1, status: 'pending', workflows: [] });
    const incoming = pipeline({
      number: 1,
      status: 'running',
      workflows: [workflow({ id: 10, state: 'running' })],
    });

    const merged = mergePipeline(existing, incoming);
    expect(merged.workflows).toHaveLength(1);
    expect(merged.workflows![0].id).toBe(10);
  });

  it('keeps existing workflows when incoming has none', () => {
    // E.g. a feed-style event that doesn't include workflows must not wipe them out.
    const existing = pipeline({
      number: 1,
      status: 'running',
      workflows: [workflow({ id: 10, state: 'running' })],
    });
    const incoming = pipeline({ number: 1, status: 'running' });

    const merged = mergePipeline(existing, incoming);
    expect(merged.workflows).toHaveLength(1);
    expect(merged.workflows![0].state).toBe('running');
  });

  it('uses incoming when state ranks tie (incoming is at least as fresh)', () => {
    const existing = pipeline({
      number: 1,
      status: 'running',
      message: 'old',
      workflows: [workflow({ id: 10, state: 'running' })],
    });
    const incoming = pipeline({
      number: 1,
      status: 'running',
      message: 'new',
      workflows: [workflow({ id: 10, state: 'running' })],
    });

    const merged = mergePipeline(existing, incoming);
    expect(merged.message).toBe('new');
  });
});

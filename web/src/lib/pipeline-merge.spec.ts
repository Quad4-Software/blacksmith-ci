import type { Pipeline, PipelineStep, PipelineWorkflow } from '~/lib/api/types';
import { describe, expect, it } from 'vitest';
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
  it('orders the lifecycle: pending < blocked < preparing < running/started < terminal', () => {
    // `blocked` ranks strictly above `pending`: the approval gate only activates after
    // the workflow was pending, so a stale pending snapshot must never tie/overwrite it.
    expect(stateRank('pending')).toBeLessThan(stateRank('blocked'));
    expect(stateRank('blocked')).toBeLessThan(stateRank('preparing'));
    expect(stateRank('preparing')).toBeLessThan(stateRank('running'));
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

  it('applies a pending->blocked gate activation and carries approval_state', () => {
    // Workflow was pending on mount; an SSE event fires when the approval gate activates,
    // flipping it to blocked with approval_state=pending. This must be applied.
    const existing = pipeline({
      number: 1,
      status: 'pending',
      workflows: [workflow({ id: 10, state: 'pending', children: [step({ id: 100, state: 'pending' })] })],
    });
    const fromSse = pipeline({
      number: 1,
      status: 'blocked',
      workflows: [
        workflow({
          id: 10,
          state: 'blocked',
          requires_approval: true,
          approval_state: 'pending',
          children: [step({ id: 100, state: 'pending' })],
        }),
      ],
    });

    const merged = mergePipeline(existing, fromSse);
    expect(merged.workflows![0].state).toBe('blocked');
    expect(merged.workflows![0].approval_state).toBe('pending');
  });

  it('does not regress a blocked workflow when a stale pending snapshot arrives after the gate activated', () => {
    // The mount-time GET /pipelines/N (read while still pending) can resolve AFTER the
    // blocked SSE event. Its stale pending snapshot must not regress blocked or drop
    // approval_state.
    const fromSse = pipeline({
      number: 1,
      status: 'blocked',
      workflows: [
        workflow({
          id: 10,
          state: 'blocked',
          requires_approval: true,
          approval_state: 'pending',
          children: [step({ id: 100, state: 'pending' })],
        }),
      ],
    });
    const stalePending = pipeline({
      number: 1,
      status: 'pending',
      workflows: [workflow({ id: 10, state: 'pending', children: [step({ id: 100, state: 'pending' })] })],
    });

    const merged = mergePipeline(fromSse, stalePending);
    expect(merged.workflows![0].state).toBe('blocked');
    expect(merged.workflows![0].approval_state).toBe('pending');
  });

  it('drops the pre-approval rows when an approval replaces them in place', () => {
    // Approving a gated pipeline deletes the blocked workflow/step rows and inserts
    // freshly-compiled ones with NEW ids but the SAME pid and attempt (attempt is only
    // bumped by reruns). The stale pre-approval snapshot must not survive the merge as a
    // phantom — its ids no longer exist server-side and 404 on any fetch (logs, approval).
    const blocked = pipeline({
      number: 5,
      status: 'blocked',
      workflows: [
        workflow({
          id: 1,
          pid: 1,
          attempt: 1,
          state: 'blocked',
          approval_state: 'pending',
          children: [step({ id: 10, pid: 1, attempt: 1, state: 'blocked' })],
        }),
      ],
    });
    const approved = pipeline({
      number: 5,
      status: 'running',
      workflows: [
        workflow({
          id: 2,
          pid: 1,
          attempt: 1,
          state: 'running',
          approval_state: 'approved',
          children: [step({ id: 20, pid: 1, attempt: 1, state: 'running' })],
        }),
      ],
    });

    const merged = mergePipeline(blocked, approved);

    const wfIds = merged.workflows!.map((w) => w.id);
    const stepIds = merged.workflows!.flatMap((w) => w.children ?? []).map((s) => s.id);
    expect(wfIds).toEqual([2]);
    expect(stepIds).toEqual([20]);
    expect(merged.workflows![0].approval_state).toBe('approved');
  });

  it('keeps every attempt when a rerun adds a new one alongside the original', () => {
    // A rerun bumps `attempt` and KEEPS the prior attempt as history, so both rows are
    // legitimate. A stale pre-rerun snapshot must not drop the freshly-created attempt,
    // and the id-regeneration fix must not collapse the two attempts into one.
    const afterRerun = pipeline({
      number: 7,
      status: 'running',
      workflows: [
        workflow({
          id: 2,
          pid: 1,
          attempt: 2,
          state: 'running',
          children: [step({ id: 20, pid: 1, attempt: 2, state: 'running' })],
        }),
      ],
    });
    const stalePreRerun = pipeline({
      number: 7,
      status: 'failure',
      workflows: [
        workflow({
          id: 1,
          pid: 1,
          attempt: 1,
          state: 'failure',
          children: [step({ id: 10, pid: 1, attempt: 1, state: 'failure' })],
        }),
      ],
    });

    const merged = mergePipeline(afterRerun, stalePreRerun);

    const wfIds = merged.workflows!.map((w) => w.id).toSorted((a, b) => a - b);
    const attempts = merged.workflows!.map((w) => w.attempt).toSorted((a, b) => a - b);
    expect(wfIds).toEqual([1, 2]);
    expect(attempts).toEqual([1, 2]);
  });
});

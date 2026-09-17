import type { Pipeline, PipelineStatus, PipelineStep, PipelineWorkflow } from '~/lib/api/types';

/**
 * State-machine progression rank for a single entity (step, workflow, or pipeline).
 *
 * The lifecycle is monotonic forward: workflows can progress through `blocked`, while steps
 * progress through `preparing`, before either reaches `running`/`started` and then terminal.
 * We use this rank to decide which of two concurrent snapshots of the same entity is "more
 * recent" so we never regress state when a stale HTTP response races a fresh SSE update.
 *
 * `blocked` ranks strictly above `pending`: a workflow only becomes `blocked` once its
 * approval gate activates, which always happens after it was `pending`. Giving them the
 * same rank let a stale `pending` HTTP snapshot (fired on mount, before the gate
 * activated) tie and overwrite a fresher `blocked` SSE event, discarding `approval_state`
 * and leaving the approval panel hidden until a full reload.
 */
export function stateRank(state: PipelineStatus): number {
  switch (state) {
    case 'pending':
      return 1;
    case 'blocked':
      return 2;
    case 'preparing':
      return 3;
    case 'started':
    case 'running':
      return 4;
    case 'declined':
    case 'error':
    case 'failure':
    case 'killed':
    case 'skipped':
    case 'success':
      return 5;
    default:
      return 0;
  }
}

// Stable identity of a step/workflow *across regenerations of the row*. The
// database `id` is NOT stable: approving a gated pipeline deletes the blocked
// workflow/step rows and inserts freshly-compiled ones with new ids (see the
// server-side Approve → WorkflowsReplace), and a rerun inserts additional rows.
// `pid` is the deterministic pipeline-local position (unique per pipeline) and
// `attempt` distinguishes reruns, so `pid:attempt` identifies "the same logical
// entity" regardless of which `id` the DB assigned. Keying the merge on `id`
// instead let an approval's pre- and post-replace snapshots coexist, leaving
// deleted rows in the store as phantoms whose ids 404 on any fetch.
function entityKey(entity: { pid: number; attempt: number }): string {
  return `${entity.pid}:${entity.attempt}`;
}

function mergeStep(existing: PipelineStep, incoming: PipelineStep): PipelineStep {
  // A replacement row starts a new lifecycle; never retain a deleted row's state.
  if (existing.id !== incoming.id) return incoming.id > existing.id ? incoming : existing;
  return stateRank(incoming.state) >= stateRank(existing.state) ? incoming : existing;
}

function mergeSteps(existing: PipelineStep[], incoming: PipelineStep[]): PipelineStep[] {
  if (existing.length === 0) return incoming;
  if (incoming.length === 0) return existing;

  const byKey = new Map<string, PipelineStep>();
  for (const s of existing) byKey.set(entityKey(s), s);
  for (const s of incoming) {
    const key = entityKey(s);
    const ex = byKey.get(key);
    byKey.set(key, ex ? mergeStep(ex, s) : s);
  }

  const seen = new Set<string>();
  const result: PipelineStep[] = [];
  for (const s of incoming) {
    const key = entityKey(s);
    seen.add(key);
    result.push(byKey.get(key)!);
  }
  for (const s of existing) {
    const key = entityKey(s);
    if (!seen.has(key)) result.push(byKey.get(key)!);
  }
  return result;
}

function isApprovalResolved(workflow: PipelineWorkflow): boolean {
  return (
    workflow.approval_state === 'approved' ||
    workflow.approval_state === 'declined' ||
    workflow.approval_state === 'expired'
  );
}

function mergeWorkflow(existing: PipelineWorkflow, incoming: PipelineWorkflow): PipelineWorkflow {
  // IDs increase when approval replaces rows. Keep the entire newer generation,
  // including its children, even when an older blocked snapshot arrives last.
  if (existing.id !== incoming.id) return incoming.id > existing.id ? incoming : existing;

  // Resolving a gate legitimately moves blocked back to pending for dispatch.
  const existingResolved = isApprovalResolved(existing);
  const incomingResolved = isApprovalResolved(incoming);
  if (existingResolved !== incomingResolved) return incomingResolved ? incoming : existing;
  // Pick the snapshot of the workflow itself with the more advanced state for top-level
  // fields (agent_id, started, finished, ...).
  const winner = stateRank(incoming.state) >= stateRank(existing.state) ? incoming : existing;
  // Children are merged independently — multiple step transitions can land between two
  // workflow-level snapshots, so even the "losing" snapshot may carry newer step state.
  const mergedChildren = mergeSteps(existing.children ?? [], incoming.children ?? []);
  return { ...winner, children: mergedChildren };
}

function mergeWorkflows(existing: PipelineWorkflow[], incoming: PipelineWorkflow[]): PipelineWorkflow[] {
  if (existing.length === 0) return incoming;
  if (incoming.length === 0) return existing;

  const byKey = new Map<string, PipelineWorkflow>();
  for (const w of existing) byKey.set(entityKey(w), w);
  for (const w of incoming) {
    const key = entityKey(w);
    const ex = byKey.get(key);
    byKey.set(key, ex ? mergeWorkflow(ex, w) : w);
  }

  // Preserve incoming order, then append any workflows only present in existing so that
  // a stale snapshot can't drop a rerun-created workflow (a rerun bumps `attempt`, so its
  // key differs from the original and both are kept). Keying on `pid:attempt` rather than
  // `id` means an approval's replaced-in-place workflow (same pid, same attempt, new id)
  // collapses onto the incoming row instead of surviving as a phantom.
  const seen = new Set<string>();
  const result: PipelineWorkflow[] = [];
  for (const w of incoming) {
    const key = entityKey(w);
    seen.add(key);
    result.push(byKey.get(key)!);
  }
  for (const w of existing) {
    const key = entityKey(w);
    if (!seen.has(key)) result.push(byKey.get(key)!);
  }
  return result;
}

/**
 * Merge an incoming pipeline snapshot with the snapshot already in the store, keeping
 * the more advanced state for every workflow and step.
 *
 * Why: the pipeline store is written from two concurrent sources — `GET /pipelines/N`
 * fired on mount/route change and SSE events broadcast on every gRPC step transition.
 * A naive `{ ...existing, ...incoming }` lets a stale HTTP response (read from the DB
 * before the agent flipped a step to `running`) overwrite a newer SSE update, leaving
 * the UI stuck showing `pending` until the next state change (typically: step finish).
 *
 * The merge guards against this by ranking states and only ever moving an entity
 * forward in its lifecycle. It is symmetric: it also protects against an old SSE
 * message arriving after a newer one (`go (*s)(message)` in the pubsub fans messages
 * out into independent goroutines whose delivery order is not guaranteed).
 */
export function mergePipeline(existing: Pipeline | undefined, incoming: Pipeline): Pipeline {
  if (!existing) return incoming;

  // Review marks a new lifecycle: approval returns a blocked pipeline to pending.
  const incomingMoreAdvanced =
    existing.reviewed !== incoming.reviewed
      ? incoming.reviewed > existing.reviewed
      : stateRank(incoming.status) >= stateRank(existing.status);
  // When incoming is more advanced, its scalars (status, started, finished, errors, ...)
  // win. When it's stale, we keep existing's scalars but still let incoming fill in any
  // immutable metadata that happens to be unset locally.
  const base: Pipeline = incomingMoreAdvanced ? { ...existing, ...incoming } : { ...incoming, ...existing };

  const existingWfs = existing.workflows ?? [];
  const incomingWfs = incoming.workflows ?? [];
  if (existingWfs.length > 0 && incomingWfs.length > 0) {
    base.workflows = mergeWorkflows(existingWfs, incomingWfs);
  } else if (incomingWfs.length > 0) {
    base.workflows = incomingWfs;
  } else if (existingWfs.length > 0) {
    base.workflows = existingWfs;
  }

  return base;
}

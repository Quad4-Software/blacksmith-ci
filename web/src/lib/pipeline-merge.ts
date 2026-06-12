import type { Pipeline, PipelineStatus, PipelineStep, PipelineWorkflow } from '~/lib/api/types';

/**
 * State-machine progression rank for a single entity (step, workflow, or pipeline).
 *
 * The pipeline lifecycle is monotonic forward: `pending` → `blocked` (approval gate
 * activates) → `running`/`started` → terminal. We use this rank to decide which of two
 * concurrent snapshots of the same entity is "more recent" so we never regress state
 * when a stale HTTP response races a fresh SSE update (or vice-versa).
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
    case 'started':
    case 'running':
      return 3;
    case 'declined':
    case 'error':
    case 'failure':
    case 'killed':
    case 'skipped':
    case 'success':
      return 4;
    default:
      return 0;
  }
}

function mergeStep(existing: PipelineStep, incoming: PipelineStep): PipelineStep {
  return stateRank(incoming.state) >= stateRank(existing.state) ? incoming : existing;
}

function mergeSteps(existing: PipelineStep[], incoming: PipelineStep[]): PipelineStep[] {
  if (existing.length === 0) return incoming;
  if (incoming.length === 0) return existing;

  const byId = new Map<number, PipelineStep>();
  for (const s of existing) byId.set(s.id, s);
  for (const s of incoming) {
    const ex = byId.get(s.id);
    byId.set(s.id, ex ? mergeStep(ex, s) : s);
  }

  const seen = new Set<number>();
  const result: PipelineStep[] = [];
  for (const s of incoming) {
    seen.add(s.id);
    result.push(byId.get(s.id)!);
  }
  for (const s of existing) {
    if (!seen.has(s.id)) result.push(byId.get(s.id)!);
  }
  return result;
}

function mergeWorkflow(existing: PipelineWorkflow, incoming: PipelineWorkflow): PipelineWorkflow {
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

  const byId = new Map<number, PipelineWorkflow>();
  for (const w of existing) byId.set(w.id, w);
  for (const w of incoming) {
    const ex = byId.get(w.id);
    byId.set(w.id, ex ? mergeWorkflow(ex, w) : w);
  }

  // Preserve incoming order, then append any workflows only present in existing so that
  // a stale snapshot can't drop a rerun-created workflow.
  const seen = new Set<number>();
  const result: PipelineWorkflow[] = [];
  for (const w of incoming) {
    seen.add(w.id);
    result.push(byId.get(w.id)!);
  }
  for (const w of existing) {
    if (!seen.has(w.id)) result.push(byId.get(w.id)!);
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

  const incomingMoreAdvanced = stateRank(incoming.status) >= stateRank(existing.status);
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

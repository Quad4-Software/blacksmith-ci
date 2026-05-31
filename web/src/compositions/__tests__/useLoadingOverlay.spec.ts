import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { __resetLoadingOverlay, useLoadingOverlay } from '../useLoadingOverlay';

describe('useLoadingOverlay', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    __resetLoadingOverlay();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('does not show overlay if action resolves before 200ms', async () => {
    const { state, withOverlay } = useLoadingOverlay();

    const promise = withOverlay('working', async () => {
      await vi.advanceTimersByTimeAsync(50);
      return 'done';
    });

    await vi.advanceTimersByTimeAsync(50);
    expect(state.visible).toBe(false);

    await promise;
    expect(state.visible).toBe(false);
  });

  it('shows overlay once 200ms elapses while action is still pending', async () => {
    const { state, withOverlay } = useLoadingOverlay();

    const promise = withOverlay('working', async () => {
      await vi.advanceTimersByTimeAsync(500);
      return 'done';
    });

    await vi.advanceTimersByTimeAsync(200);
    expect(state.visible).toBe(true);
    expect(state.text).toBe('working');

    await vi.advanceTimersByTimeAsync(400);
    await promise;
    expect(state.visible).toBe(false);
  });

  it('resolves with the wrapped action result', async () => {
    const { withOverlay } = useLoadingOverlay();

    const promise = withOverlay('working', async () => 42);
    await vi.runAllTimersAsync();
    await expect(promise).resolves.toBe(42);
  });

  it('hides overlay and re-throws when action throws', async () => {
    const { state, withOverlay } = useLoadingOverlay();

    const promise = withOverlay('working', async () => {
      await vi.advanceTimersByTimeAsync(300);
      throw new Error('boom');
    });
    // Attach a no-op handler immediately so the rejection is never "unhandled"
    // before expect(...).rejects below attaches its own handler asynchronously.
    promise.catch(() => {});

    await vi.advanceTimersByTimeAsync(200);
    expect(state.visible).toBe(true);

    await vi.advanceTimersByTimeAsync(200);
    await expect(promise).rejects.toThrow('boom');
    expect(state.visible).toBe(false);
  });

  it('keeps overlay visible while concurrent calls are pending and hides only after all settle', async () => {
    const { state, withOverlay } = useLoadingOverlay();

    let resolveA: (() => void) | undefined;
    let resolveB: (() => void) | undefined;
    const promiseA = new Promise<void>((resolve) => {
      resolveA = resolve;
    });
    const promiseB = new Promise<void>((resolve) => {
      resolveB = resolve;
    });

    const a = withOverlay('first', async () => promiseA);
    const b = withOverlay('second', async () => promiseB);

    await vi.advanceTimersByTimeAsync(200);
    expect(state.visible).toBe(true);

    resolveA!();
    await a;
    expect(state.visible).toBe(true);

    resolveB!();
    await b;
    expect(state.visible).toBe(false);
  });

  it('reflects most-recent text when overlapping calls run', async () => {
    const { state, withOverlay } = useLoadingOverlay();

    let resolveA: (() => void) | undefined;
    let resolveB: (() => void) | undefined;
    const promiseA = new Promise<void>((resolve) => {
      resolveA = resolve;
    });
    const promiseB = new Promise<void>((resolve) => {
      resolveB = resolve;
    });

    const a = withOverlay('first', async () => promiseA);
    await vi.advanceTimersByTimeAsync(200);
    expect(state.text).toBe('first');

    const b = withOverlay('second', async () => promiseB);
    expect(state.text).toBe('second');

    resolveA!();
    resolveB!();
    await a;
    await b;
    expect(state.visible).toBe(false);
    expect(state.text).toBe('');
  });
});

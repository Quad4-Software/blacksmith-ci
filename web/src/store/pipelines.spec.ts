import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it } from 'vitest';

import { usePipelineStore } from './pipelines';

describe('pipelineStore SSE reconnect counter', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('starts at 0', () => {
    const store = usePipelineStore();
    expect(store.sseConnectionId).toBe(0);
  });

  it('increments on each markSseReconnect call', () => {
    const store = usePipelineStore();
    store.markSseReconnect();
    expect(store.sseConnectionId).toBe(1);
    store.markSseReconnect();
    expect(store.sseConnectionId).toBe(2);
  });
});

// Copyright 2025 Crow authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
import { mount } from '@vue/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { nextTick, ref } from 'vue';
import PipelineLog from '../PipelineLog.vue';
import type { Pipeline, PipelineLog as PipelineLogEntry } from '~/lib/api/types';

// `localStorage` is not exposed as a bare global in this environment, but the
// component (and its icon/theme helpers) read from it at import time, so this
// has to run before the imports above (vi.hoisted is lifted to the top).
vi.hoisted(() => {
  const store = new Map<string, string>();
  globalThis.localStorage = {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => store.set(key, String(value)),
    removeItem: (key: string) => store.delete(key),
    clear: () => store.clear(),
    key: (index: number) => [...store.keys()][index] ?? null,
    get length() {
      return store.size;
    },
  };
});

const getLogs = vi.fn();
const streamLogs = vi.fn();

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ getLogs, streamLogs }),
}));

vi.mock('~/compositions/useNotifications', () => ({
  default: () => ({ notify: vi.fn() }),
}));

vi.mock('vue-router', () => ({
  useRoute: () => ({ hash: '' }),
}));

function buildPipeline(state: string): Pipeline {
  return {
    id: 1,
    number: 1,
    workflows: [
      {
        id: 10,
        pid: 1,
        name: 'workflow',
        state,
        attempt: 1,
        children: [
          { id: 100, pid: 1, name: 'step', state, started: 1000, finished: state === 'running' ? undefined : 2000 },
        ],
      },
    ],
  } as unknown as Pipeline;
}

function logEntry(line: number): PipelineLogEntry {
  return { id: line, step_id: 100, time: 0, line, data: 'aGk=', type: 0 };
}

function mountLog(state: string) {
  return mount(PipelineLog, {
    props: { pipeline: buildPipeline(state), stepId: 1 },
    shallow: true,
    global: {
      renderStubDefaultSlot: true,
      provide: {
        repo: ref({ id: 1, owner: 'owner', name: 'repo' }),
        'repo-permissions': ref({ push: true }),
      },
    },
  });
}

describe('pipeline log empty state', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    getLogs.mockReset();
    streamLogs.mockReset();
    streamLogs.mockReturnValue({ close: vi.fn() });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('waits instead of reporting missing logs while the store is still empty', async () => {
    getLogs.mockResolvedValue([]);

    const wrapper = mountLog('success');
    await vi.advanceTimersByTimeAsync(600); // let the debounced flush run

    expect(wrapper.text()).toContain('repo.pipeline.waiting_for_logs');
    expect(wrapper.text()).not.toContain('repo.pipeline.no_logs');

    wrapper.unmount();
  });

  it('picks up logs that are persisted after the view was opened', async () => {
    getLogs.mockResolvedValueOnce([]).mockResolvedValue([logEntry(0)]);

    const wrapper = mountLog('success');
    await vi.advanceTimersByTimeAsync(600);
    expect(getLogs).toHaveBeenCalledTimes(1);

    // first retry of the backoff
    await vi.advanceTimersByTimeAsync(1600);
    await nextTick();

    expect(getLogs).toHaveBeenCalledTimes(2);
    expect(wrapper.text()).not.toContain('repo.pipeline.waiting_for_logs');
    expect(wrapper.text()).not.toContain('repo.pipeline.no_logs');

    wrapper.unmount();
  });

  it('reports missing logs once the retries are exhausted', async () => {
    getLogs.mockResolvedValue([]);

    const wrapper = mountLog('success');
    await vi.advanceTimersByTimeAsync(20000);
    await nextTick();

    expect(wrapper.text()).toContain('repo.pipeline.no_logs');
    expect(wrapper.text()).not.toContain('repo.pipeline.waiting_for_logs');

    wrapper.unmount();
  });

  it('keeps waiting for a running step without falling back to the missing state', async () => {
    getLogs.mockResolvedValue([]);

    const wrapper = mountLog('running');
    await vi.advanceTimersByTimeAsync(20000);
    await nextTick();

    expect(streamLogs).toHaveBeenCalledTimes(1);
    expect(wrapper.text()).toContain('repo.pipeline.waiting_for_logs');
    expect(wrapper.text()).not.toContain('repo.pipeline.no_logs');

    wrapper.unmount();
  });
});

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
import type { Pipeline, PipelineLog as PipelineLogEntry } from '~/lib/api/types';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { nextTick, ref } from 'vue';
import PipelineLog from '../PipelineLog.vue';

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

function logEntry(line: number, text = 'hi', type = 0): PipelineLogEntry {
  return { id: line, step_id: 100, time: 0, line, data: btoa(text), type };
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

describe('pipeline log command folding', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    localStorage.clear();
    getLogs.mockReset();
    streamLogs.mockReset();
    streamLogs.mockReturnValue({ close: vi.fn() });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  const tracedLogs = [
    logEntry(0, 'cloning'),
    logEntry(1, '+ go build ./...'),
    logEntry(2, 'building'),
    logEntry(3, '+ go test ./...'),
    logEntry(4, 'testing'),
  ];

  it('renders compact headers for short commands without repeating their trace lines', async () => {
    getLogs.mockResolvedValue(tracedLogs);

    const wrapper = mountLog('success');
    await vi.advanceTimersByTimeAsync(600);
    await nextTick();

    const headers = wrapper.findAll('[data-log-section]');
    expect(headers).toHaveLength(2);
    expect(headers[0].text()).toContain('go build ./...');
    expect(headers[1].text()).toContain('go test ./...');
    expect(wrapper.findAll('button[data-log-section]')).toHaveLength(0);
    expect(wrapper.text()).not.toContain('+ go build ./...');
    expect(wrapper.text()).toContain('building');

    wrapper.unmount();
  });

  it('numbers command headers so the gutter never skips a line', async () => {
    getLogs.mockResolvedValue(tracedLogs);

    const wrapper = mountLog('success');
    await vi.advanceTimersByTimeAsync(600);
    await nextTick();

    const gutter = wrapper.findAll('a[id^="L"]');
    expect(gutter.map((cell) => cell.text())).toEqual(['1', '2', '3', '4', '5']);
    expect(gutter[1].attributes('id')).toBe('L2');
    expect(gutter[1].attributes('href')).toBe('#L2');

    wrapper.unmount();
  });

  it('lays command headers out on the same subgrid as their output', async () => {
    getLogs.mockResolvedValue(tracedLogs);

    const wrapper = mountLog('success');
    await vi.advanceTimersByTimeAsync(600);
    await nextTick();

    const header = wrapper.findAll('[data-log-section]')[0].element.parentElement!;
    const grid = wrapper.find('.auto-rows-min');
    expect(header.className).toContain('grid-cols-subgrid');
    expect(header.className).toContain('col-span-3');
    expect(grid.classes()).toContain('grid-cols-[3rem_minmax(0,1fr)_min-content]');
    expect(grid.classes()).toContain('md:grid-cols-[4rem_minmax(0,1fr)_min-content]');
    expect(wrapper.findAll('[data-log-section]')[0].classes()).toContain('min-w-0');

    wrapper.unmount();
  });

  it('wraps long command headers instead of truncating them', async () => {
    getLogs.mockResolvedValue([logEntry(0, `+ command ${'argument'.repeat(30)}`), logEntry(1, 'done')]);

    const wrapper = mountLog('success');
    await vi.advanceTimersByTimeAsync(600);
    await nextTick();

    const command = wrapper.find('[data-log-section] > span');
    expect(command.classes()).toContain('whitespace-pre-wrap');
    expect(command.classes()).toContain('wrap-anywhere');
    expect(command.classes()).not.toContain('truncate');

    wrapper.unmount();
  });

  it('hides only the lines of the section that was collapsed', async () => {
    getLogs.mockResolvedValue([
      logEntry(0, 'cloning'),
      logEntry(1, '+ go build ./...'),
      logEntry(2, 'building'),
      logEntry(3, 'compiling'),
      logEntry(4, 'linking'),
      logEntry(5, 'done'),
      logEntry(6, '+ go test ./...'),
      logEntry(7, 'testing'),
    ]);

    const wrapper = mountLog('success');
    await vi.advanceTimersByTimeAsync(600);
    await nextTick();

    await wrapper.findAll('button[data-log-section]')[0].trigger('click');

    expect(wrapper.text()).not.toContain('building');
    expect(wrapper.text()).toContain('testing');
    // the lines before the first command are never folded away
    expect(wrapper.text()).toContain('cloning');
    expect(wrapper.findAll('button[data-log-section]')).toHaveLength(1);
    expect(wrapper.findAll('[data-log-section]')).toHaveLength(2);

    wrapper.unmount();
  });

  it('renders every line ungrouped when folding is turned off', async () => {
    localStorage.setItem('crow:log-fold-commands', 'false');
    getLogs.mockResolvedValue(tracedLogs);

    const wrapper = mountLog('success');
    await vi.advanceTimersByTimeAsync(600);
    await nextTick();

    expect(wrapper.findAll('[data-log-section]')).toHaveLength(0);
    expect(wrapper.text()).toContain('+ go build ./...');

    wrapper.unmount();
  });

  it('reports how long a command ran up to the start of the next one', async () => {
    getLogs.mockResolvedValue([
      { ...logEntry(0, '+ sleep 5'), time: 1 },
      { ...logEntry(1, '+ echo done'), time: 6 },
      { ...logEntry(2, 'done'), time: 6 },
    ]);

    const wrapper = mountLog('success');
    await vi.advanceTimersByTimeAsync(600);
    await nextTick();

    expect(wrapper.findAll('[data-log-section]')[0].text()).toContain('5s');

    wrapper.unmount();
  });
});

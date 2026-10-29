import { flushPromises, mount } from '@vue/test-utils';
import type { ProcLogEntry } from '~/lib/api/types/procLog';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';
import ProcLogViewer from './ProcLogViewer.vue';

// useTheme reads localStorage at import time, which happy-dom does not provide.
vi.mock('~/compositions/useTheme', () => ({
  useTheme: () => ({
    theme: ref('light'),
    storeTheme: ref('auto'),
    themeName: ref('blacksmith'),
    currentTheme: ref({ id: 'blacksmith', name: 'Blacksmith CI', light: {}, dark: {} }),
    availableThemes: ref([]),
  }),
}));

let emit: (entry: ProcLogEntry) => void = () => {};

function fakeEventSource() {
  return {
    addEventListener: vi.fn(),
    close: vi.fn(),
    readyState: 1,
  };
}

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({
    // eslint-disable-next-line promise/prefer-await-to-callbacks
    subscribeAgentLogs: (_id: number, cb: (entry: ProcLogEntry) => void) => {
      emit = cb;
      return fakeEventSource();
    },
    // eslint-disable-next-line promise/prefer-await-to-callbacks
    subscribeAutoscalerLogs: (_id: number, cb: (entry: ProcLogEntry) => void) => {
      emit = cb;
      return fakeEventSource();
    },
  }),
}));

function entry(id: number): ProcLogEntry {
  return {
    id,
    source_type: 'agent',
    source_id: 1,
    time: id * 1_000_000,
    level: 'info',
    line: `line ${id}`,
  };
}

function mountViewer() {
  return mount(ProcLogViewer, {
    props: { source: 'agent' as const, sourceId: 1 },
    global: {
      stubs: {
        Select: true,
        SelectContent: true,
        SelectItem: true,
        SelectTrigger: true,
        SelectValue: true,
      },
    },
  });
}

function rows(wrapper: ReturnType<typeof mountViewer>) {
  return wrapper.findAll('.proclog-row');
}

describe('procLogViewer', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  // The server replays up to 5000 backlog entries on connect, one SSE message
  // each. Rendering per entry meant O(n) full-list renders and one forced
  // layout per message, which froze the browser (#1430).
  it('batches streamed entries into a single render', async () => {
    const wrapper = mountViewer();
    await flushPromises();

    for (let i = 1; i <= 500; i += 1) {
      emit(entry(i));
    }
    await flushPromises();

    expect(rows(wrapper)).toHaveLength(0);

    await vi.advanceTimersByTimeAsync(200);
    await flushPromises();

    expect(rows(wrapper)).toHaveLength(500);
    wrapper.unmount();
  });

  it('drops the oldest entries beyond the render cap', async () => {
    const wrapper = mountViewer();
    await flushPromises();

    for (let i = 1; i <= 5100; i += 1) {
      emit(entry(i));
    }
    await vi.advanceTimersByTimeAsync(200);
    await flushPromises();

    const rendered = rows(wrapper);
    expect(rendered).toHaveLength(5000);
    expect(rendered[0].text()).toContain('line 101');
    wrapper.unmount();
  });

  // EventSource reconnects on its own and the server replays its recent window,
  // so already-seen ids arrive a second time.
  it('ignores entries replayed after a reconnect', async () => {
    const wrapper = mountViewer();
    await flushPromises();

    for (let i = 1; i <= 3; i += 1) {
      emit(entry(i));
    }
    await vi.advanceTimersByTimeAsync(200);
    await flushPromises();
    expect(rows(wrapper)).toHaveLength(3);

    for (let i = 1; i <= 4; i += 1) {
      emit(entry(i));
    }
    await vi.advanceTimersByTimeAsync(200);
    await flushPromises();

    const rendered = rows(wrapper);
    expect(rendered).toHaveLength(4);
    expect(rendered[3].text()).toContain('line 4');
    wrapper.unmount();
  });

  it('ignores entries while paused', async () => {
    const wrapper = mountViewer();
    await flushPromises();

    await wrapper.findAll('button')[0].trigger('click');
    emit(entry(1));
    await vi.advanceTimersByTimeAsync(200);
    await flushPromises();

    expect(rows(wrapper)).toHaveLength(0);
    wrapper.unmount();
  });
});

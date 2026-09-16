import { mount } from '@vue/test-utils';
import type { IssueLogLine } from '~/lib/log-issues';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { defineComponent, h, ref } from 'vue';
import LogIssueBar from './LogIssueBar.vue';

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}));

vi.mock('~/compositions/useNotifications', () => ({
  default: () => ({ notify: vi.fn() }),
}));

// Icon reads the stored theme at module load, which has no window to read from here.
vi.mock('~/compositions/useTheme', () => ({
  useTheme: () => ({ theme: ref('light'), storeTheme: ref('auto') }),
}));

const passThrough = defineComponent({
  setup:
    (_, { slots }) =>
    () =>
      h('div', slots.default?.()),
});

const global = {
  stubs: {
    Icon: true,
    TooltipProvider: passThrough,
    Tooltip: passThrough,
    TooltipTrigger: passThrough,
    TooltipContent: passThrough,
  },
  mocks: {
    $t: (key: string, named?: Record<string, unknown>) => (named ? `${key}:${JSON.stringify(named)}` : key),
  },
};

function lines(...types: IssueLogLine['type'][]): IssueLogLine[] {
  return types.map((type, index) => ({ index, number: index + 1, text: `line ${index + 1}`, type }));
}

function mountBar(...types: IssueLogLine['type'][]) {
  return mount(LogIssueBar, { props: { lines: lines(...types) }, global });
}

type Bar = ReturnType<typeof mountBar>;

/** The severity chips, which are the only buttons in the bar carrying a pressed state. */
function chip(bar: Bar, severity: 'error' | 'warning') {
  return bar.findAll('button[aria-pressed]').find((button) => button.text().includes(severity))!;
}

function listedNumbers(bar: Bar): string[] {
  return bar.findAll('[data-log-issue-number]').map((number) => number.text());
}

describe('logIssueBar', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('renders nothing when the step logged no warning or error', () => {
    const wrapper = mountBar(null, null);

    expect(wrapper.find('[data-log-issues]').exists()).toBe(false);
  });

  it('hides warning entries when the warning chip is switched off', async () => {
    const wrapper = mountBar('error', 'warning', 'error');
    await wrapper.find('[aria-expanded]').trigger('click');
    expect(listedNumbers(wrapper)).toEqual(['1', '2', '3']);

    await chip(wrapper, 'warning').trigger('click');

    expect(listedNumbers(wrapper)).toEqual(['1', '3']);
  });

  it('brings the warnings back when the chip is switched on again', async () => {
    const wrapper = mountBar('error', 'warning');
    await wrapper.find('[aria-expanded]').trigger('click');

    await chip(wrapper, 'warning').trigger('click');
    await chip(wrapper, 'warning').trigger('click');

    expect(listedNumbers(wrapper)).toEqual(['1', '2']);
  });

  it('allows hiding both severities and restoring one', async () => {
    const wrapper = mountBar('error', 'warning');
    await wrapper.find('[aria-expanded]').trigger('click');

    await chip(wrapper, 'warning').trigger('click');
    await chip(wrapper, 'error').trigger('click');

    expect(listedNumbers(wrapper)).toEqual([]);
    expect(wrapper.find('[data-log-issue-next]').exists()).toBe(false);
    expect(JSON.parse(localStorage.getItem('crow:log-issue-severities')!)).toEqual(['warning', 'error']);

    await chip(wrapper, 'warning').trigger('click');
    expect(listedNumbers(wrapper)).toEqual(['2']);
  });

  it('remembers the choice, so "errors only" survives a step switch', async () => {
    const wrapper = mountBar('error', 'warning');

    await chip(wrapper, 'warning').trigger('click');

    expect(JSON.parse(localStorage.getItem('crow:log-issue-severities')!)).toEqual(['warning']);
  });

  it('applies a remembered choice on mount', async () => {
    localStorage.setItem('crow:log-issue-severities', JSON.stringify(['warning']));

    const wrapper = mountBar('error', 'warning');
    await wrapper.find('[aria-expanded]').trigger('click');

    expect(listedNumbers(wrapper)).toEqual(['1']);
  });

  it('tells the log which line to scroll to when an entry is picked', async () => {
    const wrapper = mountBar(null, 'error');
    await wrapper.find('[aria-expanded]').trigger('click');

    await wrapper.findAll('[data-log-issue-entry]')[0].trigger('click');

    expect(wrapper.emitted('jump')).toEqual([[2]]);
  });

  it('skips hidden severities when stepping to the next issue', async () => {
    const wrapper = mountBar('error', 'warning', 'error');
    await chip(wrapper, 'warning').trigger('click');

    await wrapper.find('[data-log-issue-next]').trigger('click');

    expect(wrapper.emitted('jump')).toEqual([[3]]);
  });
});

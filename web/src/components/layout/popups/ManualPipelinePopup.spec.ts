import { flushPromises, mount } from '@vue/test-utils';
import WorkflowSelector from '~/components/form/WorkflowSelector.vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';
import ManualPipelinePopup from './ManualPipelinePopup.vue';

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}));

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

// Icon pulls in useTheme, which reads localStorage at module scope.
vi.mock('~/compositions/useTheme', () => ({
  useTheme: () => ({
    theme: ref('light'),
    storeTheme: ref('auto'),
    themeName: ref('blacksmith'),
    currentTheme: ref({ id: 'blacksmith', name: 'Blacksmith CI', light: {}, dark: {} }),
    availableThemes: ref([]),
  }),
}));

const getRepoBranches = vi.fn();
const getRepoTags = vi.fn();
const getRepoManualWorkflows = vi.fn();
const getRepoManualWorkflowsWithProgress = vi.fn();
const createPipeline = vi.fn();

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ getRepoBranches, getRepoTags, getRepoManualWorkflowsWithProgress, createPipeline }),
}));

vi.mock('~/compositions/useNotifications', () => ({
  default: () => ({ notify: vi.fn() }),
}));

vi.mock('~/compositions/useLoadingOverlay', () => ({
  useLoadingOverlay: () => ({ withOverlay: (_label: string, fn: () => unknown) => fn() }),
}));

vi.mock('~/compositions/useInjectProvide', () => ({
  requiredInject: () => ref({ id: 1, default_branch: 'main' }),
}));

// The picker lives inside a portalled popover, which happy-dom cannot drive
// reliably, so selection is exercised through the component's own bindings.
// `<script setup>` bindings are not part of the public instance type, hence the
// explicit shape here.
interface PickerVm {
  tags: string[];
  refPayload: { branch?: string; tag?: string };
  isSelected: (type: 'branch' | 'tag', name: string) => boolean;
  selectRef: (type: 'branch' | 'tag', name: string) => void;
}

function mountPopup() {
  const wrapper = mount(ManualPipelinePopup, {
    props: { open: true },
    global: {
      stubs: {
        // reka-ui portals the dialog body to document.body, which puts it
        // outside the wrapper's subtree; render it inline instead.
        DialogPortal: { template: '<div><slot /></div>' },
        PopoverPortal: { template: '<div><slot /></div>' },
      },
    },
  });
  return { wrapper, vm: wrapper.vm as unknown as PickerVm };
}

describe('manualPipelinePopup ref picker', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getRepoBranches.mockResolvedValue(['main', 'feature']);
    getRepoTags.mockResolvedValue(['v1.0.0']);
    getRepoManualWorkflows.mockResolvedValue({ workflows: [], warnings: [] });
    getRepoManualWorkflowsWithProgress.mockImplementation((repoId: number, target: { branch?: string; tag?: string }) =>
      getRepoManualWorkflows(repoId, target),
    );
    createPipeline.mockResolvedValue({ number: 1 });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('reveals extended parsing progress after three seconds', async () => {
    vi.useFakeTimers();
    let resolveWorkflows: ((value: { workflows: never[]; warnings: never[] }) => void) | undefined;
    getRepoManualWorkflowsWithProgress.mockImplementation(
      async (_repoId: number, _target: unknown, onProgress: (progress: unknown) => void) => {
        onProgress({ stage: 'parsing', current: 7, total: 20 });
        return new Promise((resolve) => {
          resolveWorkflows = resolve;
        });
      },
    );

    const { wrapper } = mountPopup();
    await flushPromises();

    expect(wrapper.find('[data-slot="progress"]').exists()).toBe(false);
    await vi.advanceTimersByTimeAsync(3000);
    expect(wrapper.find('[data-slot="progress"]').exists()).toBe(true);
    expect(wrapper.text()).toContain('repo.manual_pipeline.parsing_workflows_progress');

    resolveWorkflows!({ workflows: [], warnings: [] });
    await flushPromises();
    expect(wrapper.find('[data-slot="progress"]').exists()).toBe(false);
    wrapper.unmount();
  });

  it('starts on the default branch and asks for its workflows by branch', async () => {
    mountPopup();
    await flushPromises();

    expect(getRepoManualWorkflows).toHaveBeenCalledWith(1, { branch: 'main' });
  });

  it('fetches branches and tags together', async () => {
    mountPopup();
    await flushPromises();

    expect(getRepoBranches).toHaveBeenCalled();
    expect(getRepoTags).toHaveBeenCalled();
  });

  // A forge without tag support answers 501. That must not break the dialog,
  // which still has to work for branches.
  it('stays usable when the forge cannot list tags', async () => {
    getRepoTags.mockRejectedValue(new Error('501 not implemented'));

    const { vm } = mountPopup();
    await flushPromises();

    expect(vm.tags).toEqual([]);
    expect(getRepoManualWorkflows).toHaveBeenCalledWith(1, { branch: 'main' });
  });

  it('requests workflows by tag once a tag is selected', async () => {
    const { vm } = mountPopup();
    await flushPromises();
    getRepoManualWorkflows.mockClear();

    vm.selectRef('tag', 'v1.0.0');
    await flushPromises();

    expect(getRepoManualWorkflows).toHaveBeenCalledWith(1, { tag: 'v1.0.0' });
  });

  // The two are mutually exclusive server-side, so the payload must never
  // carry both keys.
  it('builds a tag-only payload for a tag', async () => {
    const { vm } = mountPopup();
    await flushPromises();

    vm.selectRef('tag', 'v1.0.0');
    await flushPromises();

    expect(vm.refPayload).toEqual({ tag: 'v1.0.0' });
    expect(vm.refPayload).not.toHaveProperty('branch');
  });

  it('builds a branch-only payload for a branch', async () => {
    const { vm } = mountPopup();
    await flushPromises();

    vm.selectRef('branch', 'feature');
    await flushPromises();

    expect(vm.refPayload).toEqual({ branch: 'feature' });
    expect(vm.refPayload).not.toHaveProperty('tag');
  });

  // End-to-end through the child component, so the payload shape above is
  // proven to reach the API call rather than just being computed correctly.
  it('creates a pipeline against the selected tag on quick-start', async () => {
    getRepoManualWorkflows.mockResolvedValue({ workflows: [{ name: 'deploy', variables: [] }], warnings: [] });

    const { wrapper, vm } = mountPopup();
    await flushPromises();

    vm.selectRef('tag', 'v1.0.0');
    await flushPromises();

    wrapper.findComponent(WorkflowSelector).vm.$emit('quick-start', 'deploy');
    await flushPromises();

    expect(createPipeline).toHaveBeenCalledWith(1, {
      tag: 'v1.0.0',
      variables: {},
      workflows: ['deploy'],
    });
  });

  // A branch and a tag may share a name, so identity has to include the kind.
  it('distinguishes a branch from a tag of the same name', async () => {
    getRepoBranches.mockResolvedValue(['main', 'v1.0.0']);

    const { vm } = mountPopup();
    await flushPromises();

    vm.selectRef('tag', 'v1.0.0');
    await flushPromises();

    expect(vm.isSelected('tag', 'v1.0.0')).toBe(true);
    expect(vm.isSelected('branch', 'v1.0.0')).toBe(false);
  });
});

import { describe, it, expect, vi } from 'vitest';
import { ref } from 'vue';
import { mount } from '@vue/test-utils';
import ManualVariablesPanel from './ManualVariablesPanel.vue';
import { groupManualVariables } from './manualVariableGroups';
import type { ManualWorkflow } from '~/lib/api/types';

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}));

vi.mock('~/compositions/useTheme', () => ({
  useTheme: () => ({
    theme: ref('light'),
    storeTheme: ref('auto'),
    themeName: ref('blacksmith'),
    currentTheme: ref({ id: 'blacksmith', name: 'Blacksmith CI', light: {}, dark: {} }),
    availableThemes: ref([]),
  }),
}));

function wf(name: string, variables: ManualWorkflow['variables']): ManualWorkflow {
  return { name, pipeline_name: name, has_manual: true, variables };
}

const stubs = {
  VariableField: {
    props: ['variable', 'modelValue', 'sources'],
    emits: ['update:modelValue'],
    template:
      '<button type="button" class="vf" :data-name="variable.name" :data-sources="(sources || []).join(\',\')" @click="$emit(\'update:modelValue\', \'y\')"></button>',
  },
};

function mountPanel(workflows: ManualWorkflow[], selected: string[], hasSelection: boolean) {
  return mount(ManualVariablesPanel, {
    props: {
      grouped: groupManualVariables(workflows, new Set(selected)),
      modelValue: {},
      hasSelection,
    },
    global: { stubs },
  });
}

describe('manualVariablesPanel', () => {
  it('shows the no-selection hint when nothing is selected', () => {
    const w = mountPanel([wf('build', [{ name: 'A' }])], [], false);
    expect(w.text()).toContain('repo.manual_pipeline.variables.empty_no_selection');
    expect(w.findAll('.vf')).toHaveLength(0);
  });

  it('shows the no-variables hint when selected workflows declare none', () => {
    const w = mountPanel([wf('build', [])], ['build'], true);
    expect(w.text()).toContain('repo.manual_pipeline.variables.empty_no_variables');
  });

  it('renders a Shared section plus per-workflow sections with a field each', () => {
    const workflows = [
      wf('build', [{ name: 'DEPLOY_ENV' }, { name: 'LOG_LEVEL' }]),
      wf('deploy', [{ name: 'DEPLOY_ENV' }, { name: 'REPLICAS' }]),
    ];
    const w = mountPanel(workflows, ['build', 'deploy'], true);
    expect(w.text()).toContain('repo.manual_pipeline.variables.shared');
    expect(w.text()).toContain('build');
    expect(w.text()).toContain('deploy');
    const fields = w.findAll('.vf');
    expect(fields).toHaveLength(3);
    const shared = fields.find((f) => f.attributes('data-name') === 'DEPLOY_ENV')!;
    expect(shared.attributes('data-sources')!.split(',')).toEqual(expect.arrayContaining(['build', 'deploy']));
  });

  it('emits update:modelValue merged with the existing values, preserving other keys', async () => {
    const workflows = [wf('build', [{ name: 'A' }])];
    const w = mount(ManualVariablesPanel, {
      props: {
        grouped: groupManualVariables(workflows, new Set(['build'])),
        modelValue: { A: 'x', B: 'prev' },
        hasSelection: true,
      },
      global: { stubs },
    });
    await w.find('.vf').trigger('click');
    const emitted = w.emitted('update:modelValue');
    expect(emitted?.at(-1)?.[0]).toEqual({ A: 'y', B: 'prev' });
  });
});

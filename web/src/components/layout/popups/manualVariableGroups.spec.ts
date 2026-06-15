import { describe, it, expect } from 'vitest';
import { groupManualVariables } from './manualVariableGroups';
import type { ManualWorkflow } from '~/lib/api/types';

function wf(name: string, variables: ManualWorkflow['variables']): ManualWorkflow {
  return { name, pipeline_name: name, has_manual: true, variables };
}

describe('groupManualVariables', () => {
  it('puts variables declared by 2+ selected workflows in the shared set, merged', () => {
    const workflows = [
      wf('build', [
        { name: 'DEPLOY_ENV', options: ['staging', 'prod'], default: 'staging', required: true },
        { name: 'LOG_LEVEL', options: ['info'] },
      ]),
      wf('deploy', [{ name: 'DEPLOY_ENV', options: ['prod'] }, { name: 'REPLICAS' }]),
    ];
    const g = groupManualVariables(workflows, new Set(['build', 'deploy']));
    expect(g.shared.map((v) => v.name)).toEqual(['DEPLOY_ENV']);
    expect(g.sharedSources.DEPLOY_ENV).toEqual(['build', 'deploy']);
    expect(g.shared[0].options).toEqual(['staging', 'prod']);
    expect(g.shared[0].default).toBe('staging');
    expect(g.shared[0].required).toBe(true);
  });

  it('deduplicates options when merging shared variables', () => {
    const workflows = [
      wf('build', [{ name: 'TIER', options: ['free', 'pro'] }]),
      wf('deploy', [{ name: 'TIER', options: ['pro', 'enterprise'] }]),
    ];
    const g = groupManualVariables(workflows, new Set(['build', 'deploy']));
    expect(g.shared[0].options).toEqual(['free', 'pro', 'enterprise']);
  });

  it('excludes shared names from per-workflow groups and omits empty groups', () => {
    const workflows = [
      wf('build', [{ name: 'DEPLOY_ENV' }, { name: 'LOG_LEVEL' }]),
      wf('deploy', [{ name: 'DEPLOY_ENV' }]),
    ];
    const g = groupManualVariables(workflows, new Set(['build', 'deploy']));
    expect(g.groups.map((gr) => gr.workflow)).toEqual(['build']);
    expect(g.groups[0].variables.map((v) => v.name)).toEqual(['LOG_LEVEL']);
  });

  it('only counts selected workflows', () => {
    const workflows = [wf('build', [{ name: 'DEPLOY_ENV' }]), wf('deploy', [{ name: 'DEPLOY_ENV' }])];
    const g = groupManualVariables(workflows, new Set(['build']));
    expect(g.shared).toEqual([]);
    expect(g.groups).toEqual([{ workflow: 'build', variables: [{ name: 'DEPLOY_ENV' }] }]);
  });

  it('returns a deduped union in `all`', () => {
    const workflows = [
      wf('build', [{ name: 'A' }, { name: 'SHARED' }]),
      wf('deploy', [{ name: 'B' }, { name: 'SHARED' }]),
    ];
    const g = groupManualVariables(workflows, new Set(['build', 'deploy']));
    expect(g.all.map((v) => v.name).toSorted()).toEqual(['A', 'B', 'SHARED']);
  });

  it('merges shared vars first-non-empty: empty default/description falls through', () => {
    const workflows = [
      wf('build', [{ name: 'X', default: '', description: '' }]),
      wf('deploy', [{ name: 'X', default: 'prod', description: 'env' }]),
    ];
    const g = groupManualVariables(workflows, new Set(['build', 'deploy']));
    expect(g.shared).toHaveLength(1);
    expect(g.shared[0].default).toBe('prod');
    expect(g.shared[0].description).toBe('env');
  });

  it('empty selection yields empty result', () => {
    const g = groupManualVariables([wf('build', [{ name: 'A' }])], new Set());
    expect(g.all).toEqual([]);
    expect(g.groups).toEqual([]);
    expect(g.shared).toEqual([]);
  });
});

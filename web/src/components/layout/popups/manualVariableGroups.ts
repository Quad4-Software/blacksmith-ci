import type { ManualVariable, ManualWorkflow } from '~/lib/api/types';

export interface VariableGroup {
  workflow: string;
  variables: ManualVariable[];
}

export interface GroupedManualVariables {
  shared: ManualVariable[];
  sharedSources: Record<string, string[]>;
  groups: VariableGroup[];
  all: ManualVariable[];
}

export function getSelectedWorkflowDefault(variable: ManualVariable): string {
  // The UI explicitly filters to selected workflows, so an all-capable gate
  // can start safely while its YAML default remains none for unfiltered calls.
  if (variable.default === 'none' && variable.options?.includes('all')) {
    return 'all';
  }
  return variable.default ?? '';
}

function mergeShared(a: ManualVariable, b: ManualVariable): ManualVariable {
  const options = [...new Set([...(a.options ?? []), ...(b.options ?? [])])];
  return {
    name: a.name,
    // First non-empty wins: `?? ''` collapses undefined, `||` then skips empty strings.
    description: (a.description ?? '') || (b.description ?? ''),
    default: (a.default ?? '') || (b.default ?? ''),
    required: a.required || b.required,
    options: options.length > 0 ? options : undefined,
  };
}

export function groupManualVariables(workflows: ManualWorkflow[], selected: Set<string>): GroupedManualVariables {
  const selectedWorkflows = workflows.filter((w) => selected.has(w.name));

  const sources: Record<string, string[]> = {};
  for (const workflow of selectedWorkflows) {
    for (const variable of workflow.variables ?? []) {
      (sources[variable.name] ??= []).push(workflow.name);
    }
  }
  const sharedNames = new Set(Object.keys(sources).filter((name) => sources[name].length >= 2));

  const sharedByName = new Map<string, ManualVariable>();
  for (const workflow of selectedWorkflows) {
    for (const variable of workflow.variables ?? []) {
      if (!sharedNames.has(variable.name)) continue;
      const existing = sharedByName.get(variable.name);
      sharedByName.set(
        variable.name,
        existing
          ? mergeShared(existing, variable)
          : { ...variable, options: variable.options ? [...variable.options] : undefined },
      );
    }
  }
  const shared = [...sharedByName.values()].toSorted((a, b) => a.name.localeCompare(b.name));

  const groups: VariableGroup[] = [];
  for (const workflow of selectedWorkflows) {
    const variables = (workflow.variables ?? []).filter((v) => !sharedNames.has(v.name));
    if (variables.length > 0) groups.push({ workflow: workflow.name, variables });
  }

  const sharedSources: Record<string, string[]> = {};
  for (const name of sharedNames) sharedSources[name] = sources[name];

  const all = [...shared, ...groups.flatMap((g) => g.variables)];

  return { shared, sharedSources, groups, all };
}

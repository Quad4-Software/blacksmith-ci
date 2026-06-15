<template>
  <div>
    <div class="text-xs uppercase tracking-wide text-muted-foreground mb-3">
      {{ $t('repo.manual_pipeline.variables.heading') }}
    </div>

    <p v-if="!hasSelection" class="text-sm text-muted-foreground italic py-8 text-center">
      {{ $t('repo.manual_pipeline.variables.empty_no_selection') }}
    </p>
    <p v-else-if="grouped.all.length === 0" class="text-sm text-muted-foreground italic py-8 text-center">
      {{ $t('repo.manual_pipeline.variables.empty_no_variables') }}
    </p>

    <div v-else class="space-y-4">
      <div v-for="(section, index) in sections" :key="section.key">
        <hr
          v-if="index > 0"
          class="border-border mb-4"
          :class="{ 'border-t-2': index === 1 && grouped.shared.length > 0 }"
        />
        <div v-if="section.shared" class="text-sm font-semibold italic mb-2">
          {{ $t('repo.manual_pipeline.variables.shared') }}
        </div>
        <div v-else class="text-sm italic text-muted-foreground mb-2">{{ section.label }}</div>
        <div class="space-y-3">
          <VariableField
            v-for="variable in section.variables"
            :key="variable.name"
            :variable="variable"
            :sources="section.shared ? grouped.sharedSources[variable.name] : undefined"
            :model-value="modelValue[variable.name] ?? ''"
            @update:model-value="(val) => update(variable.name, val)"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue';

import VariableField from '~/components/layout/popups/VariableField.vue';
import type { GroupedManualVariables, VariableGroup } from '~/components/layout/popups/manualVariableGroups';

const props = defineProps<{
  grouped: GroupedManualVariables;
  modelValue: Record<string, string>;
  hasSelection: boolean;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, string>): void;
}>();

interface Section {
  key: string;
  label: string;
  shared: boolean;
  variables: VariableGroup['variables'];
}

const sections = computed<Section[]>(() => {
  const out: Section[] = [];
  if (props.grouped.shared.length > 0) {
    out.push({ key: '__shared__', label: '', shared: true, variables: props.grouped.shared });
  }
  for (const group of props.grouped.groups) {
    out.push({ key: group.workflow, label: group.workflow, shared: false, variables: group.variables });
  }
  return out;
});

function update(name: string, value: string) {
  emit('update:modelValue', { ...props.modelValue, [name]: value });
}
</script>

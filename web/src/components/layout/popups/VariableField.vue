<template>
  <div class="space-y-1">
    <label :for="`var-${variable.name}`" class="text-sm font-medium">
      {{ variable.name }}
      <span v-if="variable.required" class="text-destructive">{{ $t('repo.manual_pipeline.variables.required') }}</span>
    </label>
    <p v-if="sources && sources.length > 0" class="text-xs text-muted-foreground">
      {{ $t('repo.manual_pipeline.variables.used_by', { workflows: sources.join(', ') }) }}
    </p>
    <p v-if="variable.description" class="text-xs text-muted-foreground">{{ variable.description }}</p>
    <Select
      v-if="variable.options && variable.options.length > 0"
      :model-value="modelValue"
      @update:model-value="(val) => $emit('update:modelValue', String(val))"
    >
      <SelectTrigger :id="`var-${variable.name}`" class="h-8 w-full">
        <SelectValue :placeholder="$t('repo.manual_pipeline.variables.select')" />
      </SelectTrigger>
      <SelectContent>
        <SelectItem v-for="option in variable.options" :key="option" :value="option">
          {{ option }}
        </SelectItem>
      </SelectContent>
    </Select>
    <Input
      v-else
      :id="`var-${variable.name}`"
      :model-value="modelValue"
      :placeholder="$t('repo.manual_pipeline.variables.value')"
      @update:model-value="(val) => $emit('update:modelValue', String(val))"
    />
  </div>
</template>

<script lang="ts" setup>
import type { ManualVariable } from '~/lib/api/types';
import { Input } from '~/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';

defineProps<{
  variable: ManualVariable;
  modelValue: string;
  sources?: string[];
}>();

defineEmits<{
  (e: 'update:modelValue', value: string): void;
}>();
</script>

<template>
  <div class="flex flex-col gap-2">
    <!-- Header with select all -->
    <div v-if="workflows.length > 0" class="flex items-center justify-between text-sm text-muted-foreground pb-2">
      <label class="flex items-center gap-2 cursor-pointer">
        <Checkbox
          :checked="allSelected ? true : someSelected ? 'indeterminate' : false"
          @update:checked="toggleSelectAll"
        />
        <span>{{ $t('repo.manual_pipeline.select_all') }}</span>
      </label>
      <span>{{
        $t('repo.manual_pipeline.selected_count', { selected: selectedWorkflows.size, total: workflows.length })
      }}</span>
    </div>

    <!-- Workflow list -->
    <div v-if="workflows.length > 0" class="flex flex-col gap-1">
      <div
        v-for="workflow in workflows"
        :key="workflow.name"
        class="flex items-center gap-3 p-3 rounded-md border bg-card hover:bg-accent transition-colors"
        :class="isSelected(workflow.name) ? 'border-primary bg-primary/5' : 'border-border'"
      >
        <!-- Checkbox -->
        <Checkbox
          :id="`workflow-${workflow.name}`"
          :checked="isSelected(workflow.name)"
          @update:checked="toggleWorkflow(workflow.name)"
        />

        <!-- Workflow info -->
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <label :for="`workflow-${workflow.name}`" class="font-medium cursor-pointer truncate">
              {{ workflow.name }}
            </label>
            <Badge v-if="workflow.depends_on?.length" variant="outline" class="text-xs">
              {{ $t('repo.manual_pipeline.depends_on', { count: workflow.depends_on.length }) }}
            </Badge>
          </div>
          <div v-if="workflow.depends_on?.length" class="text-xs text-muted-foreground mt-1 truncate">
            {{ workflow.depends_on.join(', ') }}
          </div>
        </div>

        <!-- Quick start button -->
        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger as-child>
              <Button type="button" variant="ghost" size="icon-sm" @click.stop="$emit('quickStart', workflow.name)">
                <Icon name="play" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{{ $t('repo.manual_pipeline.quick_start') }}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>
    </div>

    <!-- Empty state -->
    <div v-else class="text-center py-8 text-muted-foreground">
      <Icon name="alert" class="w-8 h-8 mx-auto mb-2 opacity-50" />
      <p>{{ $t('repo.manual_pipeline.no_workflows') }}</p>
    </div>

    <!-- Warnings -->
    <div v-if="warnings && warnings.length > 0" class="mt-2">
      <div
        v-for="(warning, index) in warnings"
        :key="index"
        class="flex items-start gap-2 p-2 text-sm text-warning-foreground bg-warning/10 rounded-md"
      >
        <Icon name="alert" class="w-4 h-4 shrink-0 mt-0.5" />
        <span>{{ warning }}</span>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import type { ManualWorkflow } from '~/lib/api/types';
import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import { Badge } from '~/components/ui/badge';
import { Checkbox } from '~/components/ui/checkbox';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';

const props = defineProps<{
  workflows: ManualWorkflow[];
  selectedWorkflows: Set<string>;
  warnings?: string[];
}>();

const emit = defineEmits<{
  (e: 'update:selectedWorkflows', value: Set<string>): void;
  (e: 'quickStart', workflowName: string): void;
}>();

const allSelected = computed(() => {
  return props.workflows.length > 0 && props.workflows.every((w) => props.selectedWorkflows.has(w.name));
});

const someSelected = computed(() => {
  return props.workflows.some((w) => props.selectedWorkflows.has(w.name));
});

function isSelected(name: string): boolean {
  return props.selectedWorkflows.has(name);
}

function toggleWorkflow(name: string) {
  const newSet = new Set(props.selectedWorkflows);
  if (newSet.has(name)) {
    newSet.delete(name);
  } else {
    newSet.add(name);
  }
  emit('update:selectedWorkflows', newSet);
}

function toggleSelectAll() {
  if (allSelected.value) {
    emit('update:selectedWorkflows', new Set());
  } else {
    emit('update:selectedWorkflows', new Set(props.workflows.map((w) => w.name)));
  }
}
</script>

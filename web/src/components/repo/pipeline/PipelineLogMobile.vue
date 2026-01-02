<template>
  <div class="flex flex-col h-full">
    <!-- Collapsible Step Info Panel -->
    <Collapsible v-model:open="stepInfoExpanded" class="border border-border rounded-md shrink-0">
      <div class="bg-card rounded-md">
        <!-- Collapsible Header (always visible) -->
        <CollapsibleTrigger
          class="w-full flex items-center justify-between p-3 hover:bg-accent transition-colors rounded-md"
        >
          <div class="flex items-center gap-3 min-w-0 flex-1">
            <PipelineStatusIcon :status="currentStep?.state || 'pending'" class="h-5 w-5 shrink-0" />
            <div class="min-w-0 flex-1">
              <div class="font-medium text-foreground truncate">{{ currentStep?.name }}</div>
              <div class="text-sm text-muted-foreground truncate">{{ currentWorkflow?.name }}</div>
            </div>
          </div>
          <Icon
            name="chevron-down"
            class="h-5 w-5 transition-transform duration-200 shrink-0"
            :class="{ 'rotate-180': stepInfoExpanded }"
          />
        </CollapsibleTrigger>

        <!-- Collapsible Content (step details) -->
        <CollapsibleContent>
          <div class="px-3 pb-3 pt-2 space-y-2 border-t border-border">
            <!-- Exit code -->
            <div v-if="currentStep?.exit_code !== undefined" class="flex items-center gap-2">
              <span class="text-sm text-muted-foreground">{{ $t('repo.pipeline.exit_code_label') }}</span>
              <span
                class="text-sm border rounded-full px-2 py-0.5"
                :class="
                  currentStep.exit_code === 0
                    ? 'border-green-500 text-green-700 dark:text-green-400'
                    : 'border-destructive text-destructive'
                "
              >
                {{ currentStep.exit_code }}
              </span>
            </div>
            <!-- Agent info -->
            <div v-if="agentDisplay" class="flex items-center gap-2">
              <span class="text-sm text-muted-foreground">{{ $t('repo.pipeline.agent_label') }}</span>
              <a
                :href="repoPermissions?.admin ? '/admin/agents/' : '/user/agents/'"
                class="text-sm text-primary hover:text-primary/80"
              >
                {{ agentDisplay }}
              </a>
            </div>
            <!-- Duration -->
            <div v-if="currentStep?.started" class="flex items-center gap-2">
              <span class="text-sm text-muted-foreground">{{ $t('repo.pipeline.duration_label') }}</span>
              <PipelineStepDuration :step="currentStep" class="text-sm" />
            </div>
          </div>
        </CollapsibleContent>
      </div>
    </Collapsible>

    <!-- Log Viewer (fills remaining space) -->
    <div class="flex-1 min-h-0 relative mt-2" @touchstart.passive="handleTouchStart" @touchend.passive="handleTouchEnd">
      <PipelineLog
        v-model:step-id="selectedStepIdLocal"
        :pipeline="pipeline"
        :previous-step-id="previousStepId"
        :next-step-id="nextStepId"
        :auto-follow-enabled="autoFollowEnabled"
        :hide-timestamps="true"
        class="h-full"
        @update:auto-follow-enabled="$emit('update:auto-follow-enabled', $event)"
      />
    </div>

    <!-- Navigation Bar (bottom) -->
    <div class="flex items-center justify-between gap-2 p-2 border border-border rounded-md bg-card shrink-0 mt-2">
      <!-- Previous Step Button -->
      <Button variant="outline" size="sm" :disabled="!previousStepId" @click="navigateStep(previousStepId ?? null)">
        <Icon name="back" class="h-4 w-4 mr-1" />
        {{ $t('repo.pipeline.previous_step_short') }}
      </Button>

      <!-- Step Navigation Button (opens bottom sheet) -->
      <Button variant="outline" size="sm" @click="bottomSheetOpen = true">
        <Icon name="list" class="h-4 w-4 mr-1" />
        {{ $t('repo.pipeline.all_steps') }}
      </Button>

      <!-- Next Step Button -->
      <Button variant="outline" size="sm" :disabled="!nextStepId" @click="navigateStep(nextStepId ?? null)">
        {{ $t('repo.pipeline.next_step_short') }}
        <Icon name="next" class="h-4 w-4 ml-1" />
      </Button>
    </div>

    <!-- Bottom Sheet: Step Navigation -->
    <Sheet v-model:open="bottomSheetOpen">
      <SheetContent side="bottom" class="h-[70vh] flex flex-col p-4 pt-2">
        <SheetHeader class="pb-2">
          <SheetTitle>{{ $t('repo.pipeline.steps_sheet_title') }}</SheetTitle>
          <SheetDescription>{{ $t('repo.pipeline.select_step') }}</SheetDescription>
        </SheetHeader>
        <div class="flex-1 min-h-0 overflow-hidden">
          <ScrollArea class="h-full">
            <div class="flex flex-col gap-y-2 pb-4">
              <div
                v-for="workflow in pipeline.workflows"
                :key="workflow.id"
                class="rounded-md border border-border bg-card p-2 shadow-xs"
              >
                <!-- Workflow Header with Chevron -->
                <button
                  class="flex items-center gap-2 mb-2 w-full text-left hover:opacity-80 transition-opacity"
                  @click="toggleWorkflow(workflow.id)"
                >
                  <Icon
                    name="chevron-right"
                    class="size-4 transition-transform duration-200 text-muted-foreground"
                    :class="{ 'rotate-90': !workflowsCollapsed[workflow.id] }"
                  />
                  <div class="flex items-center gap-3">
                    <PipelineStatusIcon :status="workflow.state" class="h-4 w-4" />
                    <span class="font-medium text-foreground">{{ workflow.name }}</span>
                  </div>
                </button>
                <!-- Steps -->
                <div v-show="!workflowsCollapsed[workflow.id]" class="space-y-1">
                  <div
                    v-for="step in workflow.children"
                    :key="step.pid"
                    class="flex items-center gap-3 p-2 rounded transition-colors hover:bg-accent cursor-pointer ml-4"
                    :class="{ 'bg-accent': step.pid === stepId }"
                    @click="handleStepSelect(step.pid)"
                  >
                    <PipelineStatusIcon :service="step.type === 'service'" :status="step.state" class="h-4 w-4" />
                    <span class="flex-1 text-sm text-foreground">{{ step.name }}</span>
                    <PipelineStepDuration :step="step" class="text-xs text-muted-foreground" />
                  </div>
                </div>
              </div>
            </div>
          </ScrollArea>
        </div>
      </SheetContent>
    </Sheet>
  </div>
</template>

<script lang="ts" setup>
import { computed, ref, watch, onMounted } from 'vue';
import { useStorage } from '@vueuse/core';

import PipelineLog from './PipelineLog.vue';
import PipelineStatusIcon from './PipelineStatusIcon.vue';
import PipelineStepDuration from './PipelineStepDuration.vue';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '~/components/ui/sheet';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '~/components/ui/collapsible';
import { Button } from '~/components/ui/button';
import { ScrollArea } from '~/components/ui/scroll-area';
import Icon from '~/components/atomic/Icon.vue';
import { requiredInject } from '~/compositions/useInjectProvide';
import useApiClient from '~/compositions/useApiClient';
import type { Pipeline } from '~/lib/api/types';

const props = withDefaults(
  defineProps<{
    pipeline: Pipeline;
    stepId: number;
    previousStepId?: number | null;
    nextStepId?: number | null;
    autoFollowEnabled?: boolean;
  }>(),
  {
    autoFollowEnabled: false,
  },
);

const emit = defineEmits<{
  (event: 'update:step-id', stepId: number | null): void;
  (event: 'update:auto-follow-enabled', value: boolean): void;
}>();

const apiClient = useApiClient();
const repoPermissions = requiredInject('repo-permissions');

// Persistent state
const stepInfoExpanded = useStorage('crow:mobile-step-info-expanded', false);

// Transient state
const bottomSheetOpen = ref(false);
const touchStartX = ref(0);
const touchStartY = ref(0);
const workflowsCollapsed = ref<Record<number, boolean>>({});
const agentDetails = ref<{ name?: string } | null>(null);

// Computed properties
const selectedStepIdLocal = computed({
  get: () => props.stepId,
  set: (val) => emit('update:step-id', val),
});

const currentStep = computed(() => {
  if (!props.stepId || !props.pipeline?.workflows) return null;

  for (const workflow of props.pipeline.workflows) {
    const step = workflow.children?.find((s) => s.pid === props.stepId);
    if (step) return step;
  }
  return null;
});

const currentWorkflow = computed(() => {
  if (!props.stepId || !props.pipeline?.workflows) return null;

  for (const workflow of props.pipeline.workflows) {
    const step = workflow.children?.find((s) => s.pid === props.stepId);
    if (step) return workflow;
  }
  return null;
});

const agentDisplay = computed(() => {
  const agentId = currentWorkflow.value?.agent_id;
  if (!agentId) return '';

  const agentName = agentDetails.value?.name;
  if (agentName) {
    return agentName;
  }
  return agentId.toString();
});

// Methods
function navigateStep(stepId: number | null) {
  if (stepId !== null) {
    emit('update:step-id', stepId);
  }
}

function handleStepSelect(stepId: number) {
  emit('update:step-id', stepId);
  bottomSheetOpen.value = false;
}

function toggleWorkflow(workflowId: number) {
  workflowsCollapsed.value[workflowId] = !workflowsCollapsed.value[workflowId];
}

function handleTouchStart(event: TouchEvent) {
  touchStartX.value = event.touches[0].clientX;
  touchStartY.value = event.touches[0].clientY;
}

function handleTouchEnd(event: TouchEvent) {
  const touchEndX = event.changedTouches[0].clientX;
  const touchEndY = event.changedTouches[0].clientY;

  const deltaX = touchEndX - touchStartX.value;
  const deltaY = touchEndY - touchStartY.value;

  const minSwipeDistance = 50;

  // Check if it's a horizontal swipe (more horizontal than vertical)
  if (Math.abs(deltaX) > Math.abs(deltaY) && Math.abs(deltaX) > minSwipeDistance) {
    if (deltaX > 0 && props.previousStepId) {
      navigateStep(props.previousStepId);
    } else if (deltaX < 0 && props.nextStepId) {
      navigateStep(props.nextStepId);
    }
  }
}

// Load agent details when workflow changes
async function loadAgentDetails() {
  const agentId = currentWorkflow.value?.agent_id;
  if (!agentId) {
    agentDetails.value = null;
    return;
  }

  try {
    agentDetails.value = await apiClient.getAgent(agentId);
  } catch (error) {
    console.warn(`Failed to load agent ${agentId}:`, error);
    agentDetails.value = null;
  }
}

// Watch for workflow changes
watch(currentWorkflow, async () => {
  await loadAgentDetails();
});

onMounted(async () => {
  await loadAgentDetails();
});
</script>

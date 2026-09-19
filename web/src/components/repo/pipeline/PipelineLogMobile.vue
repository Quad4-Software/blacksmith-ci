<template>
  <div class="flex flex-col min-h-0 overflow-hidden">
    <!-- Collapsible Step Info Panel -->
    <Collapsible v-model:open="stepInfoExpanded" class="border border-border rounded-md shrink-0">
      <div class="bg-card rounded-md">
        <!-- Collapsible Header (always visible) -->
        <div class="flex items-center gap-2 p-3">
          <div class="flex items-center gap-2 min-w-0 flex-1">
            <PipelineStatusIcon :status="currentStep?.state || 'pending'" class="h-5 w-5 shrink-0" />
            <div class="min-w-0 flex-1 text-left">
              <div class="font-medium text-foreground truncate text-sm">{{ currentStep?.name }}</div>
              <div class="text-xs text-muted-foreground truncate">{{ currentWorkflow?.name }}</div>
            </div>
          </div>
          <CollapsibleTrigger
            class="shrink-0 inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
          >
            {{ $t('repo.pipeline.details') }}
            <Icon
              name="chevron-down"
              class="h-3.5 w-3.5 transition-transform duration-200"
              :class="{ 'rotate-180': stepInfoExpanded }"
            />
          </CollapsibleTrigger>
          <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
          <DropdownMenu v-if="hasMultipleAttempts && currentWorkflow">
            <DropdownMenuTrigger as-child>
              <button
                class="shrink-0 inline-flex items-center gap-0.5 rounded border border-primary/70 px-1 py-0.5 text-[10px] text-muted-foreground hover:bg-accent transition-colors"
              >
                #{{ currentWorkflow.attempt }}
                <Icon name="chevron-down" class="h-2.5 w-2.5" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                v-for="attempt in currentWorkflowAttempts"
                :key="attempt.id"
                class="flex items-center gap-2 text-xs"
                @click="switchToAttempt(attempt)"
              >
                <PipelineStatusIcon :status="attempt.state" class="h-3.5 w-3.5" />
                #{{ attempt.attempt }}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
          <IconButton
            v-if="hasPushPermission && currentWorkflow && isWorkflowCompleted"
            :is-loading="isRerunningWorkflow[currentWorkflow.id]"
            icon="solar:restart-bold"
            class="shrink-0 text-primary hover:text-primary/80 hover:bg-accent"
            :title="$t('repo.pipeline.actions.rerun_workflow')"
            @click="handleRerunWorkflow(currentWorkflow.id)"
          />
        </div>

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
    <div
      class="flex flex-col flex-1 min-h-0 relative mt-2"
      @touchstart.passive="handleTouchStart"
      @touchend.passive="handleTouchEnd"
    >
      <PipelineLog
        v-model:step-id="selectedStepIdLocal"
        :pipeline="pipeline"
        :previous-step-id="previousStepId"
        :next-step-id="nextStepId"
        :auto-follow-enabled="autoFollowEnabled"
        :hide-timestamps="true"
        class="flex-1 min-h-0"
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
                v-for="workflow in displayedWorkflows"
                :key="workflow.id"
                class="rounded-md border border-border bg-card p-2 shadow-xs"
              >
                <!-- Workflow Header with Chevron -->
                <div class="flex items-center gap-2 mb-2 w-full">
                  <button
                    class="flex items-center gap-2 flex-1 min-w-0 text-left hover:opacity-80 transition-opacity"
                    @click="toggleWorkflow(workflow.id)"
                  >
                    <Icon
                      name="chevron-right"
                      class="size-4 transition-transform duration-200 text-muted-foreground shrink-0"
                      :class="{ 'rotate-90': !workflowsCollapsed[workflow.id] }"
                    />
                    <PipelineStatusIcon :status="workflow.state" class="h-4 w-4 shrink-0" />
                    <span class="font-medium text-foreground truncate">{{ workflow.name }}</span>
                  </button>
                  <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                  <DropdownMenu v-if="sheetWorkflowHasReruns(workflow)">
                    <DropdownMenuTrigger as-child>
                      <button
                        class="shrink-0 inline-flex items-center gap-0.5 rounded border border-primary/70 px-1 py-0.5 text-[10px] font-medium text-muted-foreground hover:bg-accent transition-colors"
                      >
                        #{{ workflow.attempt }}
                        <Icon name="chevron-down" class="h-2.5 w-2.5" />
                      </button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="start">
                      <DropdownMenuItem
                        v-for="attempt in sheetWorkflowAttempts(workflow)"
                        :key="attempt.id"
                        class="flex items-center gap-2 text-xs"
                        @click="sheetSwitchAttempt(workflow, attempt)"
                      >
                        <PipelineStatusIcon :status="attempt.state" class="h-3.5 w-3.5" />
                        #{{ attempt.attempt }}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                  <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                  <IconButton
                    v-if="hasPushPermission && isSheetWorkflowCompleted(workflow)"
                    :is-loading="isRerunningWorkflow[workflow.id]"
                    icon="solar:restart-bold"
                    class="shrink-0 text-primary hover:text-primary/80 hover:bg-accent"
                    :title="$t('repo.pipeline.actions.rerun_workflow')"
                    @click="handleRerunWorkflow(workflow.id)"
                  />
                </div>
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import { Button } from '~/components/ui/button';
import { ScrollArea } from '~/components/ui/scroll-area';
import Icon from '~/components/atomic/Icon.vue';
import IconButton from '~/components/atomic/IconButton.vue';
import { requiredInject } from '~/compositions/useInjectProvide';
import useApiClient from '~/compositions/useApiClient';
import type { Pipeline, PipelineWorkflow } from '~/lib/api/types';
import { getDisplayedAttempts, getWorkflowAttempts, hasReruns, stepForAttemptSwitch } from '~/lib/pipeline-utils';

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
  (event: 'rerunWorkflow', workflowId: number): void;
  (event: 'rerunStep', workflowId: number, stepId: number): void;
}>();

const apiClient = useApiClient();
const repoPermissions = requiredInject('repo-permissions');
const hasPushPermission = computed(() => repoPermissions.value && repoPermissions.value.push);
const isRerunningWorkflow = ref<Record<number, boolean>>({});

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

const allWorkflows = computed(() => props.pipeline?.workflows || []);
// Show the attempt the selected step belongs to, latest attempt otherwise, so
// the sheet follows the run the log pane is showing (#1520).
const displayedWorkflows = computed(() => getDisplayedAttempts(allWorkflows.value, props.stepId));

const currentStep = computed(() => {
  if (!props.stepId) return null;

  // Search all workflows (not just latest) so older attempts' steps are found
  for (const workflow of allWorkflows.value) {
    const step = workflow.children?.find((s) => s.pid === props.stepId);
    if (step) return step;
  }
  return null;
});

const currentWorkflow = computed(() => {
  if (!props.stepId) return null;

  // Search all workflows (not just latest) so older attempts' steps are found
  for (const workflow of allWorkflows.value) {
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
    agentDetails.value = await apiClient.getAgent(agentId, { silent: true });
  } catch (error) {
    console.warn(`Failed to load agent ${agentId}:`, error);
    agentDetails.value = null;
  }
}

const currentWorkflowAttempts = computed(() => {
  const wf = currentWorkflow.value;
  if (wf == null) return [];
  return getWorkflowAttempts(allWorkflows.value, wf);
});

const hasMultipleAttempts = computed(() => currentWorkflowAttempts.value.length > 1);

const isWorkflowCompleted = computed(() => {
  const wf = currentWorkflow.value;
  return wf != null && wf.state !== 'pending' && wf.state !== 'running';
});

function switchToAttempt(attempt: PipelineWorkflow) {
  const target = stepForAttemptSwitch(attempt, currentStep.value?.name);
  if (target) {
    emit('update:step-id', target.pid);
  }
}

function sheetWorkflowAttempts(workflow: PipelineWorkflow) {
  return getWorkflowAttempts(allWorkflows.value, workflow);
}

function sheetWorkflowHasReruns(workflow: PipelineWorkflow) {
  return hasReruns(allWorkflows.value, workflow);
}

function isSheetWorkflowCompleted(workflow: PipelineWorkflow) {
  return workflow.state !== 'pending' && workflow.state !== 'running';
}

function sheetSwitchAttempt(_workflow: PipelineWorkflow, targetAttempt: PipelineWorkflow) {
  workflowsCollapsed.value[targetAttempt.id] = false;
  const target = stepForAttemptSwitch(targetAttempt, currentStep.value?.name);
  if (target) {
    handleStepSelect(target.pid);
  }
}

function handleRerunWorkflow(workflowId: number) {
  isRerunningWorkflow.value = { ...isRerunningWorkflow.value, [workflowId]: true };
  emit('rerunWorkflow', workflowId);
  // Reset loading state after a delay (parent handles the actual API call)
  setTimeout(() => {
    isRerunningWorkflow.value = { ...isRerunningWorkflow.value, [workflowId]: false };
  }, 2000);
}

// Watch for workflow changes
watch(currentWorkflow, async () => {
  await loadAgentDetails();
});

onMounted(async () => {
  await loadAgentDetails();
});
</script>

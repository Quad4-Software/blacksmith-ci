<template>
  <Container full-width class="md:min-h-xs flex min-h-[calc(100vh-10rem)] md:h-full flex-col md:px-4 overflow-hidden">
    <!-- Desktop Layout (hidden on mobile, ResizablePanelGroup has hardcoded flex that conflicts with hidden) -->
    <div class="hidden md:contents">
      <ResizablePanelGroup
        :key="panelGroupKey"
        ref="panelGroupRef"
        direction="horizontal"
        class="min-h-0 w-full grow"
        @layout="handleLayoutChange"
      >
        <!-- Sidebar Panel -->
        <ResizablePanel
          v-if="!sidebarCollapsed"
          id="sidebar-panel"
          :default-size="sidebarSize"
          :min-size="20"
          :max-size="50"
          :collapsible="false"
        >
          <div class="h-full w-full flex flex-col pr-2">
            <PipelineStepList v-model:selected-step-id="selectedStepId" :pipeline="pipeline!">
              <template #panel-chevron>
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger as-child>
                      <ShadcnButton variant="ghost" size="icon" class="h-6 w-6 p-0" @click="toggleSidebar">
                        <Icon name="chevron-left" class="h-4 w-4" />
                      </ShadcnButton>
                    </TooltipTrigger>
                    <TooltipContent side="right">
                      {{ $t('repo.pipeline.hide_steps') }}
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              </template>
            </PipelineStepList>
          </div>
        </ResizablePanel>

        <!-- Resizable Handle -->
        <ResizableHandle v-if="!sidebarCollapsed" with-handle class="mx-2" />

        <!-- Main Content Panel -->
        <ResizablePanel id="main-panel" :default-size="sidebarCollapsed ? 100 : 100 - sidebarSize">
          <div class="h-full flex flex-col pl-2">
            <!-- Expand button when sidebar is collapsed -->
            <div v-if="sidebarCollapsed" class="flex items-center mb-2">
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <ShadcnButton
                      variant="outline"
                      size="icon"
                      class="h-6 w-6 p-0 rounded-md shadow-sm"
                      @click="toggleSidebar"
                    >
                      <Icon name="chevron-right" class="h-4 w-4" />
                    </ShadcnButton>
                  </TooltipTrigger>
                  <TooltipContent side="right">
                    {{ $t('repo.pipeline.show_steps') }}
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>

            <div v-if="pipeline!.errors?.some((e) => !e.is_warning)" class="w-full mb-auto p-4">
              <Panel>
                <div class="flex flex-col items-center gap-4 text-center">
                  <Icon name="status-error" class="h-16 w-16 text-destructive" size="1.5rem" />
                  <span class="text-xl">{{ $t('repo.pipeline.we_got_some_errors') }}</span>
                  <Button color="red" :text="$t('repo.pipeline.show_errors')" :to="{ name: 'repo-pipeline-errors' }" />
                </div>
              </Panel>
            </div>
            <div v-else-if="pipeline!.status === 'blocked'" class="w-full mb-auto p-4">
              <Panel>
                <div class="flex flex-col items-center gap-4">
                  <Icon name="status-blocked" size="1.5rem" class="h-16 w-16" />
                  <span class="text-xl">{{ $t('repo.pipeline.protected.awaits') }}</span>
                  <div v-if="repoPermissions!.push" class="flex flex-wrap items-center justify-center gap-2">
                    <Button
                      color="default"
                      :text="$t('repo.pipeline.protected.approve')"
                      :is-loading="isApprovingPipeline"
                      @click="approvePipeline"
                    />
                    <Button
                      color="red"
                      :text="$t('repo.pipeline.protected.decline')"
                      :is-loading="isDecliningPipeline"
                      @click="declinePipeline"
                    />
                  </div>
                </div>
              </Panel>
            </div>
            <div v-else-if="pipeline!.status === 'declined'" class="w-full mb-auto p-4">
              <Panel>
                <div class="flex flex-col items-center gap-4">
                  <Icon name="status-declined" size="1.5rem" class="h-16 w-16 text-destructive" />
                  <p class="text-xl">{{ $t('repo.pipeline.protected.declined') }}</p>
                </div>
              </Panel>
            </div>
            <PipelineLog
              v-else-if="selectedStepId !== null"
              v-model:step-id="selectedStepId"
              :pipeline="pipeline!"
              :previous-step-id="previousStepId"
              :next-step-id="nextStepId"
              :auto-follow-enabled="autoFollowEnabled"
              class="w-full h-full"
              @update:auto-follow-enabled="handleAutoFollowUpdate"
            />
          </div>
        </ResizablePanel>
      </ResizablePanelGroup>
    </div>

    <!-- Mobile Layout -->
    <div class="flex md:hidden flex-col flex-1 min-h-0">
      <!-- Error/Blocked/Declined states -->
      <div v-if="pipeline!.errors?.some((e) => !e.is_warning)" class="p-4 flex-1">
        <Panel>
          <div class="flex flex-col items-center gap-4 text-center">
            <Icon name="status-error" class="h-16 w-16 text-destructive" size="1.5rem" />
            <span class="text-xl">{{ $t('repo.pipeline.we_got_some_errors') }}</span>
            <Button color="red" :text="$t('repo.pipeline.show_errors')" :to="{ name: 'repo-pipeline-errors' }" />
          </div>
        </Panel>
      </div>
      <div v-else-if="pipeline!.status === 'blocked'" class="p-4 flex-1">
        <Panel>
          <div class="flex flex-col items-center gap-4">
            <Icon name="status-blocked" size="1.5rem" class="h-16 w-16" />
            <span class="text-xl">{{ $t('repo.pipeline.protected.awaits') }}</span>
            <div v-if="repoPermissions!.push" class="flex flex-wrap items-center justify-center gap-2">
              <Button
                color="default"
                :text="$t('repo.pipeline.protected.approve')"
                :is-loading="isApprovingPipeline"
                @click="approvePipeline"
              />
              <Button
                color="red"
                :text="$t('repo.pipeline.protected.decline')"
                :is-loading="isDecliningPipeline"
                @click="declinePipeline"
              />
            </div>
          </div>
        </Panel>
      </div>
      <div v-else-if="pipeline!.status === 'declined'" class="p-4 flex-1">
        <Panel>
          <div class="flex flex-col items-center gap-4">
            <Icon name="status-declined" size="1.5rem" class="h-16 w-16 text-destructive" />
            <p class="text-xl">{{ $t('repo.pipeline.protected.declined') }}</p>
          </div>
        </Panel>
      </div>
      <!-- Mobile Log Viewer with step info panel and bottom sheet navigation -->
      <PipelineLogMobile
        v-else-if="selectedStepId !== null"
        :pipeline="pipeline!"
        :step-id="selectedStepId"
        :previous-step-id="previousStepId"
        :next-step-id="nextStepId"
        :auto-follow-enabled="autoFollowEnabled"
        class="flex-1"
        @update:step-id="selectedStepId = $event"
        @update:auto-follow-enabled="handleAutoFollowUpdate"
      />
      <!-- Step Selection (when no step is selected) -->
      <div v-else class="h-full overflow-y-auto">
        <div class="px-1 pb-4 pt-1">
          <div class="flex flex-col gap-y-2">
            <div
              v-for="workflow in pipeline!.workflows"
              :key="workflow.id"
              class="rounded-md border border-border bg-card p-2 shadow-xs"
            >
              <!-- Workflow Header with Chevron -->
              <button
                class="flex items-center gap-2 mb-2 w-full text-left hover:opacity-80 transition-opacity"
                @click="toggleMobileWorkflow(workflow.id)"
              >
                <Icon
                  name="chevron-right"
                  class="size-4 transition-transform duration-200 text-muted-foreground"
                  :class="{ 'rotate-90': !mobileWorkflowsCollapsed[workflow.id] }"
                />
                <div class="flex items-center gap-3">
                  <PipelineStatusIcon :status="workflow.state" class="h-4 w-4" />
                  <span class="font-medium text-foreground">{{ workflow.name }}</span>
                </div>
              </button>
              <!-- Steps -->
              <div v-show="!mobileWorkflowsCollapsed[workflow.id]" class="space-y-1">
                <div
                  v-for="step in workflow.children"
                  :key="step.pid"
                  class="flex items-center gap-3 p-2 rounded transition-colors hover:bg-accent cursor-pointer ml-5"
                  @click="selectedStepId = step.pid"
                >
                  <PipelineStatusIcon :service="step.type === 'service'" :status="step.state" class="h-4 w-4" />
                  <span class="flex-1 text-sm text-foreground">{{ step.name }}</span>
                  <PipelineStepDuration :step="step" class="text-xs text-muted-foreground" />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Container>
</template>

<script lang="ts" setup>
import { useStorage, useDebounceFn } from '@vueuse/core';
import { computed, toRef, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';

import Button from '~/components/atomic/Button.vue';
import { Button as ShadcnButton } from '~/components/ui/button';
import Icon from '~/components/atomic/Icon.vue';
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '~/components/ui/resizable';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import Container from '~/components/layout/Container.vue';
import Panel from '~/components/layout/Panel.vue';
import PipelineLog from '~/components/repo/pipeline/PipelineLog.vue';
import PipelineLogMobile from '~/components/repo/pipeline/PipelineLogMobile.vue';
import PipelineStepList from '~/components/repo/pipeline/PipelineStepList.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import PipelineStepDuration from '~/components/repo/pipeline/PipelineStepDuration.vue';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { PipelineStep } from '~/lib/api/types';

const props = defineProps<{
  stepId?: string | null;
}>();

const apiClient = useApiClient();
const router = useRouter();
const route = useRoute();
const notifications = useNotifications();
const i18n = useI18n();

const pipeline = requiredInject('pipeline');
const repo = requiredInject('repo');
const repoPermissions = requiredInject('repo-permissions');

const stepId = toRef(props, 'stepId');

const defaultStepId = computed(() => pipeline.value?.workflows?.[0].children?.[0].pid ?? null);

const selectedStepId = computed({
  get() {
    if (stepId.value !== '' && stepId.value !== null && stepId.value !== undefined) {
      const id = Number.parseInt(stepId.value, 10);

      let step = pipeline.value.workflows?.find((workflow) => workflow.pid === id)?.children[0];
      if (step) {
        return step.pid;
      }

      step = pipeline.value?.workflows?.reduce(
        (prev, p) => prev || p.children?.find((c) => c.pid === id),
        undefined as PipelineStep | undefined,
      );
      if (step) {
        return step.pid;
      }

      // return fallback if step-id is provided, but step cannot be found
      return defaultStepId.value;
    }

    // is opened on >= md-screen
    if (window.innerWidth > 768) {
      return defaultStepId.value;
    }

    return null;
  },
  set(_selectedStepId: number | null) {
    if (_selectedStepId === null) {
      router.replace({ params: { ...route.params, stepId: '' } });
      return;
    }

    router.replace({ params: { ...route.params, stepId: `${_selectedStepId}` } });
  },
});

// Get all steps in order for navigation
const allSteps = computed(() => {
  if (!pipeline.value?.workflows) return [];

  const steps: Array<{ step: PipelineStep; workflow: any }> = [];
  for (const workflow of pipeline.value.workflows) {
    for (const step of workflow.children || []) {
      steps.push({ step, workflow });
    }
  }
  return steps.sort((a, b) => a.step.pid - b.step.pid);
});

const currentStepIndex = computed(() => {
  return allSteps.value.findIndex(({ step }) => step.pid === selectedStepId.value);
});

const previousStepId = computed(() => {
  if (currentStepIndex.value > 0) {
    return allSteps.value[currentStepIndex.value - 1].step.pid;
  }
  return null;
});

const nextStepId = computed(() => {
  if (currentStepIndex.value >= 0 && currentStepIndex.value < allSteps.value.length - 1) {
    return allSteps.value[currentStepIndex.value + 1].step.pid;
  }
  return null;
});

const { doSubmit: approvePipeline, isLoading: isApprovingPipeline } = useAsyncAction(async () => {
  await apiClient.approvePipeline(repo.value.id, `${pipeline.value.number}`);
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('repo.pipeline.protected.notify_text_approve_success'),
    type: 'success',
    duration: 6000,
  });
});

const { doSubmit: declinePipeline, isLoading: isDecliningPipeline } = useAsyncAction(async () => {
  await apiClient.declinePipeline(repo.value.id, `${pipeline.value.number}`);
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('repo.pipeline.protected.notify_text_decline_success'),
    type: 'success',
    duration: 6000,
  });
});

useCrowTitle(
  computed(() => [
    i18n.t('repo.pipeline.tasks'),
    i18n.t('repo.pipeline.pipeline', { pipelineId: pipeline.value.number }),
    repo.value.full_name,
  ]),
);

// Sidebar state with localStorage persistence
const sidebarCollapsed = useStorage('crow:pipeline-sidebar-collapsed', false);
const sidebarSize = useStorage('crow:pipeline-sidebar-size', 30);
const panelGroupRef = ref();
const panelGroupKey = ref(0);

// Auto-follow state with localStorage persistence
const autoFollowEnabled = useStorage('crow:pipeline-auto-follow', false);

function handleAutoFollowUpdate(value: boolean) {
  autoFollowEnabled.value = value;
}

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value;
}

// Debounced function to update storage - only saves after user stops dragging
const debouncedSizeUpdate = useDebounceFn((newSize: number) => {
  sidebarSize.value = newSize;
}, 500);

let lastManualSize = sidebarSize.value;

function handleLayoutChange(layout: number[]) {
  // layout[0] is the sidebar panel size
  if (layout && layout.length > 0) {
    const newSize = layout[0];
    // Only update if there's a meaningful difference (more than 2%)
    // This ignores scrollbar appearance which typically causes ~15px change
    if (Math.abs(sidebarSize.value - newSize) > 2) {
      lastManualSize = newSize;
      debouncedSizeUpdate(newSize);
    }
  }
}

// Watch for external changes to sidebarSize (from popover)
watch(sidebarSize, (newSize) => {
  // Only force re-render if the change didn't come from dragging
  if (Math.abs(lastManualSize - newSize) > 2) {
    panelGroupKey.value++;
    lastManualSize = newSize;
  }
});

const mobileWorkflowsCollapsed = ref<Record<number, boolean>>({});

function toggleMobileWorkflow(workflowId: number) {
  mobileWorkflowsCollapsed.value[workflowId] = !mobileWorkflowsCollapsed.value[workflowId];
}

// Auto-follow logic: automatically switch to the running step
watch(
  [() => pipeline.value?.workflows, autoFollowEnabled],
  ([workflows, enabled]) => {
    if (!enabled || !workflows) {
      return;
    }

    // Find the first running step
    for (const workflow of workflows) {
      const runningStep = workflow.children?.find((step) => step.state === 'running');
      if (runningStep && runningStep.pid !== selectedStepId.value) {
        selectedStepId.value = runningStep.pid;
        return;
      }
    }
  },
  { deep: true, immediate: true },
);
</script>

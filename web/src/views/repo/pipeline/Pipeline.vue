<template>
  <Container full-width class="md:min-h-xs flex min-h-[calc(100vh-10rem)] md:h-full flex-col md:px-4 overflow-hidden">
    <!-- Desktop Layout (hidden on mobile, ResizablePanelGroup has hardcoded flex that conflicts with hidden) -->
    <div class="hidden md:contents">
      <ResizablePanelGroup
        :key="panelGroupKey"
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
                      <Button variant="ghost" size="icon" class="h-6 w-6 p-0" @click="toggleSidebar">
                        <Icon name="chevron-left" class="h-4 w-4" />
                      </Button>
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
                    <Button
                      variant="outline"
                      size="icon"
                      class="h-6 w-6 p-0 rounded-md shadow-sm"
                      @click="toggleSidebar"
                    >
                      <Icon name="chevron-right" class="h-4 w-4" />
                    </Button>
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
                  <Button variant="destructive" as-child>
                    <router-link :to="{ name: 'repo-pipeline-errors' }">{{
                      $t('repo.pipeline.show_errors')
                    }}</router-link>
                  </Button>
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
                      variant="outline"
                      class="relative border-orange-400 hover:border-orange-500 hover:bg-orange-50 dark:hover:bg-orange-950/20"
                      :disabled="isApprovingPipeline"
                      @click="approvePipeline"
                    >
                      <Icon v-if="isApprovingPipeline" name="spinner" class="size-5 animate-spin" />
                      <template v-else>{{ $t('repo.pipeline.protected.approve') }}</template>
                    </Button>
                    <Button
                      variant="destructive"
                      class="relative"
                      :disabled="isDecliningPipeline"
                      @click="declinePipeline"
                    >
                      <Icon v-if="isDecliningPipeline" name="spinner" class="size-5 animate-spin" />
                      <template v-else>{{ $t('repo.pipeline.protected.decline') }}</template>
                    </Button>
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
                variant="outline"
                class="relative border-orange-400 hover:border-orange-500 hover:bg-orange-50 dark:hover:bg-orange-950/20"
                :disabled="isApprovingPipeline"
                @click="approvePipeline"
              >
                <Icon v-if="isApprovingPipeline" name="spinner" class="size-5 animate-spin" />
                <template v-else>{{ $t('repo.pipeline.protected.approve') }}</template>
              </Button>
              <Button variant="destructive" class="relative" :disabled="isDecliningPipeline" @click="declinePipeline">
                <Icon v-if="isDecliningPipeline" name="spinner" class="size-5 animate-spin" />
                <template v-else>{{ $t('repo.pipeline.protected.decline') }}</template>
              </Button>
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
        <div class="px-1 pb-20 pt-1">
          <div class="flex flex-col gap-y-2">
            <template v-for="group in mobileWorkflowGroups" :key="group.key">
              <!-- Matrix group on mobile -->
              <PipelineMatrixGroupMobile
                v-if="group.workflows.length > 1"
                :workflows="group.workflows"
                :group-name="group.key"
                @select-step="(pid: number) => (selectedStepId = pid)"
              />
              <!-- Single workflow (unchanged) -->
              <div v-else class="rounded-md border border-border bg-card p-2 shadow-xs">
                <!-- Workflow Header with Chevron -->
                <button
                  class="flex items-center gap-2 mb-2 w-full text-left hover:opacity-80 transition-opacity"
                  @click="toggleMobileWorkflow(group.workflows[0].id)"
                >
                  <Icon
                    name="chevron-right"
                    class="size-4 transition-transform duration-200 text-muted-foreground"
                    :class="{ 'rotate-90': !mobileWorkflowsCollapsed[group.workflows[0].id] }"
                  />
                  <div class="flex items-center gap-3">
                    <PipelineStatusIcon :status="group.workflows[0].state" class="h-4 w-4" />
                    <span class="font-medium text-foreground">{{ group.workflows[0].name }}</span>
                  </div>
                </button>
                <!-- Steps -->
                <div v-show="!mobileWorkflowsCollapsed[group.workflows[0].id]" class="space-y-1">
                  <div
                    v-for="step in group.workflows[0].children"
                    :key="step.pid"
                    class="flex items-center gap-3 p-2 rounded transition-colors hover:bg-accent cursor-pointer ml-4"
                    @click="selectedStepId = step.pid"
                  >
                    <PipelineStatusIcon :service="step.type === 'service'" :status="step.state" class="h-4 w-4" />
                    <span class="flex-1 text-sm text-foreground">{{ step.name }}</span>
                    <PipelineStepDuration :step="step" class="text-xs text-muted-foreground" />
                  </div>
                </div>
              </div>
            </template>
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

import { Button } from '~/components/ui/button';
import Icon from '~/components/atomic/Icon.vue';
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '~/components/ui/resizable';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import Container from '~/components/layout/Container.vue';
import Panel from '~/components/layout/Panel.vue';
import PipelineLog from '~/components/repo/pipeline/PipelineLog.vue';
import PipelineLogMobile from '~/components/repo/pipeline/PipelineLogMobile.vue';
import PipelineStepList from '~/components/repo/pipeline/PipelineStepList.vue';
import PipelineMatrixGroupMobile from '~/components/repo/pipeline/PipelineMatrixGroupMobile.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import PipelineStepDuration from '~/components/repo/pipeline/PipelineStepDuration.vue';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import { usePipelineNavigation } from '~/compositions/usePipelineNavigation';
import { identifyMatrixGroups } from '~/lib/pipeline-utils';

const props = defineProps<{
  stepId?: string | null;
}>();

const apiClient = useApiClient();
const notifications = useNotifications();
const i18n = useI18n();

const pipeline = requiredInject('pipeline');
const repo = requiredInject('repo');
const repoPermissions = requiredInject('repo-permissions');

const stepId = toRef(props, 'stepId');

const { selectedStepId, previousStepId, nextStepId, autoFollowEnabled, handleAutoFollowUpdate } = usePipelineNavigation(
  { pipeline, stepIdProp: stepId },
);

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
const panelGroupKey = ref(0);

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

const mobileWorkflowGroups = computed(() => identifyMatrixGroups(pipeline.value.workflows || []));

const mobileWorkflowsCollapsed = ref<Record<number, boolean>>({});

function toggleMobileWorkflow(workflowId: number) {
  mobileWorkflowsCollapsed.value[workflowId] = !mobileWorkflowsCollapsed.value[workflowId];
}
</script>

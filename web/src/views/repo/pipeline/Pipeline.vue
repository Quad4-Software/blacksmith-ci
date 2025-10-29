<template>
  <Container full-width class="md:min-h-xs flex grow-0 flex-col md:grow md:px-4">
    <!-- Desktop Layout -->
    <div class="hidden md:flex min-h-0 w-full grow flex-row gap-2">
      <div class="flex flex-row">
        <!-- Chevron button in its own vertical panel -->
        <div class="flex flex-col items-center justify-start w-10 min-w-[2.5rem] max-w-[2.5rem]">
          <button
            class="mb-2 mr-2 bg-wp-background-100 dark:bg-wp-background-200 border border-wp-background-300 dark:border-wp-background-400 rounded-full p-1 shadow flex items-center justify-center text-2xl"
            :class="pipelinePanelCollapsed ? 'mt-0' : 'mt-2'"
            :title="pipelinePanelCollapsed ? 'Show workflow panel' : 'Hide workflow panel'"
            @click="togglePipelinePanel"
          >
            <Icon
              name="chevron-right"
              class="text-gray-700 dark:text-gray-200"
              :class="
                pipelinePanelCollapsed
                  ? 'transition-transform duration-150 rotate-90 transform w-7 h-7'
                  : 'transition-transform duration-150 w-7 h-7'
              "
            />
          </button>
        </div>
        <!-- Workflow panel -->
        <div
          class="flex flex-col transition-all duration-200 overflow-hidden"
          :class="[pipelinePanelCollapsed ? 'w-0 min-w-0 max-w-0' : 'w-lg']"
        >
          <PipelineStepList v-model:selected-step-id="selectedStepId" :pipeline="pipeline!" />
        </div>
      </div>
      <!-- Right panel: log and other content -->
      <div class="relative flex grow items-start justify-center basis-auto">
        <div v-if="pipeline!.errors?.some((e) => !e.is_warning)" class="w-full mb-auto">
          <Panel>
            <div class="flex flex-col items-center gap-4 text-center">
              <Icon name="status-error" class="h-16 w-16 text-wp-error-100" size="1.5rem" />
              <span class="text-xl">{{ $t('repo.pipeline.we_got_some_errors') }}</span>
              <Button color="red" :text="$t('repo.pipeline.show_errors')" :to="{ name: 'repo-pipeline-errors' }" />
            </div>
          </Panel>
        </div>
        <div v-else-if="pipeline!.status === 'blocked'" class="w-full mb-auto">
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
        <div v-else-if="pipeline!.status === 'declined'" class="w-full mb-auto">
          <Panel>
            <div class="flex flex-col items-center gap-4">
              <Icon name="status-declined" size="1.5rem" class="h-16 w-16 text-wp-error-100" />
              <p class="text-xl">{{ $t('repo.pipeline.protected.declined') }}</p>
            </div>
          </Panel>
        </div>
        <PipelineLog
          v-else-if="selectedStepId !== null"
          v-model:step-id="selectedStepId"
          :pipeline="pipeline!"
          class="w-full h-full"
        />
      </div>
    </div>

    <!-- Mobile Layout -->
    <div class="flex md:hidden flex-col h-full">
      <!-- Mobile Header with current step info -->
      <div
        v-if="selectedStepId !== null"
        class="flex items-center justify-between p-2 bg-wp-background-100 dark:bg-wp-background-200 border-b border-wp-background-300 dark:border-wp-background-400"
      >
        <div class="flex items-center gap-3 min-w-0 flex-1">
          <PipelineStatusIcon :status="currentStep?.state || 'pending'" class="h-5 w-5 shrink-0" />
          <div class="min-w-0 flex-1">
            <div class="font-medium text-wp-text-100 truncate">{{ currentStep?.name || 'Loading...' }}</div>
            <div class="text-sm text-wp-text-alt-100 truncate">{{ currentWorkflow?.name }}</div>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <!-- Step navigation buttons -->
          <button
            v-if="previousStepId"
            class="flex items-center justify-center w-9 h-9 rounded-full bg-wp-background-200 dark:bg-wp-background-300 border border-wp-background-300 dark:border-wp-background-400"
            :title="$t('repo.pipeline.previous_step')"
            @click="selectedStepId = previousStepId"
          >
            <Icon name="back" class="w-4 h-4 text-wp-text-100" />
          </button>
          <button
            v-if="nextStepId"
            class="flex items-center justify-center w-9 h-9 rounded-full bg-wp-background-200 dark:bg-wp-background-300 border border-wp-background-300 dark:border-wp-background-400"
            :title="$t('repo.pipeline.next_step')"
            @click="selectedStepId = nextStepId"
          >
            <Icon name="next" class="w-4 h-4 text-wp-text-100" />
          </button>
        </div>
      </div>

      <!-- Mobile Content Area -->
      <div class="flex-1 min-h-0 relative" @touchstart="handleTouchStart" @touchend="handleTouchEnd">
        <div v-if="pipeline!.errors?.some((e) => !e.is_warning)" class="p-4">
          <Panel>
            <div class="flex flex-col items-center gap-4 text-center">
              <Icon name="status-error" class="h-16 w-16 text-wp-error-100" size="1.5rem" />
              <span class="text-xl">{{ $t('repo.pipeline.we_got_some_errors') }}</span>
              <Button color="red" :text="$t('repo.pipeline.show_errors')" :to="{ name: 'repo-pipeline-errors' }" />
            </div>
          </Panel>
        </div>
        <div v-else-if="pipeline!.status === 'blocked'" class="p-4">
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
        <div v-else-if="pipeline!.status === 'declined'" class="p-4">
          <Panel>
            <div class="flex flex-col items-center gap-4">
              <Icon name="status-declined" size="1.5rem" class="h-16 w-16 text-wp-error-100" />
              <p class="text-xl">{{ $t('repo.pipeline.protected.declined') }}</p>
            </div>
          </Panel>
        </div>
        <PipelineLog
          v-else-if="selectedStepId !== null"
          v-model:step-id="selectedStepId"
          :pipeline="pipeline!"
          class="w-full h-full"
        />
        <div v-else class="h-full overflow-y-auto">
          <!-- Integrated Step Selection -->
          <div class="md:px-2 px-1 pb-4 pt-1">
            <div class="flex flex-col gap-y-2">
              <div
                v-for="workflow in pipeline!.workflows"
                :key="workflow.id"
                class="rounded-md border border-wp-background-400 bg-wp-background-100 p-2 shadow-xs dark:bg-wp-background-200"
              >
                <!-- Workflow Header with Chevron -->
                <div
                  class="flex items-center gap-2 mb-2 cursor-pointer select-none"
                  @click="toggleMobileWorkflow(workflow.id)"
                >
                  <Icon
                    name="chevron-right"
                    class="transition-transform duration-150 text-gray-500 dark:text-gray-200"
                    :class="{ 'rotate-90 transform': !mobileWorkflowsCollapsed[workflow.id] }"
                  />
                  <div class="flex items-center gap-3">
                    <PipelineStatusIcon :status="workflow.state" class="h-4 w-4" />
                    <span class="font-medium text-wp-text-100">{{ workflow.name }}</span>
                  </div>
                </div>
                <!-- Steps -->
                <div v-show="!mobileWorkflowsCollapsed[workflow.id]" class="space-y-1">
                  <div
                    v-for="step in workflow.children"
                    :key="step.pid"
                    class="flex items-center gap-3 p-2 rounded transition-colors hover:bg-wp-background-300 dark:hover:bg-wp-background-400 cursor-pointer ml-5"
                    @click="selectedStepId = step.pid"
                  >
                    <PipelineStatusIcon :service="step.type === 'service'" :status="step.state" class="h-4 w-4" />
                    <span class="flex-1 text-sm text-wp-text-100">{{ step.name }}</span>
                    <PipelineStepDuration :step="step" class="text-xs text-wp-text-alt-100" />
                  </div>
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
import { computed, toRef, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';

import Button from '~/components/atomic/Button.vue';
import Icon from '~/components/atomic/Icon.vue';
import Container from '~/components/layout/Container.vue';
import Panel from '~/components/layout/Panel.vue';
import PipelineLog from '~/components/repo/pipeline/PipelineLog.vue';
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

// Touch gesture state
const touchStartX = ref(0);
const touchStartY = ref(0);

// Computed properties for mobile view
const currentStep = computed(() => {
  if (!selectedStepId.value || !pipeline.value?.workflows) return null;

  for (const workflow of pipeline.value.workflows) {
    const step = workflow.children?.find((s) => s.pid === selectedStepId.value);
    if (step) return step;
  }
  return null;
});

const currentWorkflow = computed(() => {
  if (!selectedStepId.value || !pipeline.value?.workflows) return null;

  for (const workflow of pipeline.value.workflows) {
    const step = workflow.children?.find((s) => s.pid === selectedStepId.value);
    if (step) return workflow;
  }
  return null;
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

// Touch gesture handlers
function handleTouchStart(event: TouchEvent) {
  touchStartX.value = event.touches[0].clientX;
  touchStartY.value = event.touches[0].clientY;
}

function handleTouchEnd(event: TouchEvent) {
  const touchEndX = event.changedTouches[0].clientX;
  const touchEndY = event.changedTouches[0].clientY;

  const deltaX = touchEndX - touchStartX.value;
  const deltaY = touchEndY - touchStartY.value;

  // Minimum swipe distance
  const minSwipeDistance = 50;

  // Check if it's a horizontal swipe (more horizontal than vertical)
  if (Math.abs(deltaX) > Math.abs(deltaY) && Math.abs(deltaX) > minSwipeDistance) {
    if (deltaX > 0 && previousStepId.value) {
      // Swipe right - go to previous step
      selectedStepId.value = previousStepId.value;
    } else if (deltaX < 0 && nextStepId.value) {
      // Swipe left - go to next step
      selectedStepId.value = nextStepId.value;
    }
  }
}

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

const pipelinePanelCollapsed = ref(false);

function togglePipelinePanel() {
  pipelinePanelCollapsed.value = !pipelinePanelCollapsed.value;
}

const mobileWorkflowsCollapsed = ref<Record<number, boolean>>({});

function toggleMobileWorkflow(workflowId: number) {
  mobileWorkflowsCollapsed.value[workflowId] = !mobileWorkflowsCollapsed.value[workflowId];
}
</script>

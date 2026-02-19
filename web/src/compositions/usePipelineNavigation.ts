import { useStorage } from '@vueuse/core';
import type { Ref } from 'vue';
import { computed, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import type { Pipeline, PipelineStep } from '~/lib/api/types';

export interface UsePipelineNavigationOptions {
  pipeline: Ref<Pipeline>;
  stepIdProp: Ref<string | null | undefined>;
}

export function usePipelineNavigation({ pipeline, stepIdProp }: UsePipelineNavigationOptions) {
  const router = useRouter();
  const route = useRoute();

  const defaultStepId = computed(() => pipeline.value?.workflows?.[0].children?.[0].pid ?? null);

  const selectedStepId = computed({
    get() {
      if (stepIdProp.value !== '' && stepIdProp.value !== null && stepIdProp.value !== undefined) {
        const id = Number.parseInt(stepIdProp.value, 10);

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
        void router.replace({ params: { ...route.params, stepId: '' } });
        return;
      }

      void router.replace({ params: { ...route.params, stepId: `${_selectedStepId}` } });
    },
  });

  // Get all steps in order for navigation
  const allSteps = computed(() => {
    if (!pipeline.value?.workflows) return [];

    const steps: Array<{ step: PipelineStep; workflow: (typeof pipeline.value.workflows)[number] }> = [];
    for (const workflow of pipeline.value.workflows) {
      for (const step of workflow.children ?? []) {
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

  // Auto-follow state with localStorage persistence
  const autoFollowEnabled = useStorage('crow:pipeline-auto-follow', false);

  function handleAutoFollowUpdate(value: boolean) {
    autoFollowEnabled.value = value;
  }

  // Find the workflow containing a given step ID
  function getWorkflowForStep(stepId: number | null) {
    if (stepId === null || stepId === 0 || !pipeline.value?.workflows) return null;
    return pipeline.value.workflows.find((w) => w.children?.some((s) => s.pid === stepId)) ?? null;
  }

  // Auto-follow logic: automatically switch to the running step within the current workflow
  watch(
    [() => pipeline.value?.workflows, autoFollowEnabled, selectedStepId],
    ([workflows, enabled, currentStepId]) => {
      if (!enabled || !workflows) {
        return;
      }

      // Find the workflow containing the currently selected step
      const currentWorkflow = getWorkflowForStep(currentStepId);

      if (currentWorkflow) {
        // Only auto-follow within the current workflow
        const runningStep = currentWorkflow.children?.find((step) => step.state === 'running');
        if (runningStep && runningStep.pid !== currentStepId) {
          selectedStepId.value = runningStep.pid;
        }
      } else {
        // No step selected yet - find the first running step in any workflow
        for (const workflow of workflows) {
          const runningStep = workflow.children?.find((step) => step.state === 'running');
          if (runningStep) {
            selectedStepId.value = runningStep.pid;
            return;
          }
        }
      }
    },
    { deep: true, immediate: true },
  );

  return {
    selectedStepId,
    defaultStepId,
    previousStepId,
    nextStepId,
    autoFollowEnabled,
    handleAutoFollowUpdate,
  };
}

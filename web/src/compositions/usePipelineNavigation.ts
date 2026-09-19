import { useStorage } from '@vueuse/core';
import type { Pipeline, PipelineStep, PipelineWorkflow } from '~/lib/api/types';
import { StepType } from '~/lib/api/types';
import { getLatestAttempts, stepForAttemptSwitch, variantKey } from '~/lib/pipeline-utils';
import type { Ref } from 'vue';
import { computed, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';

export interface UsePipelineNavigationOptions {
  pipeline: Ref<Pipeline>;
  stepIdProp: Ref<string | null | undefined>;
}

// Pick the running step within a workflow that auto-follow should switch to.
// Service steps stay in the "running" state for the whole workflow lifetime, so
// they must be excluded — otherwise auto-follow keeps jumping back to the
// service instead of advancing to the actual running task step (issue #1219).
function findRunningStep(workflow: PipelineWorkflow): PipelineStep | undefined {
  return workflow.children?.find(
    (step) => (step.state === 'preparing' || step.state === 'running') && step.type !== StepType.Service,
  );
}

// Determine which step auto-follow should select, given the current selection.
// Returns the step's pid, or null when there is nothing to follow.
export function getAutoFollowStepId(workflows: PipelineWorkflow[], currentStepId: number | null): number | null {
  const currentWorkflow =
    currentStepId === null || currentStepId === 0
      ? null
      : (workflows.find((w) => w.children?.some((s) => s.pid === currentStepId)) ?? null);

  if (currentWorkflow) {
    // Only auto-follow within the current workflow.
    const runningStep = findRunningStep(currentWorkflow);
    if (runningStep && runningStep.pid !== currentStepId) {
      return runningStep.pid;
    }
    return null;
  }

  // No step selected yet - find the first running step in any workflow.
  for (const workflow of workflows) {
    const runningStep = findRunningStep(workflow);
    if (runningStep) {
      return runningStep.pid;
    }
  }
  return null;
}

// Determine which step to select when a rerun produced a newer attempt of the
// workflow that owns the current selection, so the user follows the run that
// just started instead of staying on the finished one they re-ran. Returns null
// when nothing new appeared for that workflow.
//
// `seenAttempts` maps a workflow variant to the highest attempt seen so far and
// is updated in place; while it is empty (first call) nothing is followed, so
// opening a link to a previous run does not jump to the latest attempt.
export function getRerunFollowStepId(
  workflows: PipelineWorkflow[],
  currentStepId: number | null,
  seenAttempts: Map<string, number>,
): number | null {
  const initialized = seenAttempts.size > 0;
  const currentWorkflow =
    currentStepId === null ? null : (workflows.find((w) => w.children?.some((s) => s.pid === currentStepId)) ?? null);
  const currentStepName = currentWorkflow?.children.find((s) => s.pid === currentStepId)?.name;
  const currentKey = currentWorkflow === null ? null : variantKey(currentWorkflow);

  let followStepId: number | null = null;
  for (const workflow of getLatestAttempts(workflows)) {
    const key = variantKey(workflow);
    const seen = seenAttempts.get(key);
    seenAttempts.set(key, workflow.attempt);

    if (!initialized || seen === undefined || workflow.attempt <= seen) {
      continue;
    }
    if (key !== currentKey || workflow.id === currentWorkflow?.id) {
      continue;
    }

    const target = stepForAttemptSwitch(workflow, currentStepName);
    if (target) {
      followStepId = target.pid;
    }
  }

  return followStepId;
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

  // Follow reruns: the step list renders the attempt owning the selected step,
  // so a rerun must move the selection into the new attempt to keep showing the
  // run that just started.
  const seenAttempts = new Map<string, number>();
  watch(
    () => pipeline.value?.workflows,
    (workflows) => {
      if (!workflows) {
        return;
      }
      const followStepId = getRerunFollowStepId(workflows, selectedStepId.value, seenAttempts);
      if (followStepId !== null) {
        selectedStepId.value = followStepId;
      }
    },
    { deep: true, immediate: true },
  );

  // Auto-follow state with localStorage persistence
  const autoFollowEnabled = useStorage('crow:pipeline-auto-follow', false);

  function handleAutoFollowUpdate(value: boolean) {
    autoFollowEnabled.value = value;
  }

  // Auto-follow logic: automatically switch to the running step, skipping services
  watch(
    [() => pipeline.value?.workflows, autoFollowEnabled, selectedStepId],
    ([workflows, enabled, currentStepId]) => {
      if (!enabled || !workflows) {
        return;
      }

      const nextStepId = getAutoFollowStepId(workflows, currentStepId);
      if (nextStepId !== null) {
        selectedStepId.value = nextStepId;
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

<template>
  <div
    class="bg-white dark:bg-gray-800 border-2 rounded-lg p-4 shadow-md dark:shadow-lg dark:shadow-black/25 relative min-w-[120px] max-w-[320px] w-max transition-all duration-200 cursor-pointer"
    :class="{
      'border-green-500 bg-green-50 dark:border-green-400 dark:bg-green-900/20': state === 'success',
      'border-red-600 bg-red-50 dark:bg-red-900/20': state === 'failure' || state === 'error',
      'border-yellow-600/40 bg-yellow-50 dark:border-yellow-400 dark:bg-yellow-900/20': state === 'running',
      'border-gray-300 dark:border-gray-600': state === 'pending',
      'border-dashed border-gray-300 opacity-70 dark:border-gray-600': state === 'skipped',
      'border-blue-500 bg-blue-50 dark:border-blue-400 dark:bg-blue-900/20': state === 'killed',
      'ring-2 ring-primary ring-opacity-50 scale-105': selected,
    }"
  >
    <PipelineDagHandle :node-id="id" type="target" kind="step" />
    <div class="flex items-start justify-between mb-2 gap-2">
      <div class="font-semibold text-gray-900 dark:text-gray-100 leading-tight wrap-break-word flex-1">
        {{ data.step.name }}
      </div>
      <div
        class="w-3 h-3 rounded-full"
        :class="{
          'bg-green-500 dark:bg-green-400': state === 'success',
          'bg-red-600': state === 'failure' || state === 'error',
          'bg-yellow-500 dark:bg-yellow-400': state === 'running',
          'bg-gray-400 dark:bg-gray-500': state === 'pending' || state === 'skipped',
          'bg-blue-500 dark:bg-blue-400': state === 'killed',
        }"
      ></div>
    </div>
    <div v-if="skipReasonText" class="text-xs italic text-gray-500 dark:text-gray-400 mt-1 wrap-break-word">
      {{ skipReasonText }}
    </div>
    <div v-else class="text-xs text-gray-500 dark:text-gray-400 mt-1 wrap-break-word">
      {{ executionTime }}
    </div>
    <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
    <div v-if="Object.keys(data.workflow.environ ?? {}).length > 0" class="flex flex-wrap gap-1 mt-1.5">
      <span
        v-for="(value, key) in data.workflow.environ"
        :key="key"
        class="text-[10px] bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 px-1.5 py-0.5 rounded font-mono leading-tight"
      >
        {{ key }}=<span class="font-semibold" :style="{ color: matrixValueColor(String(value)) }">{{ value }}</span>
      </span>
    </div>
    <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
    <PipelineDagHandle :node-id="id" type="source" kind="step" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import PipelineDagHandle from './PipelineDagHandle.vue';
import type { StepNodeData } from '~/lib/pipeline-dag-graph';
import { matrixValueColor } from '~/lib/pipeline-utils';

const props = defineProps<{
  id: string;
  data: StepNodeData;
  selected: boolean;
}>();

const { t } = useI18n();

const state = computed(() => props.data.step.state);

function formatDuration(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return minutes > 0 ? `${minutes}m ${seconds}s` : `${seconds}s`;
}

const executionTime = computed(() => {
  const { step } = props.data;
  if (step.started && step.finished) {
    return formatDuration(step.finished - step.started);
  }
  if (step.started && step.state === 'running') {
    return `${formatDuration(Math.floor(Date.now() / 1000 - step.started))} (running)`;
  }
  return '-';
});

const skipReasonText = computed(() => {
  const reason = props.data.skipReason;
  switch (reason?.kind) {
    case 'dependency_failed':
      return t('repo.pipeline.skip_reason.dependency_failed', { step: reason.dependency });
    case 'cancelled':
      return t('repo.pipeline.skip_reason.cancelled');
    case 'workflow_skipped':
      return t('repo.pipeline.skip_reason.workflow_skipped');
    case 'workflow_declined':
      return t('repo.pipeline.skip_reason.workflow_declined');
    case 'workflow_stopped':
      return t('repo.pipeline.skip_reason.workflow_stopped');
    case 'only_on_failure':
      return t('repo.pipeline.skip_reason.only_on_failure');
    default:
      return '';
  }
});
</script>

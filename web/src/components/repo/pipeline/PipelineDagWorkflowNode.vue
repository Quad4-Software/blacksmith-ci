<template>
  <div
    class="workflow-box border-2 border-dashed rounded-lg p-4 transition-all duration-200 bg-transparent dark:bg-gray-800/20 shadow-sm dark:shadow-black/20 relative"
    :class="{
      'border-green-500/40 dark:border-green-400/60': state === 'success',
      'border-red-600/40 dark:border-red-400/60': state === 'failure' || state === 'error',
      'border-yellow-600/40 dark:border-yellow-400/60': state === 'running',
      'border-blue-800 dark:border-blue-400/60': state === 'killed',
      'border-gray-800 dark:border-gray-600/60': isIdle,
    }"
    :style="{ width: `${data.width}px`, height: `${data.height}px` }"
  >
    <PipelineDagHandle :node-id="id" type="target" kind="workflow" />
    <PipelineDagHandle :node-id="id" type="source" kind="workflow" />

    <div class="workflow-header absolute top-4 left-7 flex items-center gap-2 z-20">
      <div
        class="text-sm font-bold px-3 py-1 rounded-lg whitespace-nowrap text-center backdrop-blur-sm"
        :class="{
          'bg-green-600 border-green-700 text-white shadow-green-500/25': state === 'success',
          'bg-red-600 border-red-700 text-white shadow-red-500/25': state === 'failure' || state === 'error',
          'bg-amber-500 border-amber-600 text-white shadow-amber-500/25': state === 'running',
          'bg-blue-700 border-blue-800 text-white shadow-blue-500/25': state === 'killed',
          'bg-slate-600 border-slate-700 text-white shadow-slate-500/25': isIdle,
        }"
      >
        <span class="flex items-center gap-2">
          <div
            class="w-2.5 h-2.5 rounded-full ring-1 ring-white/30"
            :class="{
              'bg-green-200': state === 'success',
              'bg-red-200': state === 'failure' || state === 'error',
              'bg-amber-200': state === 'running',
              'bg-blue-200': state === 'killed',
              'bg-slate-200': isIdle,
            }"
          ></div>
          <span class="font-bold">{{ data.workflow.name }}</span>
          <span v-if="data.isMatrixGroup" class="text-xs font-normal opacity-90 bg-black/10 px-1.5 py-0.5 rounded">
            {{ `${data.matrixWorkflows.length}×` }}
          </span>
        </span>
      </div>

      <div
        v-if="data.workflow.depends_on?.length"
        class="text-xs bg-gray-100 dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 px-2 py-1 rounded-md font-medium backdrop-blur-sm"
      >
        {{ `${t('repo.pipeline.workflow_dependencies')}: ${data.workflow.depends_on.length}` }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import PipelineDagHandle from './PipelineDagHandle.vue';
import type { GroupNodeData } from '~/lib/pipeline-dag-graph';

const props = defineProps<{
  id: string;
  data: GroupNodeData;
}>();

const { t } = useI18n();

const state = computed(() => props.data.workflow.state);
const isIdle = computed(() => state.value === 'pending' || state.value === 'skipped');
</script>

<style scoped>
.workflow-box {
  background: transparent !important;
  border-style: dashed !important;
  border-width: 2px !important;
  backdrop-filter: blur(1px);
  transition: all 0.3s ease;
}

.workflow-box:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

/* The header must not overlap the steps */
.workflow-header {
  min-height: 32px;
  min-width: fit-content;
  max-width: calc(100% - 32px);
  white-space: nowrap;
  justify-content: center;
  text-align: center;
  backdrop-filter: blur(8px);
  transition: all 0.2s ease;
}

.dark .workflow-header {
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
}

.workflow-header > div {
  transition: all 0.2s ease;
}

.workflow-box:hover .workflow-header > div {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.dark .workflow-header > div {
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4);
}

.dark .workflow-box:hover .workflow-header > div {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.6);
}

.workflow-header span {
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  width: 100%;
}
</style>

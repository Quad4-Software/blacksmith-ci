<template>
  <Handle
    :id="`${nodeId}-${type}`"
    :type="type"
    :position="type === 'target' ? Position.Left : Position.Right"
    :class="kind === 'step' ? 'step-handle' : 'workflow-handle'"
    class="!bg-gray-500 dark:!bg-gray-400"
  />
</template>

<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core';

defineProps<{
  nodeId: string;
  type: 'source' | 'target';
  // Workflow group handles are larger than step handles.
  kind: 'step' | 'workflow';
}>();
</script>

<style scoped>
/* eslint-disable vue-scoped-css/no-unused-selector -- Handle renders these classes */
/* A thin rectangular platform instead of Vue Flow's round handle */
.vue-flow__handle {
  border-radius: 1px !important;
  border: none !important;
  box-shadow: none !important;
}

.step-handle.vue-flow__handle {
  width: 4px !important;
  height: 16px !important;
  opacity: 0.6;
}

.workflow-handle.vue-flow__handle {
  width: 6px !important;
  height: 24px !important;
  opacity: 0.5;
}

.vue-flow__handle-left {
  left: -2px !important;
  top: 50% !important;
  transform: translateY(-50%) !important;
}

.vue-flow__handle-right {
  right: -2px !important;
  top: 50% !important;
  transform: translateY(-50%) !important;
}
</style>

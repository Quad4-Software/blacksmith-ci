<template>
  <svg class="vue-flow__marker vue-flow__container">
    <defs>
      <marker
        :id="id"
        class="vue-flow__arrowhead"
        viewBox="-6 -2 12 4"
        refX="5"
        refY="0"
        :markerWidth="width"
        :markerHeight="height"
        markerUnits="strokeWidth"
        orient="auto"
      >
        <rect
          v-if="type === 'platform'"
          :style="{
            stroke,
            strokeWidth,
          }"
          :fill="fill"
          x="-5"
          y="-1.5"
          width="10"
          height="3"
          rx="0.5"
        />

        <rect
          v-if="type === 'platform-large'"
          :style="{
            stroke,
            strokeWidth,
          }"
          :fill="fill"
          x="-6"
          y="-2"
          width="12"
          height="4"
          rx="0.5"
        />
      </marker>
    </defs>
  </svg>
</template>

<script setup lang="ts">
interface Props {
  id: string;
  type: string;
  stroke?: string;
  fill?: string;
  strokeWidth?: number;
  width?: number;
  height?: number;
}

withDefaults(defineProps<Props>(), {
  stroke: '#6b7280',
  fill: '#6b7280',
  strokeWidth: 0,
  width: 12,
  height: 4,
});
</script>

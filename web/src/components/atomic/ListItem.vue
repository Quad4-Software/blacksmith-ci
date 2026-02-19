<template>
  <component
    :is="to ? 'router-link' : clickable ? 'button' : 'div'"
    :to="to"
    class="flex w-full overflow-hidden rounded-md border border-border bg-card p-4 text-card-foreground shadow-xs transition-colors duration-150"
    :class="{
      'cursor-pointer hover:bg-accent hover:shadow-xs': clickable || to,
    }"
  >
    <slot />
  </component>
</template>

<script lang="ts" setup>
import type { RouteLocationRaw } from 'vue-router';

defineProps<{
  clickable?: boolean;
  to?: RouteLocationRaw;
}>();
</script>

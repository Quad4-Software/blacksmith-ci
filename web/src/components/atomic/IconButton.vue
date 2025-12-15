<template>
  <router-link v-if="to" :to="to" :title="title" :aria-label="title" class="icon-button h-8 w-8">
    <slot> <Icon v-if="icon" :name="icon" /> </slot>
  </router-link>
  <a
    v-else-if="href"
    :href="href"
    :title="title"
    :aria-label="title"
    class="icon-button"
    target="_blank"
    rel="noopener noreferrer"
  >
    <slot> <Icon v-if="icon" :name="icon" /> </slot>
  </a>
  <button v-else :disabled="disabled" class="icon-button" type="button" :title="title" :aria-label="title">
    <slot> <Icon v-if="icon" :name="icon" /> </slot>
    <div v-if="isLoading" class="absolute bottom-0 left-0 right-0 top-0 flex items-center justify-center">
      <Icon name="loading" class="animate-spin" />
    </div>
  </button>
</template>

<script lang="ts" setup>
import type { RouteLocationRaw } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
import type { IconNames } from '~/components/atomic/Icon.types';

defineProps<{
  icon?: IconNames;
  disabled?: boolean;
  to?: RouteLocationRaw;
  isLoading?: boolean;
  title?: string;
  href?: string;
}>();
</script>

<style scoped>
.icon-button {
  position: relative;
  display: flex;
  cursor: pointer;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 0.375rem;
  background-color: transparent;
  padding-left: 0.25rem;
  padding-right: 0.25rem;
  padding-top: 0.25rem;
  padding-bottom: 0.25rem;
  transition-property: color, background-color, border-color, text-decoration-color, fill, stroke;
  transition-duration: 100ms;
}

.icon-button:hover {
  background-color: rgba(0, 0, 0, 0.1);
}

.dark .icon-button:hover {
  background-color: rgba(255, 255, 255, 0.05);
}

.icon-button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}
</style>

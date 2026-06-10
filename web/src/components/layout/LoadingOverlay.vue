<template>
  <Teleport to="body">
    <Transition name="loading-overlay">
      <div
        v-if="state.visible"
        class="fixed inset-0 z-[100] bg-background/80 backdrop-blur-sm flex items-center justify-center"
      >
        <div
          class="bg-card text-card-foreground border rounded-lg shadow-lg px-6 py-5 flex flex-col items-center gap-3"
          role="status"
          aria-live="polite"
          aria-busy="true"
        >
          <Icon name="spinner" class="animate-spin h-8 w-8 text-primary" />
          <p class="text-sm">{{ state.text }}</p>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script lang="ts" setup>
import { watchEffect } from 'vue';
import { useEventListener, useScrollLock } from '@vueuse/core';

import Icon from '~/components/atomic/Icon.vue';
import { useLoadingOverlay } from '~/compositions/useLoadingOverlay';

const { state } = useLoadingOverlay();

useEventListener(
  window,
  'keydown',
  (event: KeyboardEvent) => {
    if (!state.visible) {
      return;
    }
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
    }
  },
  { capture: true },
);

const isLocked = useScrollLock(document.body);
watchEffect(() => {
  isLocked.value = state.visible;
});
</script>

<!-- eslint-disable vue-scoped-css/no-unused-selector -->
<style scoped>
.loading-overlay-enter-active {
  transition: opacity 0.15s ease;
}
.loading-overlay-leave-active {
  transition: opacity 0.1s ease;
}
.loading-overlay-enter-from,
.loading-overlay-leave-to {
  opacity: 0;
}
</style>

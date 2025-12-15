<template>
  <div
    class="mx-auto w-full max-w-full px-3 py-4 sm:px-4 box-border overflow-x-hidden"
    :class="{ 'md:px-0': fullWidth }"
  >
    <slot />
  </div>
</template>

<script setup lang="ts">
defineProps<{
  fullWidth?: boolean;
  fillWidth?: boolean;
}>();
</script>

<template>
  <div :class="cn('mb-4 flex flex-col space-y-1.5', $attrs.class ?? '')">
    <slot />
  </div>
</template>

<script setup lang="ts">
import { cn } from '~/lib/utils';
</script>

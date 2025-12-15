<template>
  <div :class="cn('', $attrs.class ?? '')">
    <slot />
  </div>
</template>

<script setup lang="ts">
import { cn } from '~/lib/utils';
</script>

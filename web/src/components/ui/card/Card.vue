<template>
  <div :class="cn('rounded-md border border-border bg-card p-4 text-card-foreground shadow-xs', $attrs.class ?? '')">
    <slot />
  </div>
</template>

<script setup lang="ts">
import { cn } from '~/lib/utils';
</script>

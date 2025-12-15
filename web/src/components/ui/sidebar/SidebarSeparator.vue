<template>
  <Separator
    data-slot="sidebar-separator"
    data-sidebar="separator"
    :class="cn('mx-2 w-auto', props.class)"
    :style="{ backgroundColor: 'var(--border)' }"
  >
    <slot />
  </Separator>
</template>

<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { cn } from '~/lib/utils';
import { Separator } from '~/components/ui/separator';

const props = defineProps<{
  class?: HTMLAttributes['class'];
}>();
</script>

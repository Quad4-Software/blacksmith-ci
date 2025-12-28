<template>
  <td
    data-slot="table-cell"
    :class="
      cn(
        'p-2 pl-5 align-middle whitespace-nowrap overflow-hidden text-ellipsis [&:has([role=checkbox])]:pr-0 *:[[role=checkbox]]:translate-y-[2px]',
        props.class,
      )
    "
  >
    <slot />
  </td>
</template>

<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { cn } from '~/lib/utils';

const props = defineProps<{
  class?: HTMLAttributes['class'];
}>();
</script>

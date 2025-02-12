<template>

  <div
    class="flex items-center gap-4 rounded-md border border-l-4 border-solid border-wp-hint-warn-200 bg-wp-hint-warn-100 p-4 font-bold text-wp-text-100"
  >
     <Icon v-if="!textOnly" name="alert" class="flex-shrink-0 text-wp-hint-warn-200" /> <slot
      > <span class="whitespace-pre-wrap">{{ text }}</span
      > </slot
    >
  </div>

</template>

<script lang="ts" setup>
import Icon from './Icon.vue';

defineProps<{
  textOnly?: boolean;
  text?: string;
}>();
</script>

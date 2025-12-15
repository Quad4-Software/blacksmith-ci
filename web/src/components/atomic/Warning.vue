<template>
  <div
    class="flex items-center gap-4 rounded-md border border-l-4 border-solid border-yellow-500 bg-yellow-500/10 p-4 font-bold text-foreground"
  >
    <Icon v-if="!textOnly" name="alert" class="shrink-0 text-yellow-500" />
    <slot>
      <span class="whitespace-pre-wrap">{{ text }}</span>
    </slot>
  </div>
</template>

<script lang="ts" setup>
import Icon from './Icon.vue';

defineProps<{
  textOnly?: boolean;
  text?: string;
}>();
</script>

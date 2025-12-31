<template>
  <Collapsible
    v-if="collapsable"
    v-model:open="isOpen"
    class="w-full rounded-lg border border-border bg-card shadow-sm"
  >
    <CollapsibleTrigger
      v-if="title"
      class="flex w-full items-center gap-2 bg-muted/50 px-4 py-3 font-semibold text-card-foreground transition-colors hover:bg-muted/70"
    >
      <Icon
        name="chevron-right"
        class="size-5 transition-transform duration-200"
        :class="{ 'rotate-90 transform': isOpen }"
      />
      {{ title }}
    </CollapsibleTrigger>
    <CollapsibleContent class="w-full p-4 text-card-foreground">
      <slot />
    </CollapsibleContent>
  </Collapsible>
  <div v-else class="w-full rounded-lg border border-border bg-card shadow-sm">
    <div v-if="title" class="flex w-full gap-2 bg-muted/50 px-4 py-3 font-semibold text-card-foreground">
      {{ title }}
    </div>
    <div class="w-full p-4 text-card-foreground">
      <slot />
    </div>
  </div>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue';
import Icon from '~/components/atomic/Icon.vue';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '~/components/ui/collapsible';

const props = defineProps<{
  title?: string;
  collapsable?: boolean;
  collapsedByDefault?: boolean;
  open?: boolean;
}>();

const emit = defineEmits<{
  'update:open': [value: boolean];
}>();

// Initialize from prop or default
const isOpen = ref(props.open !== undefined ? props.open : !props.collapsedByDefault);

// Sync from parent when open prop changes
watch(
  () => props.open,
  (newVal) => {
    if (newVal !== undefined) {
      isOpen.value = newVal;
    }
  },
);

// Notify parent when internal state changes
watch(isOpen, (newVal) => {
  emit('update:open', newVal);
});
</script>

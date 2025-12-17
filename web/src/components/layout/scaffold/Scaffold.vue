<template>
  <div class="flex flex-1 flex-col">
    <Header
      :go-back="goBack"
      :enable-tabs="enableTabs"
      :search="search"
      :full-width="fullWidthHeader"
      @update:search="(value) => $emit('update:search', value)"
    >
      <template #title><slot name="title" /></template>
      <template v-if="$slots.titleSuffix" #titleSuffix><slot name="titleSuffix" /></template>
      <template v-if="$slots.headerActions" #headerActions><slot name="headerActions" /></template>
      <template v-if="$slots.tabActions" #tabActions><slot name="tabActions" /></template>
    </Header>
    <!-- When fluidContent is true, don't wrap in ScrollArea - let children handle their own scrolling -->
    <div v-if="fluidContent" class="flex-1 h-full w-full max-w-full overflow-hidden flex flex-col">
      <slot />
    </div>
    <ScrollArea v-else class="flex-1 h-full w-full max-w-full overflow-x-hidden">
      <Container>
        <slot />
      </Container>
    </ScrollArea>
  </div>
</template>

<script setup lang="ts">
import Container from '~/components/layout/Container.vue';
import { ScrollArea } from '~/components/ui/scroll-area';
import { useTabsProvider } from '~/compositions/useTabs';

import Header from './Header.vue';

const props = defineProps<{
  // Header
  goBack?: () => void;
  search?: string;
  fullWidthHeader?: boolean;

  // Tabs
  enableTabs?: boolean;

  // Content
  fluidContent?: boolean;
}>();

defineEmits<{
  (event: 'update:search', value: string): void;
}>();

if (props.enableTabs) {
  useTabsProvider();
}
</script>

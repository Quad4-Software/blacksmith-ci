<template>
  <header class="border-b border-border bg-background text-foreground shadow-sm">
    <div
      class="mx-auto w-full max-w-full px-3 py-2 sm:px-4 box-border overflow-x-hidden flex flex-col justify-center"
      :class="{ 'sm:max-w-6xl': !fullWidth }"
    >
      <div class="flex w-full flex-col gap-2 md:flex-row md:items-center md:justify-center md:gap-20 py-2">
        <div
          class="flex min-h-10 items-center"
          :class="{
            'md:flex-1': searchBoxPresent,
            'justify-between': !isPipelineDetail,
          }"
        >
          <div class="flex min-h-10 items-center">
            <IconButton
              v-if="goBack"
              icon="back"
              :title="$t('back')"
              class="md:display-unset mr-2 hidden h-8 w-8 shrink-0 md:justify-between"
              @click="goBack"
            />
            <h1 class="flex min-w-0 md:min-w-[120px] items-center gap-x-2 text-xl text-foreground">
              <slot name="title" />
              <span v-if="$slots.titleSuffix" class="text-sm font-normal text-muted-foreground">
                <slot name="titleSuffix" />
              </span>
            </h1>
          </div>
          <!-- Mobile header actions in same row as title (only for non-pipeline detail pages) -->
          <div v-if="$slots.headerActions && !isPipelineDetail" class="flex min-w-0 items-center gap-x-2 md:hidden">
            <slot name="headerActions" />
          </div>
        </div>
        <SearchInput
          v-if="searchBoxPresent"
          class="order-3 w-full grow md:order-0 md:w-auto"
          :aria-label="$t('search')"
          :placeholder="$t('search')"
          :model-value="search"
          @update:model-value="(value) => $emit('update:search', String(value))"
        />
        <!-- Desktop header actions in separate row -->
        <div
          v-if="$slots.headerActions"
          class="flex min-w-0 items-center gap-x-2 md:justify-end"
          :class="{
            'md:flex-1': searchBoxPresent,
            'hidden md:flex': !isPipelineDetail,
          }"
        >
          <slot name="headerActions" />
        </div>
      </div>

      <div v-if="enableTabs" class="flex flex-col md:flex-row md:items-center min-w-0 w-full mt-2 md:mt-0">
        <Tabs class="order-2 md:order-0 min-w-0 shrink-0 flex-none" />
        <div v-if="$slots.tabActions" class="order-1 md:order-1 flex-1 w-full overflow-x-hidden">
          <slot name="tabActions" />
        </div>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';

import IconButton from '~/components/atomic/IconButton.vue';
import SearchInput from '~/components/ui/input/SearchInput.vue';

import Tabs from './Tabs.vue';

const props = defineProps<{
  goBack?: () => void;
  enableTabs?: boolean;
  search?: string;
  fullWidth?: boolean;
}>();

defineEmits<{
  (event: 'update:search', query: string): void;
}>();

const route = useRoute();
const searchBoxPresent = computed(() => props.search !== undefined);

// Check if we're on a pipeline detail page
const isPipelineDetail = computed(() => {
  return route.name?.toString().startsWith('repo-pipeline');
});
</script>

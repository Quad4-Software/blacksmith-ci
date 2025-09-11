<template>
  <header
    class="border-wp-background-400 bg-wp-background-100 text-wp-text-100 shadow-xs dark:border-wp-background-100 dark:bg-wp-background-300"
    :class="{ 'md:px-4': fullWidth }"
  >
    <Container :full-width="fullWidth" class="py-0!">
      <div class="flex w-full flex-col gap-2 py-2 md:flex-row md:items-center md:justify-between md:gap-10">
        <div
          class="flex min-h-10 content-start items-center"
          :class="{
            'md:flex-1': searchBoxPresent,
            'justify-between': !isPipelineDetail,
          }"
        >
          <div class="flex min-h-10 content-start items-center">
            <IconButton
              v-if="goBack"
              icon="back"
              :title="$t('back')"
              class="md:display-unset mr-2 hidden h-8 w-8 shrink-0 md:justify-between"
              @click="goBack"
            />
            <h1 class="flex min-w-0 items-center gap-x-2 text-xl italic text-wp-text-100">
              <slot name="title" />
              <span v-if="$slots.titleSuffix" class="text-sm font-normal not-italic text-wp-text-alt-100">
                <slot name="titleSuffix" />
              </span>
            </h1>
          </div>
          <!-- Mobile header actions in same row as title (only for non-pipeline detail pages) -->
          <div v-if="$slots.headerActions && !isPipelineDetail" class="flex min-w-0 items-center gap-x-2 md:hidden">
            <slot name="headerActions" />
          </div>
        </div>
        <TextField
          v-if="searchBoxPresent"
          class="order-3 w-full grow md:order-none md:w-auto"
          :aria-label="$t('search')"
          :placeholder="$t('search')"
          :model-value="search"
          @update:model-value="(value: string) => $emit('update:search', value)"
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

      <div
        v-if="enableTabs"
        class="flex flex-col gap-0 md:gap-0 md:flex-row md:items-center md:justify-between md:py-0"
      >
        <Tabs class="order-2 md:order-none" />
        <div v-if="$slots.headerActions" class="flex flex-wrap content-start pb-2 md:justify-end gap-2">
          <slot name="tabActions" />
        </div>
      </div>
    </Container>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';

import IconButton from '~/components/atomic/IconButton.vue';
import TextField from '~/components/form/TextField.vue';
import Container from '~/components/layout/Container.vue';

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

<template>
  <div v-if="totalPages > 1" class="flex justify-center items-center gap-2 mt-6 pt-4 border-t border-border">
    <!-- Previous button -->
    <Button
      :disabled="currentPage === 1"
      :title="$t('pagination.previous')"
      @click="$emit('update:current-page', currentPage - 1)"
    >
      <Icon name="chevron-left" />
    </Button>

    <!-- First page -->
    <Button
      v-if="showFirstPage"
      :class="cn(currentPage === 1 && 'bg-primary text-primary-foreground')"
      @click="$emit('update:current-page', 1)"
    >
      {{ firstPageNumber }}
    </Button>

    <!-- Ellipsis after first page -->
    <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
    <span v-if="showFirstEllipsis" class="px-2 text-muted-foreground">...</span>

    <!-- Page numbers around current page -->
    <Button
      v-for="page in visiblePages"
      :key="page"
      :class="cn(currentPage === page && 'bg-primary text-primary-foreground')"
      @click="$emit('update:current-page', page)"
    >
      {{ page }}
    </Button>

    <!-- Ellipsis before last page -->
    <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
    <span v-if="showLastEllipsis" class="px-2 text-muted-foreground">...</span>

    <!-- Last page -->
    <Button
      v-if="showLastPage"
      :class="cn(currentPage === totalPages && 'bg-primary text-primary-foreground')"
      @click="$emit('update:current-page', totalPages)"
    >
      {{ totalPages }}
    </Button>

    <!-- Next button -->
    <Button
      :disabled="currentPage === totalPages"
      :title="$t('pagination.next')"
      @click="$emit('update:current-page', currentPage + 1)"
    >
      <Icon name="chevron-right" />
    </Button>

    <!-- Jump to Last button (only show if there are more pages beyond current) -->
    <Button v-if="hasMore" :is-loading="isJumpingToLast" :title="$t('pagination.last')" @click="$emit('jumpToLast')">
      <Icon name="chevron-right" />
      <Icon name="chevron-right" class="-ml-2" />
    </Button>
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue';

import Button from '~/components/atomic/Button.vue';
import Icon from '~/components/atomic/Icon.vue';
import { cn } from '~/lib/utils';

const props = defineProps<{
  currentPage: number;
  totalPages: number;
  hasMore?: boolean;
  isJumpingToLast?: boolean;
  maxVisible?: number; // Maximum number of page buttons to show (default: 5)
}>();

defineEmits<{
  'update:current-page': [page: number];
  jumpToLast: [];
}>();

const maxVisible = computed(() => props.maxVisible ?? 5);

// Constants for page numbers to avoid i18n raw text warnings
const firstPageNumber = 1;

// Calculate which pages to show
const visiblePages = computed(() => {
  const pages: number[] = [];
  const half = Math.floor(maxVisible.value / 2);
  let start = Math.max(2, props.currentPage - half);
  let end = Math.min(props.totalPages - 1, props.currentPage + half);

  // Adjust if we're near the beginning or end
  if (props.currentPage <= half + 1) {
    end = Math.min(props.totalPages - 1, maxVisible.value);
  } else if (props.currentPage >= props.totalPages - half) {
    start = Math.max(2, props.totalPages - maxVisible.value + 1);
  }

  for (let i = start; i <= end; i++) {
    pages.push(i);
  }

  return pages;
});

// Show first page if not in visible pages
const showFirstPage = computed(() => {
  return props.totalPages > 1;
});

// Show last page if not in visible pages
const showLastPage = computed(() => {
  return props.totalPages > 1 && !visiblePages.value.includes(props.totalPages);
});

// Show ellipsis after first page
const showFirstEllipsis = computed(() => {
  return visiblePages.value.length > 0 && visiblePages.value[0] > 2;
});

// Show ellipsis before last page
const showLastEllipsis = computed(() => {
  return visiblePages.value.length > 0 && visiblePages.value[visiblePages.value.length - 1] < props.totalPages - 1;
});
</script>

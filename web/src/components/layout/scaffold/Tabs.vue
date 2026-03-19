<template>
  <!-- Main tabs container -->
  <div ref="tabsRef" class="flex min-w-0 flex-auto gap-0 md:gap-1 pl-1 overflow-hidden">
    <router-link
      v-for="tab in visibleTabs"
      :key="tab.title"
      :to="tab.to"
      class="flex shrink-0 cursor-pointer items-center whitespace-nowrap border-b-2 border-transparent py-1 text-foreground transition-colors"
      :active-class="tab.matchChildren ? 'border-primary!' : ''"
      :exact-active-class="tab.matchChildren ? '' : 'border-primary!'"
    >
      <span
        class="flex w-full flex-row items-center justify-center gap-1 sm:gap-2 rounded-md px-1.5 py-1 sm:px-2 hover:bg-accent hover:text-accent-foreground transition-colors text-sm"
      >
        <Icon v-if="tab.icon" :name="tab.icon" :size="tab.iconSize" :class="tab.iconClass" class="shrink-0" />
        <span>{{ tab.title }}</span> <CountBadge v-if="tab.count" :value="tab.count" />
      </span>
    </router-link>
    <!-- Overflow dropdown -->
    <DropdownMenu v-if="hiddenTabs.length">
      <DropdownMenuTrigger
        class="flex shrink-0 cursor-pointer items-center whitespace-nowrap border-b-2 border-transparent py-1"
      >
        <span
          class="flex h-7 w-7 items-center justify-center rounded-md hover:bg-accent hover:text-accent-foreground transition-colors"
        >
          <Icon name="dots" />
        </span>
      </DropdownMenuTrigger>
      <DropdownMenuContent :align="visibleTabs.length === 0 ? 'start' : 'end'">
        <DropdownMenuItem v-for="tab in hiddenTabs" :key="tab.title" as-child>
          <router-link :to="tab.to" class="flex w-full cursor-pointer items-center gap-2 px-2 py-1.5">
            <Icon v-if="tab.icon" :name="tab.icon" :size="tab.iconSize" :class="tab.iconClass" class="shrink-0" />
            <span>{{ tab.title }}</span>
          </router-link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue';

import CountBadge from '~/components/atomic/CountBadge.vue';
import Icon from '~/components/atomic/Icon.vue';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import { useTabsClient } from '~/compositions/useTabs';

const { tabs } = useTabsClient();
const tabsRef = ref<HTMLElement | null>(null);
const visibleCount = ref(tabs.value.length);

const visibleTabs = computed(() => tabs.value.slice(0, visibleCount.value));
const hiddenTabs = computed(() => tabs.value.slice(visibleCount.value));

const updateVisibleItems = () => {
  visibleCount.value = tabs.value.length;

  nextTick(() => {
    const availableWidth = tabsRef.value!.clientWidth || 0;
    const moreButtonWidth = 32; // This need to match the width of the IconButton (w-8 = 32px)
    // gap-0 on mobile, md:gap-1 (4px) on desktop (768px+)
    const gapWidth = window.innerWidth >= 768 ? 4 : 0;
    let totalWidth = 0;

    // eslint-disable-next-line e18e/prefer-spread-syntax
    const items = Array.from(tabsRef.value!.children);

    // First, check if all tabs fit without the more button
    let totalWidthAllTabs = 0;
    for (let i = 0; i < items.length; i++) {
      const itemWidth = items[i].getBoundingClientRect().width;
      totalWidthAllTabs += itemWidth;
      if (i > 0) totalWidthAllTabs += gapWidth;
    }

    // If all tabs fit without overflow, show them all
    if (totalWidthAllTabs <= availableWidth) {
      visibleCount.value = tabs.value.length;
      return;
    }

    // Otherwise, find how many fit with the more button reserved
    for (let i = 0; i < items.length; i++) {
      const itemWidth = items[i].getBoundingClientRect().width;
      totalWidth += itemWidth;
      if (i > 0) totalWidth += gapWidth;

      if (totalWidth > availableWidth - (moreButtonWidth + gapWidth)) {
        visibleCount.value = Math.max(1, i); // Always show at least 1 tab
        return;
      }
    }

    visibleCount.value = tabs.value.length;
  });
};

// Re-calculate when tabs change
watch(tabs, () => {
  nextTick(updateVisibleItems);
});

onMounted(() => {
  const resizeObserver = new ResizeObserver(() => {
    requestAnimationFrame(updateVisibleItems);
  });

  if (tabsRef.value!) {
    resizeObserver.observe(tabsRef.value);
  }

  window.addEventListener('resize', updateVisibleItems);

  nextTick(updateVisibleItems);

  onUnmounted(() => {
    resizeObserver.disconnect();
    window.removeEventListener('resize', updateVisibleItems);
  });
});
</script>

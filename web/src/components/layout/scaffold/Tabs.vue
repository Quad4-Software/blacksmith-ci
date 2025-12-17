<template>
  <!-- Main tabs container -->
  <div ref="tabsRef" class="flex min-w-0 flex-auto gap-0 md:gap-1 pl-1">
    <router-link
      v-for="tab in visibleTabs"
      :key="tab.title"
      :to="tab.to"
      class="flex cursor-pointer items-center whitespace-nowrap border-b-2 border-transparent py-1 text-foreground transition-colors"
      :active-class="tab.matchChildren ? 'border-primary!' : ''"
      :exact-active-class="tab.matchChildren ? '' : 'border-primary!'"
    >
      <span
        class="flex w-full min-w-0 sm:min-w-20 flex-row items-center justify-center gap-1 sm:gap-2 rounded-md px-1.5 py-1 sm:px-2 hover:bg-accent hover:text-accent-foreground transition-colors text-sm"
      >
        <Icon v-if="tab.icon" :name="tab.icon" :size="tab.iconSize" :class="tab.iconClass" class="shrink-0" />
        <span>{{ tab.title }}</span> <CountBadge v-if="tab.count" :value="tab.count" />
      </span>
    </router-link>
    <!-- Overflow dropdown -->
    <DropdownMenu v-if="hiddenTabs.length">
      <DropdownMenuTrigger as-child>
        <Button variant="ghost" size="icon-sm" class="tabs-more-button border-b-2 border-transparent py-1">
          <Icon name="dots" />
        </Button>
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
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue';

import CountBadge from '~/components/atomic/CountBadge.vue';
import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
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
    const gapWidth = 4; // This need to match the gap between the tabs (md:gap-1 = 4px)
    let totalWidth = 0;

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

<template>
  <Tooltip v-if="tooltip">
    <TooltipTrigger as-child>
      <Button
        data-sidebar="trigger"
        data-slot="sidebar-trigger"
        variant="ghost"
        size="icon"
        :class="cn('h-8 w-8', props.class)"
        @click="toggleSidebar"
      >
        <PhSidebarSimple />
        <span class="sr-only" aria-hidden="true">Toggle Sidebar</span>
      </Button>
    </TooltipTrigger>
    <TooltipContent side="right" align="center">
      <slot name="tooltip">{{ tooltip }}</slot>
    </TooltipContent>
  </Tooltip>

  <Button
    v-else
    data-sidebar="trigger"
    data-slot="sidebar-trigger"
    variant="ghost"
    size="icon"
    :class="cn('h-8 w-8', props.class)"
    @click="toggleSidebar"
  >
    <PhSidebarSimple />
    <span class="sr-only" aria-hidden="true">Toggle Sidebar</span>
  </Button>
</template>

<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { PhSidebarSimple } from '@phosphor-icons/vue';
import { cn } from '~/lib/utils';
import { Button } from '~/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '~/components/ui/tooltip';
import { useSidebar } from './utils';

const props = defineProps<{
  class?: HTMLAttributes['class'];
  tooltip?: string;
}>();

const { toggleSidebar } = useSidebar();
</script>

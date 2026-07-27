<template>
  <TooltipPortal>
    <TooltipContent
      data-slot="tooltip-content"
      v-bind="{ ...forwarded, ...$attrs }"
      :class="
        cn(
          surfaceClass,
          'animate-in fade-in-0 zoom-in-95 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2 z-50 w-fit rounded-md px-3 py-1.5 text-xs text-balance',
          props.class,
        )
      "
    >
      <slot />

      <TooltipArrow :class="cn(arrowClass, 'z-50 size-2.5 translate-y-[calc(-50%_-_2px)] rotate-45 rounded-[2px]')" />
    </TooltipContent>
  </TooltipPortal>
</template>

<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core';
import { cn } from '~/lib/utils';
import type { TooltipContentEmits, TooltipContentProps } from 'reka-ui';
import { TooltipArrow, TooltipContent, TooltipPortal, useForwardPropsEmits } from 'reka-ui';
import type { HTMLAttributes } from 'vue';
import { computed } from 'vue';

defineOptions({
  inheritAttrs: false,
});

const props = withDefaults(
  defineProps<
    TooltipContentProps & {
      class?: HTMLAttributes['class'];
      // `primary` (default) uses the accent-colored surface for short labels.
      // `popover` uses a neutral card surface so colored or muted content (e.g. the
      // matrix variant grid) keeps good contrast in both light and dark themes.
      variant?: 'primary' | 'popover';
    }
  >(),
  {
    sideOffset: 4,
    variant: 'primary',
  },
);

const emits = defineEmits<TooltipContentEmits>();

const delegatedProps = reactiveOmit(props, 'class', 'variant');
const forwarded = useForwardPropsEmits(delegatedProps, emits);

const surfaceClass = computed(() =>
  props.variant === 'popover'
    ? 'bg-popover text-popover-foreground border border-border shadow-md'
    : 'bg-primary text-primary-foreground',
);

const arrowClass = computed(() =>
  props.variant === 'popover' ? 'bg-popover fill-popover' : 'bg-primary fill-primary',
);
</script>

<template>
  <component
    :is="to === undefined ? Button : httpLink ? 'a' : 'router-link'"
    v-bind="btnAttrs"
    :variant="variant"
    :size="size"
    :class="cn(buttonClass, props.class)"
    :title="title"
    :disabled="disabled || isLoading"
  >
    <Icon v-if="startIcon" :name="startIcon" :class="cn('size-4', { invisible: isLoading, 'mr-1': text })" />
    <span :class="{ invisible: isLoading }">{{ text || '' }}</span>
    <slot v-if="!text && !startIcon && !endIcon && !isLoading" />
    <Icon v-if="endIcon" :name="endIcon" :class="cn('size-4 ml-1', { invisible: isLoading })" />
    <div v-if="isLoading" class="absolute inset-0 flex items-center justify-center">
      <Icon name="spinner" class="size-5 animate-spin" />
    </div>
  </component>
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import type { RouteLocationRaw } from 'vue-router';
import { Button } from '~/components/ui/button';
import type { ButtonVariants } from '~/components/ui/button';
import type { IconNames } from '~/components/atomic/Icon.types';
import Icon from '~/components/atomic/Icon.vue';
import { cn } from '~/lib/utils';

const props = withDefaults(
  defineProps<{
    text?: string;
    title?: string;
    disabled?: boolean;
    to?: RouteLocationRaw;
    color?: 'blue' | 'default' | 'red' | 'gray' | 'green';
    variant?: ButtonVariants['variant'];
    size?: ButtonVariants['size'];
    startIcon?: IconNames;
    endIcon?: IconNames;
    isLoading?: boolean;
    class?: string;
  }>(),
  {
    text: undefined,
    title: undefined,
    to: undefined,
    color: 'gray',
    variant: undefined,
    size: 'default',
    startIcon: undefined,
    endIcon: undefined,
    class: undefined,
  },
);

const httpLink = computed(() => typeof props.to === 'string' && props.to.startsWith('http'));

// Map legacy color prop to shadcn-vue variants
const variant = computed<ButtonVariants['variant']>(() => {
  if (props.variant) return props.variant;

  switch (props.color) {
    case 'blue':
      return 'default';
    case 'red':
      return 'destructive';
    case 'default':
      return 'outline';
    case 'green':
      return 'default';
    case 'gray':
    default:
      return 'default'; // Use primary color for all buttons
  }
});

const buttonClass = computed(() => {
  const classes = ['relative'];

  // Add Crow-specific styling for certain colors
  if (props.color === 'default') {
    classes.push('border-orange-400 hover:border-orange-500 hover:bg-orange-50 dark:hover:bg-orange-950/20');
  } else if (props.color === 'green') {
    classes.push('bg-green-600 hover:bg-green-700 text-white');
  }

  return classes.join(' ');
});

const btnAttrs = computed(() => {
  if (props.to === undefined) {
    return { type: 'button' };
  }

  if (httpLink.value) {
    return { href: props.to, target: '_blank', rel: 'noopener noreferrer' };
  }

  return { to: props.to };
});
</script>

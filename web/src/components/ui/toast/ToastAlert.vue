<template>
  <div data-slot="toast-alert" :class="cn(toastAlertVariants({ variant }), props.class)" role="alert">
    <Icon :name="toastIcons[variant]" class="size-4 translate-y-0.5" />
    <div class="grid gap-0.5">
      <div data-slot="toast-title" class="font-medium leading-none tracking-tight">{{ title }}</div>
      <div
        v-if="description"
        data-slot="toast-description"
        class="text-sm text-muted-foreground"
        :class="{ 'text-destructive/90': variant === 'error' }"
      >
        <div v-if="isHtmlDescription" v-html="sanitizedDescription" />
        <template v-else>{{ description }}</template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { computed } from 'vue';
import DOMPurify from 'dompurify';

import Icon from '~/components/atomic/Icon.vue';
import { cn } from '~/lib/utils';

import type { ToastVariant } from '.';
import { toastAlertVariants, toastIcons } from '.';

const props = withDefaults(
  defineProps<{
    class?: HTMLAttributes['class'];
    variant?: ToastVariant;
    title: string;
    description?: string;
  }>(),
  {
    variant: 'default',
  },
);

// Detect HTML content using a proper regex that matches HTML tags
const isHtmlDescription = computed(() => {
  return props.description ? /<[a-z][\s\S]*>/i.test(props.description) : false;
});

// Sanitize HTML to prevent XSS attacks while allowing safe tags like <a>
const sanitizedDescription = computed(() => {
  if (!props.description) return '';
  return DOMPurify.sanitize(props.description, {
    ALLOWED_TAGS: ['a', 'strong', 'em', 'b', 'i', 'br'],
    ALLOWED_ATTR: ['href', 'class', 'target', 'rel'],
  });
});
</script>

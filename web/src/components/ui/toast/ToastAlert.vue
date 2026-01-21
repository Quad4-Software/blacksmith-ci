<template>
  <div
    data-slot="toast-alert"
    :class="cn(toastAlertVariants({ variant }), props.class, 'cursor-pointer')"
    role="alert"
    @click="handleClick"
  >
    <Icon :name="toastIcons[variant]" class="size-4 shrink-0" />
    <div class="grid gap-0.5 flex-1 min-w-0">
      <div data-slot="toast-title" class="font-medium leading-none tracking-tight">{{ title }}</div>
      <div
        v-if="description"
        data-slot="toast-description"
        class="text-sm text-muted-foreground break-words flex items-start gap-1"
        :class="{ 'text-destructive/90': variant === 'error' }"
      >
        <span class="flex-1">
          <span v-if="isHtmlDescription" v-html="sanitizedDescription" />
          <template v-else>{{ description }}</template>
        </span>
        <button
          type="button"
          class="shrink-0 p-0.5 rounded hover:bg-black/10 dark:hover:bg-white/10 transition-colors"
          :title="copied ? 'Copied!' : 'Copy'"
          @click.stop="copyToClipboard"
        >
          <Icon :name="copied ? 'lucide:check' : 'lucide:copy'" class="size-3" />
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { computed, ref } from 'vue';
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

const emit = defineEmits<{
  (e: 'closeToast'): void;
}>();

const copied = ref(false);

function handleClick() {
  emit('closeToast');
}

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

// Get plain text from description (strip HTML tags)
const plainTextDescription = computed(() => {
  if (!props.description) return '';
  // Strip HTML tags for copying
  return props.description.replace(/<[^>]*>/g, '');
});

async function copyToClipboard() {
  await navigator.clipboard.writeText(plainTextDescription.value);
  copied.value = true;
  setTimeout(() => {
    copied.value = false;
  }, 2000);
}
</script>

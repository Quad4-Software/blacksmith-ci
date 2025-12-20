<template><span v-html="contentHTML" /></template>

<script setup lang="ts">
import DOMPurify from 'dompurify';
import { marked } from 'marked';
import { computed } from 'vue';

const props = defineProps<{
  content: string;
}>();

// Only allow inline formatting tags - no block elements
const ALLOWED_TAGS = ['strong', 'em', 'code', 'b', 'i', 's', 'del'];

const contentHTML = computed<string>(() => {
  // parseInline only renders inline elements (no paragraphs, headers, lists, etc.)
  const dirtyHTML = marked.parseInline(props.content);
  return DOMPurify.sanitize(dirtyHTML as string, {
    ALLOWED_TAGS,
    ALLOWED_ATTR: [], // No attributes needed for inline formatting
  });
});
</script>

<style scoped>
:deep(code) {
  background-color: hsl(var(--muted));
  padding: 0.125rem 0.25rem;
  border-radius: 0.25rem;
  font-size: 0.875rem;
  font-family: ui-monospace, monospace;
}
</style>

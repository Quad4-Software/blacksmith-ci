<template>
  <div class="w-full space-y-6 p-2 md:p-8 max-w-6xl mx-auto">
    <div class="flex flex-col justify-center">
      <div class="flex items-center justify-between">
        <h1 class="flex items-center gap-1 text-xl text-card-foreground">
          {{ title }} <DocsLink v-if="docsUrl" :topic="title" :url="docsUrl" />
        </h1>
        <slot v-if="$slots.titleActions" name="titleActions" />
      </div>

      <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <p v-if="description" class="text-sm text-muted-foreground">{{ description }}</p>

        <div v-if="$slots.headerActions"><slot name="headerActions" /></div>
      </div>
      <slot name="headerEnd" />
    </div>
    <slot />
  </div>
</template>

<script setup lang="ts">
import DocsLink from '~/components/atomic/DocsLink.vue';

defineProps<{
  title: string;
  description?: string;
  docsUrl?: string;
}>();
</script>

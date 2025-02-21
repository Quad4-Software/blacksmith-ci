<template>
  <span class="inline-flex text-xs font-medium">
    <span
      class="flex items-center rounded-l-full border border-wp-custom-highlight-100 py-0.5 pl-2 pr-1 text-gray-300 text-wp-text-100"
      :class="{
        'rounded-r-full pr-2': value === undefined,
        'border-wp-error-100!': label === 'global secret',
        'border-wp-hint-warn-200!': label === 'organization secret',
      }"
    >
      {{ label }}
    </span>
    <span
      v-if="value !== undefined"
      class="flex items-center rounded-r-full border border-l-0 border-wp-custom-highlight-100 py-0.5 pl-1 pr-2"
    >
      {{ value }}
    </span>
  </span>
</template>

<script lang="ts" setup>
defineProps<{
  label: string;
  value?: string | number;
}>();
</script>

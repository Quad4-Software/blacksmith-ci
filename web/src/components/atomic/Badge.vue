<template>
  <span class="inline-flex text-xs font-medium">
    <span
      class="border-wp-custom-highlight-100 text-wp-text-100 flex items-center rounded-l-full border py-0.5 pr-1 pl-2 text-gray-300"
      :class="{
        'rounded-r-full pr-2': value === undefined,
        '!border-wp-error-100': label === 'global secret',
        '!border-wp-hint-warn-200': label === 'organization secret',
      }"
    >
      {{ label }}
    </span>
    <span
      v-if="value !== undefined"
      class="border-wp-custom-highlight-100 flex items-center rounded-r-full border border-l-0 py-0.5 pr-2 pl-1"
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

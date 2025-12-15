<template>
  <div class="space-y-1">
    <div class="text-sm font-medium text-muted-foreground">{{ label }}</div>
    <div v-if="$slots.default" class="text-sm">
      <slot />
    </div>
    <div v-else class="text-sm">
      {{ displayValue }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

const props = defineProps<{
  label: string;
  value?: string | number | boolean | null | undefined;
}>();

const { t } = useI18n();

const displayValue = computed(() => {
  if (props.value === null || props.value === undefined || props.value === '') {
    return t('repo.pipeline.metadata.fields.na');
  }
  return String(props.value);
});
</script>

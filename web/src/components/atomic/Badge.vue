<template>
  <span class="inline-flex text-xs font-medium">
    <Badge
      :variant="labelVariant"
      :class="
        cn('rounded-l-full border py-0.5 pl-2 pr-1', {
          'rounded-r-full pr-2': value === undefined,
          'border-destructive!': label === 'global secret',
          'border-yellow-500!': label === 'organization secret' || label === 'custom',
          'border-yellow-600!': label === 'type' && value === 'global',
        })
      "
    >
      {{ label }}
    </Badge>
    <Badge
      v-if="value !== undefined"
      :variant="valueVariant"
      :class="
        cn('rounded-r-full border border-l-0 py-0.5 pl-1 pr-2', {
          'border-yellow-500!':
            label === 'custom' || (label && (label.startsWith('org:') || label.startsWith('repo:'))),
          'border-yellow-600!': label === 'type' && value === 'global',
        })
      "
    >
      {{ value }}
    </Badge>
  </span>
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import { Badge } from '~/components/ui/badge';
import type { BadgeVariants } from '~/components/ui/badge';
import { cn } from '~/lib/utils';

const props = defineProps<{
  label: string;
  value?: string | number;
}>();

// Determine variant based on label type
const labelVariant = computed<BadgeVariants['variant']>(() => {
  if (props.label === 'global secret') return 'destructive';
  if (props.label === 'organization secret' || props.label === 'custom') return 'secondary';
  return 'outline';
});

const valueVariant = computed<BadgeVariants['variant']>(() => {
  if (props.label === 'type' && props.value === 'global') return 'secondary';
  if (props.label === 'custom' || props.label?.startsWith('org:') || props.label?.startsWith('repo:')) {
    return 'secondary';
  }
  return 'outline';
});
</script>

<template>
  <CheckboxRoot
    data-slot="checkbox"
    v-bind="delegatedProps"
    :model-value="props.checked"
    :class="
      cn(
        'peer bg-background border-input dark:border-muted-foreground/60 data-[state=checked]:bg-secondary data-[state=checked]:text-secondary-foreground data-[state=checked]:border-secondary focus-visible:border-secondary focus-visible:ring-secondary/50 aria-invalid:ring-destructive/20 aria-invalid:border-destructive size-4 shrink-0 rounded-[4px] border shadow-xs transition-all outline-none focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-50',
        props.class,
      )
    "
    @update:model-value="(val: boolean | 'indeterminate') => emits('update:checked', val)"
  >
    <CheckboxIndicator data-slot="checkbox-indicator" class="grid place-content-center text-current transition-none">
      <slot>
        <Check class="size-3.5" />
      </slot>
    </CheckboxIndicator>
  </CheckboxRoot>
</template>

<script setup lang="ts">
import type { CheckboxRootProps } from 'reka-ui';
import type { HTMLAttributes } from 'vue';
import { reactiveOmit } from '@vueuse/core';
import { Check } from 'lucide-vue-next';
import { CheckboxIndicator, CheckboxRoot } from 'reka-ui';
import { cn } from '~/lib/utils';

const props = defineProps<CheckboxRootProps & { class?: HTMLAttributes['class']; checked?: boolean }>();
const emits = defineEmits<{
  'update:checked': [value: boolean | 'indeterminate'];
}>();

const delegatedProps = reactiveOmit(props, 'class', 'checked');
</script>

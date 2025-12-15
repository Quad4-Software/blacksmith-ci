<template>
  <div :class="cn('relative', $attrs.class as string)">
    <Input
      :model-value="modelValue"
      :placeholder="placeholder"
      class="pr-8"
      v-bind="{ ...$attrs, class: undefined }"
      @update:model-value="handleUpdate"
    />
    <Button
      v-if="modelValue && modelValue.length > 0"
      variant="ghost"
      size="icon-sm"
      class="absolute right-0 top-1/2 -translate-y-1/2 h-full hover:bg-transparent"
      @click="handleClear"
    >
      <Icon name="close" class="h-4 w-4 text-muted-foreground hover:text-foreground transition-colors" />
    </Button>
  </div>
</template>

<script lang="ts" setup>
import { cn } from '~/lib/utils';
import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import Input from '~/components/ui/input/Input.vue';

interface Props {
  modelValue?: string;
  placeholder?: string;
}

interface Emits {
  (event: 'update:modelValue', value: string): void;
}

defineProps<Props>();
const emit = defineEmits<Emits>();

function handleUpdate(value: string | number) {
  emit('update:modelValue', String(value));
}

function handleClear() {
  emit('update:modelValue', '');
}
</script>

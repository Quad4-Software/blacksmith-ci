<template><Input v-model="innerValue" :placeholder="placeholder" type="number" /></template>

<script lang="ts" setup>
import { computed, toRef } from 'vue';

import { Input } from '~/components/ui/input';

const props = defineProps<{
  modelValue: number;
  placeholder?: string;
}>();

const emit = defineEmits<{
  (event: 'update:modelValue', value: number): void;
}>();

const modelValue = toRef(props, 'modelValue');
const innerValue = computed({
  get: () => modelValue.value.toString(),
  set: (value) => {
    emit('update:modelValue', Number.parseFloat(value));
  },
});
</script>

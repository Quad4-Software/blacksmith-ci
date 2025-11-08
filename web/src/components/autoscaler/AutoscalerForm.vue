<template>
  <form @submit.prevent="$emit('save')">
    <InputField v-slot="{ id }" :label="$t('admin.settings.autoscalers.name.name')">
      <TextField
        :id="id"
        v-model="autoscaler.name"
        :placeholder="$t('admin.settings.autoscalers.name.placeholder')"
        required
      />
    </InputField>

    <div class="flex gap-2">
      <Button type="button" color="gray" :text="$t('cancel')" @click="$emit('cancel')" />
      <Button :is-loading="isSaving" type="submit" color="default" :text="$t('admin.settings.autoscalers.add')" />
    </div>
  </form>
</template>

<script lang="ts" setup>
import { computed } from 'vue';

import Button from '~/components/atomic/Button.vue';
import InputField from '~/components/form/InputField.vue';
import TextField from '~/components/form/TextField.vue';
import type { Autoscaler } from '~/lib/api/types';

const props = defineProps<{
  modelValue: Partial<Autoscaler>;
  isSaving: boolean;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', value: Partial<Autoscaler>): void;
  (e: 'save'): void;
  (e: 'cancel'): void;
}>();

const autoscaler = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
});
</script>

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
      <Button type="button" variant="outline" @click="$emit('cancel')">{{ $t('cancel') }}</Button>
      <Button
        type="submit"
        variant="outline"
        class="relative border-orange-400 hover:border-orange-500 hover:bg-orange-50 dark:hover:bg-orange-950/20"
        :disabled="isSaving"
      >
        <Icon v-if="isSaving" name="spinner" class="size-5 animate-spin" />
        <template v-else>{{ $t('admin.settings.autoscalers.add') }}</template>
      </Button>
    </div>
  </form>
</template>

<script lang="ts" setup>
import { computed } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
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

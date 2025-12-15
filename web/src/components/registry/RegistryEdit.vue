<template>
  <div v-if="innerValue">
    <form @submit.prevent="save">
      <FieldGroup>
        <Field>
          <FieldLabel for="registry-address">{{ $t('registries.address.address') }}</FieldLabel>
          <!-- TODO: check input field Address is a valid address -->
          <Input
            id="registry-address"
            v-model="innerValue.address"
            :placeholder="$t('registries.address.desc')"
            required
            :disabled="isEditing || isReadOnly"
          />
        </Field>

        <Field>
          <FieldLabel for="registry-username">{{ $t('username') }}</FieldLabel>
          <Input
            id="registry-username"
            v-model="innerValue.username"
            :placeholder="$t('username')"
            required
            :disabled="isReadOnly"
          />
        </Field>

        <Field v-if="!isReadOnly">
          <FieldLabel for="registry-password">{{ $t('password') }}</FieldLabel>
          <Input
            id="registry-password"
            v-model="innerValue.password"
            :placeholder="$t('password')"
            :required="!isEditing"
          />
        </Field>
      </FieldGroup>

      <div v-if="!isReadOnly" class="mt-6 flex gap-2">
        <Button type="button" variant="outline" @click="$emit('cancel')">
          {{ $t('cancel') }}
        </Button>
        <Button type="submit" :disabled="isSaving">
          <Icon v-if="isSaving" name="spinner" class="size-4 animate-spin mr-2" />
          {{ isEditing ? $t('registries.save') : $t('registries.add') }}
        </Button>
      </div>
    </form>
  </div>
</template>

<script lang="ts" setup>
import { computed, toRef } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '~/components/ui/field';
import { Input } from '~/components/ui/input';
import type { Registry } from '~/lib/api/types';

const props = defineProps<{
  modelValue: Partial<Registry>;
  isSaving: boolean;
}>();

const emit = defineEmits<{
  (event: 'update:modelValue', value: Partial<Registry> | undefined): void;
  (event: 'save', value: Partial<Registry>): void;
  (event: 'cancel'): void;
}>();

const modelValue = toRef(props, 'modelValue');
const innerValue = computed({
  get: () => modelValue.value,
  set: (value) => {
    emit('update:modelValue', value);
  },
});
const isEditing = computed(() => !!innerValue.value?.id);
const isReadOnly = computed(() => !!innerValue.value?.readonly);

function save() {
  if (!innerValue.value) {
    return;
  }

  emit('save', innerValue.value);
}
</script>

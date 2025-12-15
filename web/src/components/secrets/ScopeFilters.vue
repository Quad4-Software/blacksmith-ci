<template>
  <div class="flex items-center gap-4">
    <div class="flex items-center gap-2">
      <Switch id="user-scope" :checked="modelValue.user" @update:checked="updateUser" />
      <Label for="user-scope" class="text-sm font-medium cursor-pointer whitespace-nowrap">
        {{ $t('scope.user') }}
      </Label>
    </div>
    <div class="flex items-center gap-2">
      <Switch id="org-scope" :checked="modelValue.org" @update:checked="updateOrg" />
      <Label for="org-scope" class="text-sm font-medium cursor-pointer whitespace-nowrap">
        {{ $t('scope.org') }}
      </Label>
    </div>
    <div class="flex items-center gap-2">
      <Switch id="global-scope" :checked="modelValue.global" @update:checked="updateGlobal" />
      <Label for="global-scope" class="text-sm font-medium cursor-pointer whitespace-nowrap">
        {{ $t('scope.global') }}
      </Label>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { Label } from '~/components/ui/label';
import { Switch } from '~/components/ui/switch';
import type { ScopeFilters } from '~/compositions/useUserSecretsRegistries';

const props = defineProps<{
  modelValue: ScopeFilters;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', value: ScopeFilters): void;
}>();

function updateUser(value: boolean) {
  emit('update:modelValue', { ...props.modelValue, user: value });
}

function updateOrg(value: boolean) {
  emit('update:modelValue', { ...props.modelValue, org: value });
}

function updateGlobal(value: boolean) {
  emit('update:modelValue', { ...props.modelValue, global: value });
}
</script>

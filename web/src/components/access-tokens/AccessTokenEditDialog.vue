<template>
  <Dialog v-model:open="isOpen">
    <DialogContent class="sm:max-w-[500px] max-h-[90vh] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>{{ $t('access_tokens.edit') }}</DialogTitle>
        <DialogDescription>
          {{ $t('access_tokens.edit_description') }}
        </DialogDescription>
      </DialogHeader>

      <form class="space-y-6 mt-4" @submit.prevent="onSubmit">
        <!-- Name Field -->
        <div class="space-y-3">
          <label class="text-sm font-medium">{{ $t('access_tokens.name') }}</label>
          <Input v-model="form.name" :placeholder="$t('access_tokens.name_placeholder')" />
        </div>

        <!-- Scopes Field -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="text-sm font-medium">{{ $t('access_tokens.scopes') }}</label>
            <Button type="button" variant="link" size="sm" class="h-auto p-0 text-xs" @click="toggleAllScopes">
              {{ allScopesSelected ? $t('access_tokens.deselect_all') : $t('access_tokens.select_all') }}
            </Button>
          </div>
          <p class="text-xs text-muted-foreground">{{ $t('access_tokens.scopes_description') }}</p>

          <div class="space-y-4 mt-3">
            <!-- User Scopes -->
            <div class="space-y-3">
              <p class="text-xs font-medium text-muted-foreground uppercase">{{ $t('access_tokens.user_scopes') }}</p>
              <div v-for="scope in USER_SCOPES" :key="scope" class="flex items-start gap-3">
                <Checkbox
                  :id="`edit-scope-${scope}`"
                  :checked="form.scopes.includes(scope)"
                  @update:checked="(checked: boolean | 'indeterminate') => toggleScope(scope, checked)"
                />
                <div class="grid gap-0.5 leading-none">
                  <label :for="`edit-scope-${scope}`" class="text-sm font-medium cursor-pointer">
                    {{ scope }}
                  </label>
                  <p class="text-xs text-muted-foreground">
                    {{ SCOPE_DESCRIPTIONS[scope] }}
                  </p>
                </div>
              </div>
            </div>

            <!-- Admin Scopes (by instance permission) -->
            <div v-if="adminScopes.length > 0" class="space-y-3">
              <p class="text-xs font-medium text-muted-foreground uppercase">{{ $t('access_tokens.admin_scopes') }}</p>
              <div v-for="scope in adminScopes" :key="scope" class="flex items-start gap-3">
                <Checkbox
                  :id="`edit-scope-${scope}`"
                  :checked="form.scopes.includes(scope)"
                  @update:checked="(checked: boolean | 'indeterminate') => toggleScope(scope, checked)"
                />
                <div class="grid gap-0.5 leading-none">
                  <label :for="`edit-scope-${scope}`" class="text-sm font-medium cursor-pointer">
                    {{ scope }}
                  </label>
                  <p class="text-xs text-muted-foreground">
                    {{ SCOPE_DESCRIPTIONS[scope] }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="flex justify-end gap-2 pt-4">
          <Button type="button" variant="outline" @click="isOpen = false">
            {{ $t('cancel') }}
          </Button>
          <Button type="submit" :disabled="isSaving || !isFormValid">
            <Icon v-if="isSaving" name="spinner" class="size-4 animate-spin mr-2" />
            {{ $t('save') }}
          </Button>
        </div>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import usePermissions, { adminScopesFor } from '~/compositions/usePermissions';
import type { AccessToken, AccessTokenScope, UpdateAccessTokenRequest } from '~/lib/api/types';
import { SCOPE_DESCRIPTIONS, USER_SCOPES } from '~/lib/api/types';

const props = withDefaults(
  defineProps<{
    token?: AccessToken | null;
    isSaving?: boolean;
    open?: boolean;
  }>(),
  {
    token: null,
    isSaving: false,
  },
);

const emit = defineEmits<{
  (event: 'save', value: UpdateAccessTokenRequest): void;
  (event: 'update:open', value: boolean): void;
}>();

const { can } = usePermissions();

// The admin scopes follow the server's rule in GetAvailableScopes, not the admin flag.
const adminScopes = computed(() => adminScopesFor(can));

const isOpen = computed({
  get: () => props.open ?? false,
  set: (value) => {
    emit('update:open', value);
  },
});

const form = ref<{
  name: string;
  scopes: AccessTokenScope[];
}>({
  name: '',
  scopes: [],
});

const isFormValid = computed(() => {
  return form.value.name.trim().length > 0 && form.value.scopes.length > 0;
});

const availableScopes = computed(() => {
  return [...USER_SCOPES, ...adminScopes.value];
});

const allScopesSelected = computed(() => {
  return availableScopes.value.every((scope) => form.value.scopes.includes(scope));
});

function toggleAllScopes() {
  if (allScopesSelected.value) {
    form.value.scopes = [];
  } else {
    form.value.scopes = [...availableScopes.value];
  }
}

function toggleScope(scope: AccessTokenScope, checked: boolean | 'indeterminate') {
  if (checked === true) {
    if (!form.value.scopes.includes(scope)) {
      form.value.scopes.push(scope);
    }
  } else {
    form.value.scopes = form.value.scopes.filter((s) => s !== scope);
  }
}

function onSubmit() {
  if (!isFormValid.value) return;

  const request: UpdateAccessTokenRequest = {
    name: form.value.name.trim(),
    scopes: form.value.scopes,
  };

  emit('save', request);
}

// Initialize form when token changes or dialog opens
watch(
  [() => props.token, isOpen],
  ([token, open]) => {
    if (open && token) {
      form.value = {
        name: token.name || '',
        scopes: (token.scopes || []) as AccessTokenScope[],
      };
    }
  },
  { immediate: true },
);

// Reset form when dialog closes
watch(isOpen, (open) => {
  if (!open) {
    form.value = { name: '', scopes: [] };
  }
});
</script>

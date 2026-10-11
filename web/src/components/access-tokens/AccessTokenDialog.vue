<template>
  <Dialog v-model:open="isOpen">
    <DialogTrigger v-if="!hideTrigger" as-child>
      <slot name="trigger">
        <Button size="sm">
          <Icon name="plus" class="mr-2 h-4 w-4" />
          {{ $t('access_tokens.create') }}
        </Button>
      </slot>
    </DialogTrigger>
    <DialogContent class="sm:max-w-[500px] max-h-[90vh] overflow-y-auto">
      <!-- Token Created View -->
      <template v-if="createdToken">
        <DialogHeader>
          <DialogTitle>{{ $t('access_tokens.created_title') }}</DialogTitle>
          <DialogDescription>
            {{ $t('access_tokens.created_description') }}
          </DialogDescription>
        </DialogHeader>

        <Alert class="mt-4 border-amber-500/50 text-amber-700 dark:text-amber-400 [&>svg]:text-amber-500">
          <Icon name="alert" class="h-4 w-4" />
          <AlertDescription>
            {{ $t('access_tokens.copy_warning') }}
          </AlertDescription>
        </Alert>

        <div class="mt-4 space-y-2">
          <label class="text-sm font-medium">{{ $t('access_tokens.your_token') }}</label>
          <div class="flex gap-2">
            <Input :model-value="createdToken.token" readonly class="font-mono text-xs" />
            <Button variant="outline" size="icon" @click="copyToken">
              <Icon :name="copied ? 'ph:check' : 'ph:copy'" class="h-4 w-4" />
            </Button>
          </div>
        </div>

        <div class="flex justify-end gap-2 pt-4">
          <Button @click="handleClose">
            {{ $t('done') }}
          </Button>
        </div>
      </template>

      <!-- Create Form View -->
      <template v-else>
        <DialogHeader>
          <DialogTitle>{{ $t('access_tokens.create') }}</DialogTitle>
          <DialogDescription>
            {{ $t('access_tokens.create_description') }}
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
                    :id="`scope-${scope}`"
                    :checked="form.scopes.includes(scope)"
                    @update:checked="(checked: boolean | 'indeterminate') => toggleScope(scope, checked)"
                  />
                  <div class="grid gap-0.5 leading-none">
                    <label :for="`scope-${scope}`" class="text-sm font-medium cursor-pointer">
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
                <p class="text-xs font-medium text-muted-foreground uppercase">
                  {{ $t('access_tokens.admin_scopes') }}
                </p>
                <div v-for="scope in adminScopes" :key="scope" class="flex items-start gap-3">
                  <Checkbox
                    :id="`scope-${scope}`"
                    :checked="form.scopes.includes(scope)"
                    @update:checked="(checked: boolean | 'indeterminate') => toggleScope(scope, checked)"
                  />
                  <div class="grid gap-0.5 leading-none">
                    <label :for="`scope-${scope}`" class="text-sm font-medium cursor-pointer">
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

          <!-- Expiration Field -->
          <div class="space-y-3">
            <label class="text-sm font-medium">{{ $t('access_tokens.expiration') }}</label>
            <Select v-model="expirationOption">
              <SelectTrigger>
                <SelectValue :placeholder="$t('access_tokens.expiration_placeholder')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="never">{{ $t('access_tokens.expiration_never') }}</SelectItem>
                <SelectItem value="30d">{{ $t('access_tokens.expiration_30d') }}</SelectItem>
                <SelectItem value="60d">{{ $t('access_tokens.expiration_60d') }}</SelectItem>
                <SelectItem value="90d">{{ $t('access_tokens.expiration_90d') }}</SelectItem>
                <SelectItem value="1y">{{ $t('access_tokens.expiration_1y') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="flex justify-end gap-2 pt-4">
            <Button type="button" variant="outline" @click="isOpen = false">
              {{ $t('cancel') }}
            </Button>
            <Button type="submit" :disabled="isSaving || !isFormValid">
              <Icon v-if="isSaving" name="spinner" class="size-4 animate-spin mr-2" />
              {{ $t('access_tokens.create') }}
            </Button>
          </div>
        </form>
      </template>
    </DialogContent>
  </Dialog>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import { Alert, AlertDescription } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import usePermissions, { adminScopesFor } from '~/compositions/usePermissions';
import type { AccessTokenScope, AccessTokenWithSecret, CreateAccessTokenRequest } from '~/lib/api/types';
import { SCOPE_DESCRIPTIONS, USER_SCOPES } from '~/lib/api/types';

const props = withDefaults(
  defineProps<{
    createdToken?: AccessTokenWithSecret | null;
    isSaving?: boolean;
    open?: boolean;
    hideTrigger?: boolean;
  }>(),
  {
    createdToken: null,
    isSaving: false,
    hideTrigger: false,
  },
);

const emit = defineEmits<{
  (event: 'save', value: CreateAccessTokenRequest): void;
  (event: 'cancel'): void;
  (event: 'update:open', value: boolean): void;
}>();

const { can } = usePermissions();

// The admin scopes follow the server's rule in GetAvailableScopes, not the admin flag.
const adminScopes = computed(() => adminScopesFor(can));

const isOpen = computed({
  get: () => props.open ?? false,
  set: (value) => {
    emit('update:open', value);
    if (!value) {
      emit('cancel');
    }
  },
});

const form = ref<{
  name: string;
  scopes: AccessTokenScope[];
}>({
  name: '',
  scopes: [],
});

const expirationOption = ref<string>('never');
const copied = ref(false);

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

function toggleScope(scope: AccessTokenScope, checked: boolean | 'indeterminate' | undefined) {
  if (checked === true) {
    if (!form.value.scopes.includes(scope)) {
      form.value.scopes.push(scope);
    }
  } else {
    form.value.scopes = form.value.scopes.filter((s) => s !== scope);
  }
}

function getExpirationTimestamp(): number {
  if (expirationOption.value === 'never') {
    return 0;
  }

  const now = Math.floor(Date.now() / 1000);
  const day = 24 * 60 * 60;

  switch (expirationOption.value) {
    case '30d':
      return now + 30 * day;
    case '60d':
      return now + 60 * day;
    case '90d':
      return now + 90 * day;
    case '1y':
      return now + 365 * day;
    default:
      return 0;
  }
}

function onSubmit() {
  if (!isFormValid.value) return;

  const request: CreateAccessTokenRequest = {
    name: form.value.name.trim(),
    scopes: form.value.scopes,
    expires_at: getExpirationTimestamp(),
  };

  emit('save', request);
}

async function copyToken() {
  if (!props.createdToken?.token) return;

  try {
    await navigator.clipboard.writeText(props.createdToken.token);
    copied.value = true;
    setTimeout(() => {
      copied.value = false;
    }, 2000);
  } catch {
    // Failed to copy
  }
}

function handleClose() {
  isOpen.value = false;
}

// Reset form when dialog closes
watch(isOpen, (open) => {
  if (!open) {
    form.value = { name: '', scopes: [] };
    expirationOption.value = 'never';
    copied.value = false;
  }
});
</script>

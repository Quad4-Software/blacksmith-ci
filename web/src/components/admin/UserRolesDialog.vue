<template>
  <!-- Escape, an overlay click and the close button cannot dismiss the dialog while the save runs. -->
  <Dialog :open="open" @update:open="(value) => !isSaving && emit('update:open', value)">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{{ $t('admin.settings.users.edit_roles') }}</DialogTitle>
        <DialogDescription>{{ user?.name }}</DialogDescription>
      </DialogHeader>
      <form @submit.prevent="save">
        <div class="space-y-4">
          <div v-for="role in roles" :key="role.name" class="flex items-start gap-3">
            <Checkbox
              :id="`role-${role.name}`"
              class="mt-0.5"
              :checked="selected.includes(role.name)"
              :disabled="role.locked || isSaving"
              @update:checked="(value) => toggle(role.name, value === true)"
            />
            <div class="grid gap-1">
              <Label :for="`role-${role.name}`">{{ role.label }}</Label>
              <p class="text-sm text-muted-foreground">{{ role.description }}</p>
              <p v-if="role.locked" class="text-xs text-muted-foreground">
                {{ $t('admin.settings.users.role_managed') }}
              </p>
            </div>
          </div>
          <p v-if="error" role="alert" class="text-sm text-destructive">{{ error }}</p>
        </div>
        <DialogFooter class="mt-6">
          <Button variant="outline" type="button" :disabled="isSaving" @click="emit('update:open', false)">
            {{ $t('cancel') }}
          </Button>
          <Button type="submit" :disabled="isSaving">
            <Icon v-if="isSaving" name="loading" class="h-4 w-4 mr-2 animate-spin" />
            {{ $t('save') }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog';
import { Label } from '~/components/ui/label';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useConfig from '~/compositions/useConfig';
import useNotifications from '~/compositions/useNotifications';
import useRoleTexts from '~/compositions/useRoleTexts';
import type { UserEntity } from '~/lib/api/types';
import { instanceRoles } from '~/lib/api/types';

const props = defineProps<{
  open: boolean;
  user: UserEntity | null;
}>();

const emit = defineEmits<{
  'update:open': [value: boolean];
  saved: [];
}>();

const { t } = useI18n();
const apiClient = useApiClient();
const notifications = useNotifications();

// Roles set by an identity provider cannot be newly assigned here.
// One the user already holds can be kept or removed, so it stays editable.
const managedRoles = useConfig().auth.managed_roles ?? [];

const roleTexts = useRoleTexts();

// A managed role is locked unless the user holds it.
function isLocked(role: string) {
  return managedRoles.includes(role) && !(props.user?.roles ?? []).includes(role);
}

const roles = computed(() => instanceRoles.map((name) => ({ name, locked: isLocked(name), ...roleTexts.value[name] })));

const selected = ref<string[]>([]);
const error = ref('');

watch(
  () => [props.open, props.user] as const,
  ([open, user]) => {
    if (open) {
      selected.value = [...(user?.roles ?? [])];
      error.value = '';
    }
  },
  { immediate: true },
);

function toggle(role: string, checked: boolean) {
  selected.value = checked ? [...selected.value, role] : selected.value.filter((r) => r !== role);
}

const { doSubmit: save, isLoading: isSaving } = useAsyncAction(
  async () => {
    const user = props.user;
    if (!user) return;
    error.value = '';

    // PATCH replaces these fields from the body, so send the current values along with the roles.
    await apiClient.updateUser({
      login: user.name,
      email: user.email ?? '',
      avatar_url: user.avatar ?? '',
      admin: !!user.admin,
      description: user.description ?? '',
      timezone: user.timezone ?? '',
      // The role set replaces the user's roles, so held managed roles that stay checked are sent too.
      roles: instanceRoles.filter((r) => !isLocked(r) && selected.value.includes(r)),
    });

    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.users.roles_saved'),
      type: 'success',
    });
    emit('saved');
    emit('update:open', false);
  },
  (e: unknown) => {
    error.value = e instanceof Error ? e.message : String(e);
  },
);
</script>

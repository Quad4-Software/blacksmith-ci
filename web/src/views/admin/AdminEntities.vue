<template>
  <Settings :title="$t('admin.settings.entities.title')" :description="$t('admin.settings.entities.desc')">
    <template #headerActions>
      <Button @click="showAddUser">
        <Icon name="plus" class="h-4 w-4 mr-2" />
        {{ $t('admin.settings.users.add') }}
      </Button>
    </template>

    <div class="space-y-4">
      <EntityTable
        :entities="data"
        :is-deleting="isDeleting"
        :search-term="searchTerm"
        @edit-user="handleEditUser"
        @delete="handleDelete"
        @bulk-delete="handleBulkDelete"
        @bulk-toggle-admin="handleBulkToggleAdmin"
        @update:search-term="searchTerm = $event"
      />

      <!-- Pagination Controls -->
      <div v-if="data && data.length > 0" class="flex items-center justify-center gap-2">
        <Button variant="outline" size="sm" :disabled="page === 1 || loading" @click="previousPage">
          <Icon name="chevron-left" class="h-4 w-4" />
          {{ $t('admin.settings.entities.previous') }}
        </Button>
        <span class="text-sm text-muted-foreground"> {{ $t('admin.settings.entities.page') }} {{ page }} </span>
        <Button variant="outline" size="sm" :disabled="!hasMore || loading" @click="nextPage">
          {{ $t('admin.settings.entities.next') }}
          <Icon name="chevron-right" class="h-4 w-4" />
        </Button>
      </div>
    </div>

    <!-- User Edit Dialog -->
    <Dialog v-model:open="isEditDialogOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {{ isEditingUser ? $t('admin.settings.users.edit_user') : $t('admin.settings.users.add') }}
          </DialogTitle>
        </DialogHeader>
        <form @submit.prevent="saveUser">
          <div class="space-y-4">
            <InputField v-slot="{ id }" :label="$t('admin.settings.users.login')">
              <Input :id="id" v-model="selectedUser.login" :disabled="isEditingUser" />
            </InputField>
            <InputField v-slot="{ id }" :label="$t('admin.settings.users.email')">
              <Input :id="id" v-model="selectedUser.email" />
            </InputField>
            <InputField v-slot="{ id }" :label="$t('admin.settings.users.avatar_url')">
              <div class="flex gap-2">
                <img v-if="selectedUser.avatar_url" class="h-8 w-8 rounded-md" :src="selectedUser.avatar_url" />
                <Input :id="id" v-model="selectedUser.avatar_url" />
              </div>
            </InputField>
            <InputField :label="$t('admin.settings.users.admin.admin')">
              <Checkbox
                :model-value="selectedUser.admin || false"
                :label="$t('admin.settings.users.admin.placeholder')"
                @update:model-value="selectedUser.admin = $event"
              />
            </InputField>
          </div>
          <DialogFooter class="mt-6">
            <Button variant="outline" type="button" @click="isEditDialogOpen = false">
              {{ $t('cancel') }}
            </Button>
            <Button :disabled="isSaving" type="submit">
              <Icon v-if="isSaving" name="loading" class="h-4 w-4 mr-2 animate-spin" />
              {{ isEditingUser ? $t('admin.settings.users.save') : $t('admin.settings.users.add') }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  </Settings>
</template>

<script lang="ts" setup>
import { watchDebounced } from '@vueuse/core';
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import EntityTable from '~/components/admin/EntityTable.vue';
import Checkbox from '~/components/form/Checkbox.vue';
import InputField from '~/components/form/InputField.vue';
import Settings from '~/components/layout/Settings.vue';
import { Button } from '~/components/ui/button';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import useNotifications from '~/compositions/useNotifications';
import { usePaginationWithPages } from '~/compositions/usePaginate';
import type { Entity, User } from '~/lib/api/types';
import { isUserEntity } from '~/lib/api/types';

const apiClient = useApiClient();
const notifications = useNotifications();
const { t } = useI18n();

const searchTerm = ref('');
const isEditDialogOpen = ref(false);
const selectedUser = ref<Partial<User>>({});
const isEditingUser = computed(() => !!selectedUser.value.id);

async function loadEntities(page: number): Promise<Entity[] | null> {
  return apiClient.getEntities({
    page,
    perPage: 15,
    search: searchTerm.value,
  });
}

const { page, data, hasMore, loading, nextPage, previousPage, resetPage } = usePaginationWithPages(loadEntities, {
  pageSize: 15,
});

// Reload when search changes
watchDebounced(
  searchTerm,
  async () => {
    await resetPage();
  },
  { debounce: 300 },
);

function handleEditUser(entity: Entity) {
  if (!isUserEntity(entity)) return;

  selectedUser.value = {
    id: entity.id,
    login: entity.name,
    email: entity.email || '',
    avatar_url: entity.avatar || '',
    admin: entity.admin || false,
    forge_id: entity.forge_id,
  };
  isEditDialogOpen.value = true;
}

function showAddUser() {
  selectedUser.value = { login: '', email: '', avatar_url: '', admin: false };
  isEditDialogOpen.value = true;
}

const { doSubmit: saveUser, isLoading: isSaving } = useAsyncAction(async () => {
  if (!selectedUser.value) {
    throw new Error("Unexpected: Can't get user");
  }

  if (isEditingUser.value) {
    await apiClient.updateUser(selectedUser.value);
    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.users.notify_text_saved'),
      type: 'success',
    });
  } else {
    await apiClient.createUser(selectedUser.value);
    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.users.notify_text_created'),
      type: 'success',
    });
  }

  isEditDialogOpen.value = false;
  selectedUser.value = {};
  await resetPage();
});

const { doSubmit: handleDelete, isLoading: isDeleting } = useAsyncAction(async (entity: Entity) => {
  if (entity.entity_type === 'user') {
    await apiClient.deleteUser({ login: entity.name } as User);
    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.users.notify_text_deleted'),
      type: 'success',
    });
  } else {
    await apiClient.deleteOrg({ id: entity.id } as any);
    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.orgs.notify_text_deleted'),
      type: 'success',
    });
  }
  await resetPage();
});

async function handleBulkDelete(entities: Entity[]) {
  let successCount = 0;
  const failedEntities: string[] = [];

  for (const entity of entities) {
    try {
      if (entity.entity_type === 'user') {
        await apiClient.deleteUser({ login: entity.name } as User);
      } else {
        await apiClient.deleteOrg({ id: entity.id } as any);
      }
      successCount++;
    } catch (error) {
      failedEntities.push(entity.name);
      console.error('Failed to delete entity:', entity.name, error);
    }
  }

  if (successCount > 0) {
    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.entities.bulk_delete_success', { count: successCount }),
      type: 'success',
    });
  }

  if (failedEntities.length > 0) {
    notifications.notify({
      title: t('notify_title_error'),
      text: `${t('admin.settings.entities.bulk_delete_failed', { count: failedEntities.length })}: ${failedEntities.join(', ')}`,
      type: 'error',
      duration: 10000,
    });
  }

  await resetPage();
}

async function handleBulkToggleAdmin(entities: Entity[]) {
  let successCount = 0;
  const failedUsers: string[] = [];

  for (const entity of entities) {
    if (!isUserEntity(entity)) continue;

    try {
      await apiClient.updateUser({
        login: entity.name,
        email: entity.email,
        avatar_url: entity.avatar,
        admin: !entity.admin,
      });
      successCount++;
    } catch (error) {
      failedUsers.push(entity.name);
      console.error('Failed to toggle admin for user:', entity.name, error);
    }
  }

  if (successCount > 0) {
    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.entities.bulk_toggle_admin_success', { count: successCount }),
      type: 'success',
    });
  }

  if (failedUsers.length > 0) {
    notifications.notify({
      title: t('notify_title_error'),
      text: `${t('admin.settings.entities.bulk_toggle_admin_failed', { count: failedUsers.length })}: ${failedUsers.join(', ')}`,
      type: 'error',
      duration: 10000,
    });
  }

  await resetPage();
}

useCrowTitle(computed(() => [t('admin.settings.entities.title'), t('admin.settings.settings')]));
</script>

<template>
  <Scaffold fill-width>
    <template #title>
      {{ $t('registries.all_registries') }}
    </template>

    <template #headerActions>
      <div class="flex items-center gap-3 flex-wrap">
        <RegistryDialog
          v-model:open="isDialogOpen"
          :registry="selectedRegistry"
          :is-saving="isSaving"
          @save="createRegistry"
          @cancel="handleDialogClose"
        >
          <template #trigger>
            <Button size="sm" @click="showAddRegistry">
              <Icon name="plus" class="mr-2 h-4 w-4" />
              {{ $t('registries.add') }}
            </Button>
          </template>
        </RegistryDialog>
      </div>
    </template>

    <div class="max-w-8xl mx-auto space-y-4">
      <RegistryListGrouped
        :registries="registries"
        :is-deleting="isDeleting"
        :search-term="searchTerm"
        @edit="editRegistry"
        @delete="deleteRegistry"
        @update:search-term="searchTerm = $event"
      />
    </div>
  </Scaffold>
</template>

<script lang="ts" setup>
import { cloneDeep } from 'lodash';
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import RegistryDialog from '~/components/registry/RegistryDialog.vue';
import RegistryListGrouped from '~/components/registry/RegistryListGrouped.vue';
import { Button } from '~/components/ui/button';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useAuthentication from '~/compositions/useAuthentication';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import useNotifications from '~/compositions/useNotifications';
import { usePagination } from '~/compositions/usePaginate';
import type { RegistryWithOrg } from '~/lib/api/types';

const emptyRegistry: Partial<RegistryWithOrg> = {
  address: '',
  username: '',
  password: '',
  scope_type: 'user',
};

const apiClient = useApiClient();
const notifications = useNotifications();
const { t } = useI18n();
const { user } = useAuthentication();

const searchTerm = ref('');
const isDialogOpen = ref(false);
const selectedRegistry = ref<Partial<RegistryWithOrg>>();

async function loadRegistries(page: number) {
  return apiClient.getUserRegistryList({
    scope: 'user,org,global',
    search: searchTerm.value,
    page,
  });
}

const { resetPage, data: registries } = usePagination(loadRegistries, () => true);

// Reload when search changes
watch(searchTerm, async () => {
  await resetPage();
});

const { doSubmit: createRegistry, isLoading: isSaving } = useAsyncAction(
  async (registryData: Partial<RegistryWithOrg>) => {
    if (!registryData || !user) {
      throw new Error("Unexpected: Can't get registry or user");
    }

    const registry = registryData;
    const isEditing = !!registry.id;

    // Determine target org based on scope
    let targetOrgId = user.org_id;
    if (registry.scope_type === 'global') {
      // Use global endpoint
      if (isEditing) {
        await apiClient.updateGlobalRegistry(registry);
      } else {
        await apiClient.createGlobalRegistry(registry);
      }
    } else if (registry.scope_type === 'org' && registry.org_id) {
      targetOrgId = registry.org_id;
      if (isEditing) {
        await apiClient.updateOrgRegistry(targetOrgId, registry);
      } else {
        await apiClient.createOrgRegistry(targetOrgId, registry);
      }
    } else {
      // User/personal org
      if (isEditing) {
        await apiClient.updateOrgRegistry(targetOrgId, registry);
      } else {
        await apiClient.createOrgRegistry(targetOrgId, registry);
      }
    }

    notifications.notify({
      title: t('notify_title_success'),
      text: isEditing ? t('registries.notify_text_saved') : t('registries.notify_text_created'),
      type: 'success',
    });

    isDialogOpen.value = false;
    selectedRegistry.value = undefined;
    await resetPage();
  },
);

const { doSubmit: deleteRegistry, isLoading: isDeleting } = useAsyncAction(async (registry: RegistryWithOrg) => {
  if (!user) {
    throw new Error('Unexpected: User not found');
  }

  if (registry.scope_type === 'global') {
    await apiClient.deleteGlobalRegistry(registry.address);
  } else {
    const targetOrgId = registry.org_id || user.org_id;
    await apiClient.deleteOrgRegistry(targetOrgId, registry.address);
  }

  notifications.notify({
    title: t('notify_title_success'),
    text: t('registries.notify_text_deleted'),
    type: 'success',
  });

  await resetPage();
});

function editRegistry(registry: RegistryWithOrg) {
  selectedRegistry.value = cloneDeep(registry);
  isDialogOpen.value = true;
}

function showAddRegistry() {
  selectedRegistry.value = cloneDeep(emptyRegistry);
  isDialogOpen.value = true;
}

function handleDialogClose() {
  selectedRegistry.value = undefined;
  isDialogOpen.value = false;
}

useCrowTitle(computed(() => [t('registries.all_registries')]));
</script>

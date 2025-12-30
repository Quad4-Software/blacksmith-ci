<template>
  <Scaffold fill-width>
    <template #title>
      {{ $t('registries.all_registries') }}
    </template>

    <template #headerActions>
      <div class="flex items-center gap-3 flex-wrap">
        <Button v-if="selectedRegistry" size="sm" @click="selectedRegistry = undefined">
          <Icon name="back" class="mr-2 h-4 w-4" />
          {{ $t('back') }}
        </Button>
        <Button v-else size="sm" @click="showAddRegistry">
          <Icon name="plus" class="mr-2 h-4 w-4" />
          {{ $t('registries.add') }}
        </Button>
      </div>
    </template>

    <div v-if="!selectedRegistry" class="max-w-5xl mx-auto space-y-4">
      <RegistryListGrouped
        :registries="registries"
        :is-deleting="isDeleting"
        :search-term="searchTerm"
        @edit="editRegistry"
        @delete="deleteRegistry"
        @update:search-term="searchTerm = $event"
      />
    </div>

    <div v-else class="max-w-3xl">
      <RegistryEdit
        v-model="selectedRegistry"
        :is-saving="isSaving"
        @save="createRegistry"
        @cancel="selectedRegistry = undefined"
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
import RegistryEdit from '~/components/registry/RegistryEdit.vue';
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

const selectedRegistry = ref<Partial<RegistryWithOrg>>();
const isEditingRegistry = computed(() => !!selectedRegistry.value?.id);

async function loadRegistries(page: number) {
  return apiClient.getUserRegistryList({
    scope: 'user,org',
    search: searchTerm.value,
    page,
  });
}

const { resetPage, data: registries } = usePagination(loadRegistries, () => !selectedRegistry.value);

// Reload when search changes
watch(searchTerm, async () => {
  await resetPage();
});

const { doSubmit: createRegistry, isLoading: isSaving } = useAsyncAction(async () => {
  if (!selectedRegistry.value || !user) {
    throw new Error("Unexpected: Can't get registry or user");
  }

  // Determine target org based on scope
  let targetOrgId = user.org_id;
  if (selectedRegistry.value.scope_type === 'global') {
    // Use global endpoint
    if (isEditingRegistry.value) {
      await apiClient.updateGlobalRegistry(selectedRegistry.value);
    } else {
      await apiClient.createGlobalRegistry(selectedRegistry.value);
    }
  } else if (selectedRegistry.value.scope_type === 'org' && selectedRegistry.value.org_id) {
    targetOrgId = selectedRegistry.value.org_id;
    if (isEditingRegistry.value) {
      await apiClient.updateOrgRegistry(targetOrgId, selectedRegistry.value);
    } else {
      await apiClient.createOrgRegistry(targetOrgId, selectedRegistry.value);
    }
  } else {
    // User/personal org
    if (isEditingRegistry.value) {
      await apiClient.updateOrgRegistry(targetOrgId, selectedRegistry.value);
    } else {
      await apiClient.createOrgRegistry(targetOrgId, selectedRegistry.value);
    }
  }

  notifications.notify({
    title: t('notify_title_success'),
    text: isEditingRegistry.value ? t('registries.notify_text_saved') : t('registries.notify_text_created'),
    type: 'success',
  });

  selectedRegistry.value = undefined;
  await resetPage();
});

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
}

function showAddRegistry() {
  selectedRegistry.value = cloneDeep(emptyRegistry);
}

useCrowTitle(computed(() => [t('registries.all_registries')]));
</script>

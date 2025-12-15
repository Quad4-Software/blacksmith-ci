<template>
  <Dialog v-model:open="registryDialog.isOpen.value">
    <DialogContent class="sm:max-w-[600px]">
      <DialogHeader>
        <DialogTitle>{{ $t('registries.add') }}</DialogTitle>
        <DialogDescription>
          {{ $t('registries.add_desc') }}
        </DialogDescription>
      </DialogHeader>

      <RegistryEdit v-model="selectedRegistry" :is-saving="isSaving" @save="handleSave" @cancel="handleCancel" />
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';

import RegistryEdit from '~/components/registry/RegistryEdit.vue';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useGlobalDialogs } from '~/compositions/useGlobalDialogs';
import useNotifications from '~/compositions/useNotifications';
import type { Registry } from '~/lib/api/types';

const { t } = useI18n();
const apiClient = useApiClient();
const notifications = useNotifications();
const { registryDialog } = useGlobalDialogs();

const selectedRegistry = ref<Partial<Registry>>({
  address: '',
  username: '',
  password: '',
});

const { doSubmit: handleSave, isLoading: isSaving } = useAsyncAction(async () => {
  if (!selectedRegistry.value) {
    throw new Error("Unexpected: Can't get registry");
  }

  await apiClient.createGlobalRegistry(selectedRegistry.value);

  notifications.notify({
    title: t('notify_title_success'),
    text: t('registries.notify_text_created'),
    type: 'success',
  });

  selectedRegistry.value = {
    address: '',
    username: '',
    password: '',
  };
  registryDialog.close();
});

function handleCancel() {
  selectedRegistry.value = {
    address: '',
    username: '',
    password: '',
  };
  registryDialog.close();
}
</script>

<template>
  <AgentDialog
    ref="dialogRef"
    v-model:open="isOpen"
    :agent="null"
    :is-saving="isSaving"
    :is-admin="isAdmin"
    @save="handleSave"
    @cancel="handleCancel"
  />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import AgentDialog from '~/components/agent/AgentDialog.vue';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useAuthentication from '~/compositions/useAuthentication';
import { useGlobalDialogs } from '~/compositions/useGlobalDialogs';
import useNotifications from '~/compositions/useNotifications';
import type { Agent } from '~/lib/api/types';

const { t } = useI18n();
const apiClient = useApiClient();
const notifications = useNotifications();
const { agentDialog } = useGlobalDialogs();
const { user } = useAuthentication();

const isAdmin = computed(() => !!user?.admin);
const dialogRef = ref<InstanceType<typeof AgentDialog> | null>(null);

const isOpen = computed({
  get: () => agentDialog.isOpen.value,
  set: (value) => {
    if (!value) {
      agentDialog.close();
    }
  },
});

const { doSubmit: handleSave, isLoading: isSaving } = useAsyncAction(async (agentData: Partial<Agent>) => {
  const createdAgent = await apiClient.createAgent(agentData);

  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.agents.notify_text_created'),
    type: 'success',
  });

  // Show the created agent with token
  dialogRef.value?.showCreatedAgent(createdAgent);
});

function handleCancel() {
  agentDialog.close();
}
</script>

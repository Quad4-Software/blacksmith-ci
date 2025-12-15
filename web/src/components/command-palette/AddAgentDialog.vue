<template>
  <Dialog v-model:open="agentDialog.isOpen.value">
    <DialogContent class="sm:max-w-[600px]">
      <DialogHeader>
        <DialogTitle>{{ $t('admin.settings.agents.add') }}</DialogTitle>
        <DialogDescription>
          {{ $t('admin.settings.agents.add_desc') }}
        </DialogDescription>
      </DialogHeader>

      <AgentForm
        v-model="selectedAgent"
        :is-editing-agent="false"
        :is-saving="isSaving"
        @save="handleSave"
        @cancel="handleCancel"
      />
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';

import AgentForm from '~/components/agent/AgentForm.vue';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useGlobalDialogs } from '~/compositions/useGlobalDialogs';
import useNotifications from '~/compositions/useNotifications';
import type { Agent } from '~/lib/api/types';

const { t } = useI18n();
const apiClient = useApiClient();
const notifications = useNotifications();
const { agentDialog } = useGlobalDialogs();

const selectedAgent = ref<Partial<Agent>>({
  name: '',
});

const { doSubmit: handleSave, isLoading: isSaving } = useAsyncAction(async () => {
  if (!selectedAgent.value) {
    throw new Error("Unexpected: Can't get agent");
  }

  await apiClient.createAgent(selectedAgent.value);

  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.agents.notify_text_created'),
    type: 'success',
  });

  selectedAgent.value = {
    name: '',
  };
  agentDialog.close();
});

function handleCancel() {
  selectedAgent.value = {
    name: '',
  };
  agentDialog.close();
}
</script>

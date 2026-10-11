<template>
  <div class="mx-auto w-full max-w-none px-4">
    <Settings :title="$t('admin.settings.agents.agents')" :description="$t('user.settings.agents.desc')">
      <template #headerActions>
        <Button v-if="selectedAgent" @click="selectedAgent = undefined">
          <Icon name="back" class="size-4 mr-1" />{{ $t('admin.settings.agents.show') }}
        </Button>
        <Button v-else @click="showAddAgent">
          <Icon name="plus" class="size-4 mr-1" />{{ $t('admin.settings.agents.add') }}
        </Button>
      </template>

      <AgentList
        v-if="!selectedAgent"
        :loading="loading"
        :agents="agents"
        :is-deleting="isDeleting"
        :user-org-id="user.org_id"
        :agent-filters="agentFilters"
        :current-page="page"
        :has-more="hasMore"
        @update:current-page="goToPage"
        @update-filters="(filters) => Object.assign(agentFilters, filters)"
        @edit="editAgent"
        @delete="deleteAgent"
        @filters-changed="resetPage"
      />
      <AgentForm
        v-else
        v-model="selectedAgent"
        :is-editing-agent="isEditingAgent"
        :is-saving="isSaving"
        @save="saveAgent"
        @cancel="selectedAgent = undefined"
      />
    </Settings>
  </div>
</template>

<script lang="ts" setup>
import { cloneDeep } from 'lodash';
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import Settings from '~/components/layout/Settings.vue';
import AgentForm from '~/components/agent/AgentForm.vue';
import AgentList from '~/components/agent/AgentList.vue';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useAuthentication from '~/compositions/useAuthentication';
import useNotifications from '~/compositions/useNotifications';
import { usePaginationWithPages } from '~/compositions/usePaginate';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { Agent, AgentWithToken } from '~/lib/api/types';

const apiClient = useApiClient();
const notifications = useNotifications();
const { user } = useAuthentication();
const { t } = useI18n();

if (!user) {
  throw new Error('Unexpected: User should be authenticated');
}

const selectedAgent = ref<Partial<AgentWithToken>>();
const isEditingAgent = computed(() => !!selectedAgent.value?.id);

// Agent list filters
const agentFilters = ref({
  search: '',
  platform: '',
  backend: '',
  sort: 'status',
  order: 'asc',
});

const loadAgents = (page: number) => apiClient.getAgents({ page, ...agentFilters.value });
const { page, resetPage, goToPage, data: agents, loading, hasMore } = usePaginationWithPages(loadAgents);

const { doSubmit: saveAgent, isLoading: isSaving } = useAsyncAction(async () => {
  if (!selectedAgent.value) {
    throw new Error("Unexpected: Can't get agent");
  }

  if (isEditingAgent.value) {
    await apiClient.updateOrgAgent(user.org_id, selectedAgent.value.id!, selectedAgent.value);
    selectedAgent.value = undefined;
  } else {
    selectedAgent.value = await apiClient.createOrgAgent(user.org_id, selectedAgent.value);
  }
  notifications.notify({
    title: t('notify_title_success'),
    text: isEditingAgent.value
      ? t('admin.settings.agents.notify_text_saved')
      : t('admin.settings.agents.notify_text_created'),
    type: 'success',
    duration: 6000,
  });
  await resetPage();
});

const { doSubmit: deleteAgent, isLoading: isDeleting } = useAsyncAction(async (_agent: Agent) => {
  // eslint-disable-next-line no-alert
  if (!confirm(t('admin.settings.agents.delete_confirm'))) {
    return;
  }

  await apiClient.deleteOrgAgent(user.org_id, _agent.id);
  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.agents.notify_text_deleted'),
    type: 'success',
    duration: 6000,
  });
  await resetPage();
});

function editAgent(agent: Agent) {
  selectedAgent.value = cloneDeep(agent);
}

function showAddAgent() {
  selectedAgent.value = { name: '' };
}

useCrowTitle(computed(() => [t('admin.settings.agents.agents'), t('user.settings.settings')]));
</script>

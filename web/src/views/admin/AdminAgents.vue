<template>
  <AgentManager
    :description="$t('admin.settings.agents.desc')"
    :load-agents="loadAgents"
    :create-agent="createAgent"
    :update-agent="updateAgent"
    :delete-agent="deleteAgent"
    :agent-filters="agentFilters"
    :is-admin="true"
    @update-filters="(filters) => Object.assign(agentFilters, filters)"
  />
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import AgentManager from '~/components/agent/AgentManager.vue';
import useApiClient from '~/compositions/useApiClient';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { Agent } from '~/lib/api/types';

const apiClient = useApiClient();

// Agent list filters
const agentFilters = ref({
  search: '',
  platform: '',
  backend: '',
  sort: 'status',
  order: 'asc',
});

const loadAgents = (page: number) =>
  apiClient.getAgents({
    page,
    ...agentFilters.value,
  });

const createAgent = (agent: Partial<Agent>) => apiClient.createAgent(agent);
const updateAgent = (agent: Agent) => apiClient.updateAgent(agent);
const deleteAgent = (agent: Agent) => apiClient.deleteAgent(agent);

const { t } = useI18n();
useCrowTitle(computed(() => [t('admin.settings.agents.agents'), t('admin.settings.settings')]));
</script>

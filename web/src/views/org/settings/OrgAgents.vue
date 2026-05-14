<template>
  <AgentManager
    :description="$t('org.settings.agents.desc')"
    :load-agents="loadAgents"
    :create-agent="createAgent"
    :update-agent="updateAgent"
    :delete-agent="deleteAgent"
    :agent-filters="agentFilters"
    :user-org-id="org.id"
  />
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import AgentManager from '~/components/agent/AgentManager.vue';
import useApiClient from '~/compositions/useApiClient';
import { requiredInject } from '~/compositions/useInjectProvide';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { Agent } from '~/lib/api/types';

const apiClient = useApiClient();
const org = requiredInject('org');

// Agent list filters
const agentFilters = ref({
  search: '',
  platform: '',
  backend: '',
  sort: 'status',
  order: 'asc',
});

const loadAgents = (page: number) => apiClient.getOrgAgents(org.value.id, { page, ...agentFilters.value });
const createAgent = (agent: Partial<Agent>) => apiClient.createOrgAgent(org.value.id, agent);
const updateAgent = (agent: Agent) => apiClient.updateOrgAgent(org.value.id, agent.id, agent);
const deleteAgent = (agent: Agent) => apiClient.deleteOrgAgent(org.value.id, agent.id);

const { t } = useI18n();
useCrowTitle(computed(() => [t('admin.settings.agents.agents'), org.value.name]));
</script>

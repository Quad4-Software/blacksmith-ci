<template>
  <AgentManager
    :description="$t('user.settings.agents.desc')"
    :load-agents="loadAgents"
    :create-agent="createAgent"
    :update-agent="updateAgent"
    :delete-agent="deleteAgent"
    :user-org-id="user.org_id"
  >
  </AgentManager>
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

import AgentManager from '~/components/agent/AgentManager.vue';
import useApiClient from '~/compositions/useApiClient';
import useAuthentication from '~/compositions/useAuthentication';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { Agent } from '~/lib/api/types';

const apiClient = useApiClient();
const { user } = useAuthentication();

if (!user) {
  throw new Error('Unexpected: User should be authenticated');
}

const loadAgents = (page: number) => apiClient.getAgents({ page });
const createAgent = (agent: Partial<Agent>) => apiClient.createOrgAgent(user.org_id, agent);
const updateAgent = (agent: Agent) => apiClient.updateOrgAgent(user.org_id, agent.id, agent);
const deleteAgent = (agent: Agent) => apiClient.deleteOrgAgent(user.org_id, agent.id);

const { t } = useI18n();
useCrowTitle(computed(() => [t('admin.settings.agents.agents'), t('user.settings.settings')]));
</script>

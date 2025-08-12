<template>
  <div class="mx-auto w-full max-w-none px-4">
    <Settings :title="$t('admin.settings.agents.agents')" :description="$t('admin.settings.agents.desc')">
      <AgentList
        :loading="loading"
        :agents="agents"
        :is-deleting="false"
        :is-admin="false"
        :user-org-id="undefined"
        @edit="() => {}"
        @delete="() => {}"
      />
    </Settings>
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

import Settings from '~/components/layout/Settings.vue';
import AgentList from '~/components/agent/AgentList.vue';
import useApiClient from '~/compositions/useApiClient';
import { usePagination } from '~/compositions/usePaginate';
import { useCrowTitle } from '~/compositions/useCrowTitle';

const apiClient = useApiClient();
const { t } = useI18n();

const loadAgents = (page: number) => apiClient.getAgents({ page });
const { data: agents, loading } = usePagination(loadAgents);

useCrowTitle(computed(() => [t('admin.settings.agents.agents'), t('user.settings.settings')]));
</script>

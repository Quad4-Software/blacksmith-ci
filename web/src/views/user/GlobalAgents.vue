<template>
  <div class="mx-auto w-full max-w-none px-4">
    <Settings :title="$t('admin.settings.agents.agents')" :description="$t('admin.settings.agents.desc')">
      <AgentList
        :loading="loading"
        :agents="agents"
        :is-deleting="false"
        :is-admin="false"
        :user-org-id="undefined"
        :agent-filters="agentFilters"
        :current-page="page"
        :has-more="hasMore"
        @update:current-page="goToPage"
        @update-filters="(filters) => Object.assign(agentFilters, filters)"
        @edit="() => {}"
        @delete="() => {}"
        @filters-changed="resetPage"
      />
    </Settings>
  </div>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Settings from '~/components/layout/Settings.vue';
import AgentList from '~/components/agent/AgentList.vue';
import useApiClient from '~/compositions/useApiClient';
import { usePaginationWithPages } from '~/compositions/usePaginate';
import { useCrowTitle } from '~/compositions/useCrowTitle';

const apiClient = useApiClient();
const { t } = useI18n();

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

useCrowTitle(computed(() => [t('admin.settings.agents.agents'), t('user.settings.settings')]));
</script>

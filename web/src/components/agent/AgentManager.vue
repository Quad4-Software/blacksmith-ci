<template>
  <Settings :title="$t('admin.settings.agents.agents')" :description>
    <template #headerActions>
      <Button
        v-if="selectedAgent"
        :text="$t('admin.settings.agents.show')"
        start-icon="back"
        @click="selectedAgent = undefined"
      />
      <template v-if="!selectedAgent">
        <div class="flex flex-col md:flex-row items-center justify-center gap-2">
          <Button text="Refresh" class="w-auto" @click="resetPage" />
          <Button :text="$t('admin.settings.agents.add')" start-icon="plus" @click="showAddAgent" />
        </div>
      </template>
    </template>
    <template #headerEnd>
      <slot name="headerEnd" />
    </template>
    <AgentList
      v-if="!selectedAgent"
      :loading="loading"
      :agents="agents"
      :is-deleting="isDeleting"
      :is-admin="isAdmin"
      :user-org-id="userOrgId"
      :agent-filters="agentFilters"
      :current-page="page"
      :has-more="hasMore"
      :is-jumping-to-last="isJumpingToLast"
      @update:current-page="goToPage"
      @jump-to-last="handleJumpToLast"
      @update-filters="(filters) => emit('updateFilters', filters)"
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
</template>

<script lang="ts" setup>
import { cloneDeep } from 'lodash';
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Button from '~/components/atomic/Button.vue';
import Settings from '~/components/layout/Settings.vue';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useNotifications from '~/compositions/useNotifications';
import { usePaginationWithPages } from '~/compositions/usePaginate';
import type { Agent } from '~/lib/api/types';

import AgentForm from './AgentForm.vue';
import AgentList from './AgentList.vue';

const props = defineProps<{
  description: string;
  loadAgents: (page: number) => Promise<Agent[] | null>;
  createAgent: (agent: Partial<Agent>) => Promise<Agent>;
  updateAgent: (agent: Agent) => Promise<Agent | void>;
  deleteAgent: (agent: Agent) => Promise<unknown>;
  agentFilters: {
    search: string;
    platform: string;
    backend: string;
    sort: string;
    order: string;
  };
  isAdmin?: boolean;
  userOrgId?: number;
}>();

const emit = defineEmits<{
  updateFilters: [filters: { search?: string; platform?: string; backend?: string; sort?: string; order?: string }];
}>();

const notifications = useNotifications();
const { t } = useI18n();

const selectedAgent = ref<Partial<Agent>>();
const isEditingAgent = computed(() => !!selectedAgent.value?.id);

// Use page-based pagination with page numbers
const {
  page,
  resetPage,
  goToPage,
  goToLastPage,
  data: agents,
  loading,
  hasMore,
} = usePaginationWithPages(props.loadAgents);

// Track if we're jumping to last page
const isJumpingToLast = ref(false);

async function handleJumpToLast() {
  isJumpingToLast.value = true;
  try {
    await goToLastPage();
  } finally {
    isJumpingToLast.value = false;
  }
}

const { doSubmit: saveAgent, isLoading: isSaving } = useAsyncAction(async () => {
  if (!selectedAgent.value) {
    throw new Error("Unexpected: Can't get agent");
  }

  if (isEditingAgent.value) {
    await props.updateAgent(selectedAgent.value as Agent);
    selectedAgent.value = undefined;
  } else {
    selectedAgent.value = await props.createAgent(selectedAgent.value);
  }
  notifications.notify({
    title: t('notify_title_success'),
    text: isEditingAgent.value
      ? t('admin.settings.agents.notify_text_saved')
      : t('admin.settings.agents.notify_text_created'),
    type: 'success',
    duration: 6000,
  });
  resetPage();
});

const { doSubmit: deleteAgent, isLoading: isDeleting } = useAsyncAction(async (_agent: Agent) => {
  // eslint-disable-next-line no-alert
  if (!confirm(t('admin.settings.agents.delete_confirm'))) {
    return;
  }

  await props.deleteAgent(_agent);
  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.agents.notify_text_deleted'),
    type: 'success',
    duration: 6000,
  });
  resetPage();
});

function editAgent(agent: Agent) {
  selectedAgent.value = cloneDeep(agent);
}

function showAddAgent() {
  selectedAgent.value = { name: '' };
}
</script>

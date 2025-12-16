<template>
  <div class="mx-auto w-full max-w-none px-4 overflow-y-auto">
    <Settings :title="$t('admin.settings.agents.agents')" :description="description">
      <template #headerActions>
        <Button
          v-if="selectedAgent"
          :text="$t('admin.settings.agents.show')"
          start-icon="back"
          @click="selectedAgent = undefined"
        />
        <Button v-else :text="$t('admin.settings.agents.add')" start-icon="plus" @click="showAddAgent" />
      </template>

      <AgentList
        v-if="!selectedAgent"
        :loading="loading"
        :agents="agents"
        :is-deleting="isDeleting"
        :is-admin="isAdmin"
        :user-org-id="user.org_id"
        :agent-filters="agentFilters"
        :current-page="page"
        :has-more="hasMore"
        @update:current-page="goToPage"
        @update-filters="(filters) => Object.assign(agentFilters, filters)"
        @edit="editAgent"
        @delete="openDeleteDialog"
        @filters-changed="softResetPage"
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

    <!-- Delete confirmation dialog -->
    <AlertDialog :open="!!agentToDelete" @update:open="(open) => !open && (agentToDelete = undefined)">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('admin.settings.agents.delete_agent') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('admin.settings.agents.delete_confirm') }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="confirmDelete">{{ $t('delete') }}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>

<script lang="ts" setup>
import { cloneDeep } from 'lodash';
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute } from 'vue-router';

import Button from '~/components/atomic/Button.vue';
import Settings from '~/components/layout/Settings.vue';
import AgentForm from '~/components/agent/AgentForm.vue';
import AgentList from '~/components/agent/AgentList.vue';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '~/components/ui/alert-dialog';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useAuthentication from '~/compositions/useAuthentication';
import useNotifications from '~/compositions/useNotifications';
import { usePaginationWithPages } from '~/compositions/usePaginate';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { Agent } from '~/lib/api/types';

const apiClient = useApiClient();
const notifications = useNotifications();
const { user } = useAuthentication();
const { t } = useI18n();
const route = useRoute();

if (!user) {
  throw new Error('Unexpected: User should be authenticated');
}

const isAdmin = computed(() => !!user.admin);

// Description changes based on user role
const description = computed(() => (isAdmin.value ? t('admin.settings.agents.desc') : t('user.settings.agents.desc')));

const selectedAgent = ref<Partial<Agent>>();
const isEditingAgent = computed(() => !!selectedAgent.value?.id);
const agentToDelete = ref<Agent>();

// Agent list filters - initialize search from query param if present
const initialSearch = typeof route.query.search === 'string' ? route.query.search : '';
const agentFilters = ref({
  search: initialSearch,
  platform: '',
  backend: '',
  sort: 'status',
  order: 'asc',
});

const loadAgents = (page: number) => apiClient.getAgents({ page, ...agentFilters.value });
const { page, goToPage, data: agents, loading, hasMore } = usePaginationWithPages(loadAgents);

// Soft reset for filters - doesn't clear data, just resets to page 1
async function softResetPage() {
  if (page.value !== 1) {
    page.value = 1; // This triggers the watcher to reload without clearing data
  } else {
    // Already on page 1, manually trigger reload
    const newData = await loadAgents(1);
    if (newData) {
      agents.value = newData;
    }
  }
}

const { doSubmit: saveAgent, isLoading: isSaving } = useAsyncAction(async () => {
  if (!selectedAgent.value) {
    throw new Error("Unexpected: Can't get agent");
  }

  const wasEditing = isEditingAgent.value;

  if (wasEditing) {
    // Editing: check permissions first
    const canEdit = (selectedAgent.value.org_id === -1 && isAdmin.value) || selectedAgent.value.org_id === user.org_id;

    if (!canEdit) {
      throw new Error('Unauthorized: Cannot edit this agent');
    }

    if (selectedAgent.value.org_id === -1) {
      // Global agent - admin only
      await apiClient.updateAgent(selectedAgent.value as Agent);
    } else {
      // Org-scoped agent - user's own org
      await apiClient.updateOrgAgent(selectedAgent.value.org_id!, selectedAgent.value.id!, selectedAgent.value);
    }
    selectedAgent.value = undefined;
  } else {
    // Creating: admins create global agents, others create org-scoped
    if (isAdmin.value) {
      selectedAgent.value = await apiClient.createAgent(selectedAgent.value);
    } else {
      selectedAgent.value = await apiClient.createOrgAgent(user.org_id, selectedAgent.value);
    }
  }
  notifications.notify({
    title: t('notify_title_success'),
    text: wasEditing ? t('admin.settings.agents.notify_text_saved') : t('admin.settings.agents.notify_text_created'),
    type: 'success',
    duration: 6000,
  });
  softResetPage();
});

function openDeleteDialog(agent: Agent) {
  // Check permissions before showing dialog
  const canDelete = (agent.org_id === -1 && isAdmin.value) || agent.org_id === user!.org_id;

  if (!canDelete) {
    notifications.notify({
      title: t('notify_title_error'),
      text: 'Unauthorized: Cannot delete this agent',
      type: 'error',
      duration: 6000,
    });
    return;
  }

  agentToDelete.value = agent;
}

const { doSubmit: confirmDelete, isLoading: isDeleting } = useAsyncAction(async () => {
  if (!agentToDelete.value) {
    return;
  }

  const agent = agentToDelete.value;

  // Use appropriate API based on agent type
  if (agent.org_id === -1) {
    // Global agent - admin only
    await apiClient.deleteAgent(agent);
  } else {
    // Org-scoped agent - user's own org
    await apiClient.deleteOrgAgent(agent.org_id, agent.id);
  }

  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.agents.notify_text_deleted'),
    type: 'success',
    duration: 6000,
  });

  agentToDelete.value = undefined;
  softResetPage();
});

function editAgent(agent: Agent) {
  selectedAgent.value = cloneDeep(agent);
}

function showAddAgent() {
  selectedAgent.value = { name: '' };
}

useCrowTitle(computed(() => [t('admin.settings.agents.agents')]));
</script>

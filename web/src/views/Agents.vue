<template>
  <div class="mx-auto w-full max-w-none overflow-y-auto">
    <Settings :title="$t('admin.settings.agents.agents')" :description="description">
      <template v-if="schedulingStrategy" #titleBadge>
        <Badge variant="secondary">
          {{ $t('admin.settings.agents.scheduling_strategy', { strategy: schedulingStrategy }) }}
        </Badge>
      </template>
      <template #headerActions>
        <Button @click="showAddAgent">
          <Icon name="plus" class="size-4 mr-1" />{{ $t('admin.settings.agents.add') }}
        </Button>
      </template>

      <AgentList
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
    </Settings>

    <!-- Agent Create/Edit Dialog -->
    <AgentDialog
      ref="agentDialogRef"
      v-model:open="dialogOpen"
      :agent="selectedAgent"
      :is-saving="isSaving"
      :is-admin="isAdmin"
      @save="handleSave"
      @cancel="dialogOpen = false"
    />

    <!-- Delete confirmation dialog -->
    <AlertDialog :open="!!agentToDelete">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('admin.settings.agents.delete_agent') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('admin.settings.agents.delete_confirm') }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel @click="agentToDelete = undefined">{{ $t('cancel') }}</AlertDialogCancel>
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

import Icon from '~/components/atomic/Icon.vue';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import Settings from '~/components/layout/Settings.vue';
import AgentDialog from '~/components/agent/AgentDialog.vue';
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
import useConfig from '~/compositions/useConfig';
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

// Server-wide active scheduling strategy, surfaced from the injected config.
const schedulingStrategy = useConfig().schedulingStrategy;

// Description changes based on user role
const description = computed(() => (isAdmin.value ? t('admin.settings.agents.desc') : t('user.settings.agents.desc')));

const dialogOpen = ref(false);
const selectedAgent = ref<Agent | null>(null);
const agentToDelete = ref<Agent>();
const agentDialogRef = ref<InstanceType<typeof AgentDialog> | null>(null);

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

const { doSubmit: handleSave, isLoading: isSaving } = useAsyncAction(async (agentData: Partial<Agent>) => {
  const isEditing = !!agentData.id;

  if (isEditing) {
    // Editing: check permissions first
    const canEdit = isAdmin.value || (agentData.org_ids?.includes(user.org_id) ?? false);

    if (!canEdit) {
      throw new Error('Unauthorized: Cannot edit this agent');
    }

    if (isAdmin.value) {
      // Admin uses the global endpoint, which works for any agent regardless of org scope
      await apiClient.updateAgent(agentData as Agent);
    } else {
      // Non-admin: must use the org-scoped endpoint for their own org
      await apiClient.updateOrgAgent(user.org_id, agentData.id!, agentData);
    }

    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.agents.notify_text_saved'),
      type: 'success',
      duration: 6000,
    });

    dialogOpen.value = false;
  } else {
    // Creating: admins create global agents, others create org-scoped
    let createdAgent: Agent;
    if (isAdmin.value) {
      createdAgent = await apiClient.createAgent(agentData);
    } else {
      createdAgent = await apiClient.createOrgAgent(user.org_id, agentData);
    }

    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.agents.notify_text_created'),
      type: 'success',
      duration: 6000,
    });

    // Show the created agent with token in dialog
    agentDialogRef.value?.showCreatedAgent(createdAgent);
  }

  softResetPage();
});

function openDeleteDialog(agent: Agent) {
  // Check permissions before showing dialog
  const canDelete = isAdmin.value || agent.org_ids.includes(user!.org_id);

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

  if (isAdmin.value) {
    // Admin uses the global endpoint, which works for any agent regardless of org scope
    await apiClient.deleteAgent(agent);
  } else {
    // Non-admin: must use the org-scoped endpoint for their own org
    await apiClient.deleteOrgAgent(user.org_id, agent.id);
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
  dialogOpen.value = true;
}

function showAddAgent() {
  selectedAgent.value = null;
  dialogOpen.value = true;
}

useCrowTitle(computed(() => [t('admin.settings.agents.agents')]));
</script>

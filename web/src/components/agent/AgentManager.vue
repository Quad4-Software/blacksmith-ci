<template>
  <Settings :title="$t('admin.settings.agents.agents')" :description>
    <template #headerActions>
      <Button variant="default" size="sm" @click="showAddAgent">
        <Icon name="plus" class="size-4 mr-1" />{{ $t('admin.settings.agents.add') }}
      </Button>
    </template>
    <template #headerEnd>
      <slot name="headerEnd" />
    </template>

    <AgentList
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
      @delete="openDeleteDialog"
      @filters-changed="resetPage"
    />

    <!-- Agent Create/Edit Dialog -->
    <AgentDialog
      ref="agentDialogRef"
      v-model:open="dialogOpen"
      :agent="selectedAgent"
      :is-saving="isSaving"
      :is-admin="isAdmin"
      :scope-editable="props.scopeEditable"
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
  </Settings>
</template>

<script lang="ts" setup>
import { cloneDeep } from 'lodash';
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import Settings from '~/components/layout/Settings.vue';
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
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useNotifications from '~/compositions/useNotifications';
import { usePaginationWithPages } from '~/compositions/usePaginate';
import type { Agent } from '~/lib/api/types';

import AgentDialog from './AgentDialog.vue';
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
  scopeEditable?: boolean;
}>();

const emit = defineEmits<{
  updateFilters: [filters: { search?: string; platform?: string; backend?: string; sort?: string; order?: string }];
}>();

const notifications = useNotifications();
const { t } = useI18n();

const dialogOpen = ref(false);
const selectedAgent = ref<Agent | null>(null);
const agentToDelete = ref<Agent>();
const agentDialogRef = ref<InstanceType<typeof AgentDialog> | null>(null);

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

const { doSubmit: handleSave, isLoading: isSaving } = useAsyncAction(async (agentData: Partial<Agent>) => {
  const isEditing = !!agentData.id;

  if (isEditing) {
    await props.updateAgent(agentData as Agent);
    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.agents.notify_text_saved'),
      type: 'success',
      duration: 6000,
    });
    dialogOpen.value = false;
  } else {
    const createdAgent = await props.createAgent(agentData);
    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.agents.notify_text_created'),
      type: 'success',
      duration: 6000,
    });
    // Show the created agent with token in dialog
    agentDialogRef.value?.showCreatedAgent(createdAgent);
  }

  await resetPage();
});

function openDeleteDialog(agent: Agent) {
  agentToDelete.value = agent;
}

const { doSubmit: confirmDelete, isLoading: isDeleting } = useAsyncAction(async () => {
  if (!agentToDelete.value) {
    return;
  }

  await props.deleteAgent(agentToDelete.value);
  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.agents.notify_text_deleted'),
    type: 'success',
    duration: 6000,
  });

  agentToDelete.value = undefined;
  await resetPage();
});

function editAgent(agent: Agent) {
  selectedAgent.value = cloneDeep(agent);
  dialogOpen.value = true;
}

function showAddAgent() {
  selectedAgent.value = null;
  dialogOpen.value = true;
}
</script>

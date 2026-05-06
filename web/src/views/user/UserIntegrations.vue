<template>
  <Scaffold fill-width>
    <template #title>
      {{ $t('integrations.title') }}
    </template>

    <template #headerActions>
      <div class="flex items-center gap-3 flex-wrap">
        <IntegrationDialog
          v-model:open="isDialogOpen"
          :integration="selectedIntegration"
          :is-saving="isSaving"
          @save="saveIntegration"
          @cancel="handleDialogClose"
        >
          <template #trigger>
            <Button size="sm" @click="showAddIntegration">
              <Icon name="plus" class="mr-2 h-4 w-4" />
              {{ $t('integrations.add') }}
            </Button>
          </template>
        </IntegrationDialog>
      </div>
    </template>

    <div class="max-w-7xl mx-auto space-y-4">
      <IntegrationList
        :integrations="integrations"
        :is-deleting="isDeleting"
        :is-testing="isTesting"
        :users-map="usersMap"
        :orgs-map="orgsMap"
        :repos-map="reposMap"
        @edit="editIntegration"
        @delete="deleteIntegration"
        @test="testIntegrationConnection"
      />
    </div>
  </Scaffold>
</template>

<script lang="ts" setup>
import { cloneDeep } from 'lodash';
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import IntegrationDialog from '~/components/integrations/IntegrationDialog.vue';
import IntegrationList from '~/components/integrations/IntegrationList.vue';
import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import { Button } from '~/components/ui/button';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import useNotifications from '~/compositions/useNotifications';
import { usePagination } from '~/compositions/usePaginate';
import type { Integration, Org, Repo, User } from '~/lib/api/types';

const emptyIntegration: Partial<Integration> = {
  name: '',
  type: 'vault',
  description: '',
  config: '',
  allowed_user_ids: [],
  allowed_org_ids: [],
  allowed_repo_ids: [],
  is_active: true,
};

const apiClient = useApiClient();
const notifications = useNotifications();
const { t } = useI18n();

const isDialogOpen = ref(false);
const selectedIntegration = ref<Partial<Integration>>();

// Lookup data for displaying names in tooltips
const users = ref<User[]>([]);
const orgs = ref<Org[]>([]);
const repos = ref<Repo[]>([]);

// Create lookup maps for quick access
const usersMap = computed(() => new Map(users.value.map((u) => [u.id, u])));
const orgsMap = computed(() => new Map(orgs.value.map((o) => [o.id, o])));
const reposMap = computed(() => new Map(repos.value.map((r) => [r.id, r])));

onMounted(async () => {
  // Load lookup data in parallel
  const [usersData, orgsData, reposData] = await Promise.all([
    apiClient.getUsers().catch(() => []),
    apiClient.getUserOrgs().catch(() => []),
    apiClient.getActiveRepos().catch(() => []),
  ]);
  users.value = usersData || [];
  orgs.value = orgsData || [];
  repos.value = reposData || [];
});

async function loadIntegrations(page: number) {
  return apiClient.getIntegrationList({ page });
}

const { resetPage, data: integrations } = usePagination(loadIntegrations, () => true);

function notifyError(error: unknown) {
  const message = error instanceof Error ? error.message : String(error);
  notifications.notify({
    title: t('notify_title_error'),
    text: message,
    type: 'error',
  });
}

const { doSubmit: saveIntegration, isLoading: isSaving } = useAsyncAction(
  async (integrationData: Partial<Integration>) => {
    if (!integrationData) {
      throw new Error("Unexpected: Can't get integration data");
    }

    const isEditing = !!integrationData.id;

    try {
      if (isEditing) {
        await apiClient.updateIntegration(integrationData.id!, integrationData);
      } else {
        await apiClient.createIntegration(integrationData);
      }

      notifications.notify({
        title: t('notify_title_success'),
        text: isEditing ? t('integrations.notify_saved') : t('integrations.notify_created'),
        type: 'success',
      });

      isDialogOpen.value = false;
      selectedIntegration.value = undefined;
      await resetPage();
    } catch (error) {
      notifyError(error);
    }
  },
);

const { doSubmit: deleteIntegration, isLoading: isDeleting } = useAsyncAction(async (integration: Integration) => {
  try {
    await apiClient.deleteIntegration(integration.id);

    notifications.notify({
      title: t('notify_title_success'),
      text: t('integrations.notify_deleted'),
      type: 'success',
    });

    await resetPage();
  } catch (error) {
    notifyError(error);
  }
});

const { doSubmit: testIntegrationConnection, isLoading: isTesting } = useAsyncAction(
  async (integration: Integration) => {
    try {
      await apiClient.testIntegration(integration.id);

      notifications.notify({
        title: t('notify_title_success'),
        text: t('integrations.notify_test_success'),
        type: 'success',
      });
    } catch (error) {
      notifyError(error);
    }
  },
);

function editIntegration(integration: Integration) {
  selectedIntegration.value = cloneDeep(integration);
  isDialogOpen.value = true;
}

function showAddIntegration() {
  selectedIntegration.value = cloneDeep(emptyIntegration);
  isDialogOpen.value = true;
}

function handleDialogClose() {
  selectedIntegration.value = undefined;
  isDialogOpen.value = false;
}

useCrowTitle(computed(() => [t('integrations.title')]));
</script>

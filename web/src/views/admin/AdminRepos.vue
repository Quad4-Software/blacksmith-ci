<template>
  <Settings :title="$t('admin.settings.repos.repos')" :description="$t('admin.settings.repos.desc')">
    <template #headerActions>
      <Button :disabled="isRepairingRepos" @click="repairRepos">
        {{ $t('admin.settings.repos.repair.repair') }}
      </Button>
    </template>

    <div class="space-y-4">
      <RepoTable
        :repos="data"
        :search-term="searchTerm"
        @update:search-term="searchTerm = $event"
        @bulk-disable="handleBulkDisable"
        @bulk-delete="handleBulkDelete"
      />

      <!-- Pagination Controls -->
      <div v-if="data && data.length > 0" class="flex items-center justify-center gap-2">
        <Button variant="outline" size="sm" :disabled="page === 1 || loading" @click="previousPage">
          <Icon name="chevron-left" class="h-4 w-4" />
          {{ $t('admin.settings.entities.previous') }}
        </Button>
        <span class="text-sm text-muted-foreground"> {{ $t('admin.settings.entities.page') }} {{ page }} </span>
        <Button variant="outline" size="sm" :disabled="!hasMore || loading" @click="nextPage">
          {{ $t('admin.settings.entities.next') }}
          <Icon name="chevron-right" class="h-4 w-4" />
        </Button>
      </div>
    </div>
  </Settings>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import RepoTable from '~/components/admin/RepoTable.vue';
import Settings from '~/components/layout/Settings.vue';
import { Button } from '~/components/ui/button';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useNotifications from '~/compositions/useNotifications';
import { usePaginationWithPages } from '~/compositions/usePaginate';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { Repo } from '~/lib/api/types';

const apiClient = useApiClient();
const notifications = useNotifications();
const { t } = useI18n();

const searchTerm = ref('');

async function loadRepos(page: number): Promise<Repo[] | null> {
  return apiClient.getAllRepos({ page });
}

const { page, data, hasMore, loading, nextPage, previousPage, resetPage } = usePaginationWithPages(loadRepos, {
  pageSize: 15,
});

// Reload when search changes
watch(searchTerm, async () => {
  await resetPage();
});

const { doSubmit: repairRepos, isLoading: isRepairingRepos } = useAsyncAction(async () => {
  await apiClient.repairAllRepos();
  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.repos.repair.notify_text_success'),
    type: 'success',
    duration: 6000,
  });
});

async function handleBulkDisable(repos: Repo[]) {
  let successCount = 0;
  const failedRepos: string[] = [];

  for (const repo of repos) {
    try {
      await apiClient.deleteRepo(repo.id, false); // false = disable, not delete
      successCount++;
    } catch (error) {
      failedRepos.push(repo.full_name);
      console.error('Failed to disable repo:', repo.full_name, error);
    }
  }

  if (successCount > 0) {
    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.repos.bulk_disable_success', { count: successCount }),
      type: 'success',
    });
  }

  if (failedRepos.length > 0) {
    notifications.notify({
      title: t('notify_title_error'),
      text: `${t('admin.settings.repos.bulk_disable_failed', { count: failedRepos.length })}: ${failedRepos.join(', ')}`,
      type: 'error',
      duration: 10000,
    });
  }

  await resetPage();
}

async function handleBulkDelete(repos: Repo[]) {
  let successCount = 0;
  const failedRepos: string[] = [];

  for (const repo of repos) {
    try {
      await apiClient.deleteRepo(repo.id, true); // true = delete completely
      successCount++;
    } catch (error) {
      failedRepos.push(repo.full_name);
      console.error('Failed to delete repo:', repo.full_name, error);
    }
  }

  if (successCount > 0) {
    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.repos.bulk_delete_success', { count: successCount }),
      type: 'success',
    });
  }

  if (failedRepos.length > 0) {
    notifications.notify({
      title: t('notify_title_error'),
      text: `${t('admin.settings.repos.bulk_delete_failed', { count: failedRepos.length })}: ${failedRepos.join(', ')}`,
      type: 'error',
      duration: 10000,
    });
  }

  await resetPage();
}

useCrowTitle(computed(() => [t('admin.settings.repos.repos'), t('admin.settings.settings')]));
</script>

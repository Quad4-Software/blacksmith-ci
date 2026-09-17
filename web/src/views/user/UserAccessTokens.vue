<template>
  <Scaffold fill-width>
    <template #title>
      {{ $t('access_tokens.title') }}
    </template>

    <template #headerActions>
      <div class="flex items-center gap-3 flex-wrap">
        <AccessTokenDialog
          v-model:open="isDialogOpen"
          :created-token="createdToken"
          :is-saving="isSaving"
          @save="createToken"
          @cancel="handleDialogClose"
        >
          <template #trigger>
            <Button size="sm">
              <Icon name="plus" class="mr-2 h-4 w-4" />
              {{ $t('access_tokens.create') }}
            </Button>
          </template>
        </AccessTokenDialog>
      </div>
    </template>

    <div class="space-y-6 max-w-7xl mx-auto">
      <!-- Info Alert -->
      <Alert>
        <Icon name="ph:info" class="h-4 w-4" />
        <AlertDescription>
          {{ $t('access_tokens.info') }}
        </AlertDescription>
      </Alert>

      <!-- Token Table -->
      <AccessTokenTable
        :tokens="tokens"
        :is-deleting="isDeleting"
        :is-updating="isUpdating"
        @delete="deleteToken"
        @bulk-delete="bulkDeleteTokens"
        @update="updateToken"
      />
    </div>
  </Scaffold>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import AccessTokenDialog from '~/components/access-tokens/AccessTokenDialog.vue';
import AccessTokenTable from '~/components/access-tokens/AccessTokenTable.vue';
import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import { Alert, AlertDescription } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import useNotifications from '~/compositions/useNotifications';
import { usePagination } from '~/compositions/usePaginate';
import type { AccessTokenWithSecret, CreateAccessTokenRequest, UpdateAccessTokenRequest } from '~/lib/api/types';

const apiClient = useApiClient();
const notifications = useNotifications();
const { t } = useI18n();

const isDialogOpen = ref(false);
const createdToken = ref<AccessTokenWithSecret | null>(null);

async function loadTokens(page: number) {
  return apiClient.getAccessTokenList({ page });
}

const { resetPage, data: tokens } = usePagination(loadTokens, () => true);

const { doSubmit: createToken, isLoading: isSaving } = useAsyncAction(async (data: CreateAccessTokenRequest) => {
  const result = await apiClient.createAccessToken(data);
  createdToken.value = result;

  notifications.notify({
    title: t('notify_title_success'),
    text: t('access_tokens.notify_created'),
    type: 'success',
  });

  await resetPage();
});

const { doSubmit: deleteToken, isLoading: isDeleting } = useAsyncAction(async (tokenId: number) => {
  await apiClient.deleteAccessToken(tokenId);

  notifications.notify({
    title: t('notify_title_success'),
    text: t('access_tokens.notify_deleted'),
    type: 'success',
  });

  await resetPage();
});

const { doSubmit: bulkDeleteTokens } = useAsyncAction(async (tokenIds: number[]) => {
  await apiClient.deleteAccessTokenBatch(tokenIds);

  notifications.notify({
    title: t('notify_title_success'),
    text: t('access_tokens.notify_bulk_deleted', { count: tokenIds.length }),
    type: 'success',
  });

  await resetPage();
});

const { doSubmit: updateToken, isLoading: isUpdating } = useAsyncAction(
  async (tokenId: number, data: UpdateAccessTokenRequest) => {
    await apiClient.updateAccessToken(tokenId, data);

    notifications.notify({
      title: t('notify_title_success'),
      text: t('access_tokens.notify_updated'),
      type: 'success',
    });

    await resetPage();
  },
);

function handleDialogClose() {
  createdToken.value = null;
  isDialogOpen.value = false;
}

useCrowTitle(computed(() => [t('access_tokens.title')]));
</script>

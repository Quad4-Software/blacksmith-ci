<template>
  <Settings
    :title="$t('secrets.secrets')"
    :description="$t('admin.settings.secrets.desc')"
    docs-url="docs/usage/secrets"
  >
    <template #headerActions>
      <SecretDialog
        v-model:open="isDialogOpen"
        :secret="selectedSecret"
        :is-saving="isSaving"
        lock-scope
        @save="createSecret"
        @cancel="handleDialogClose"
      >
        <template #trigger>
          <Button size="sm" @click="showAddSecret">
            <Icon name="plus" class="mr-2 h-4 w-4" />
            {{ $t('secrets.add') }}
          </Button>
        </template>
      </SecretDialog>
    </template>
    <template #headerEnd> <Warning class="mt-4 text-sm" :text="$t('admin.settings.secrets.warning')" /> </template>
    <SecretListGrouped
      context="admin"
      :secrets="secrets"
      :is-deleting="isDeleting || isBulkDeleting"
      :search-term="searchTerm"
      @edit="editSecret"
      @delete="deleteSecret"
      @bulk-delete="bulkDeleteSecrets"
      @update:search-term="searchTerm = $event"
      @update-events="updateSecretEvents"
    />
  </Settings>
</template>

<script lang="ts" setup>
import { cloneDeep } from 'lodash';
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import Warning from '~/components/atomic/Warning.vue';
import Settings from '~/components/layout/Settings.vue';
import SecretDialog from '~/components/secrets/SecretDialog.vue';
import SecretListGrouped from '~/components/secrets/SecretListGrouped.vue';
import { Button } from '~/components/ui/button';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import useNotifications from '~/compositions/useNotifications';
import { usePagination } from '~/compositions/usePaginate';
import { toScopedSecrets } from '~/lib/secret-scope';
import { WebhookEvents } from '~/lib/api/types';
import type { Secret, SecretWithOrg } from '~/lib/api/types';

const emptySecret: Partial<SecretWithOrg> = {
  name: '',
  value: '',
  images: [],
  events: [WebhookEvents.Push],
  scope_type: 'global',
  source: 'internal',
};

const apiClient = useApiClient();
const notifications = useNotifications();
const i18n = useI18n();

const searchTerm = ref('');
const isDialogOpen = ref(false);
const selectedSecret = ref<Partial<SecretWithOrg>>();

async function loadSecrets(page: number): Promise<Secret[] | null> {
  return apiClient.getGlobalSecretList({ page });
}

const { resetPage, data: _secrets } = usePagination(loadSecrets, () => !isDialogOpen.value);

const secrets = computed(() => toScopedSecrets(_secrets.value, { orgName: '' }));

const { doSubmit: createSecret, isLoading: isSaving } = useAsyncAction(
  async (secret: Partial<Secret>) => {
    if (secret.id) {
      await apiClient.updateGlobalSecret(secret);
    } else {
      await apiClient.createGlobalSecret(secret);
    }
    notifications.notify({
      title: i18n.t('notify_title_success'),
      text: secret.id ? i18n.t('secrets.notify_text_saved') : i18n.t('secrets.notify_text_created'),
      type: 'success',
      duration: 6000,
    });
    handleDialogClose();
    await resetPage();
  },
  // eslint-disable-next-line promise/prefer-await-to-callbacks -- useAsyncAction API requires callback
  (error) => {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      text: error instanceof Error ? error.message : i18n.t('unknown_error'),
      type: 'error',
    });
  },
);

const { doSubmit: deleteSecret, isLoading: isDeleting } = useAsyncAction(async (secret: SecretWithOrg) => {
  await apiClient.deleteGlobalSecret(secret.name);
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('secrets.notify_text_deleted'),
    type: 'success',
    duration: 6000,
  });
  await resetPage();
});

const { doSubmit: bulkDeleteSecrets, isLoading: isBulkDeleting } = useAsyncAction(async (secrets: SecretWithOrg[]) => {
  for (const secret of secrets) {
    await apiClient.deleteGlobalSecret(secret.name);
  }
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('secrets.notify_text_deleted'),
    type: 'success',
    duration: 6000,
  });
  await resetPage();
});

const { doSubmit: updateSecretEvents } = useAsyncAction(
  async (secret: SecretWithOrg, events: WebhookEvents[]) => {
    await apiClient.updateGlobalSecret({
      id: secret.id,
      name: secret.name,
      value: secret.value,
      images: secret.images,
      events,
      source: secret.source || 'internal',
    });
    notifications.notify({
      title: i18n.t('notify_title_success'),
      text: i18n.t('secrets.notify_text_saved'),
      type: 'success',
      duration: 6000,
    });
    await resetPage();
  },
  // eslint-disable-next-line promise/prefer-await-to-callbacks -- useAsyncAction API requires callback
  (error) => {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      text: error instanceof Error ? error.message : i18n.t('unknown_error'),
      type: 'error',
    });
  },
);

function editSecret(secret: SecretWithOrg) {
  selectedSecret.value = cloneDeep(secret);
  isDialogOpen.value = true;
}

function showAddSecret() {
  selectedSecret.value = cloneDeep(emptySecret);
  isDialogOpen.value = true;
}

function handleDialogClose() {
  selectedSecret.value = undefined;
  isDialogOpen.value = false;
}

useCrowTitle(computed(() => [i18n.t('secrets.secrets'), i18n.t('admin.settings.settings')]));
</script>

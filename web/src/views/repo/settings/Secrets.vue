<template>
  <Settings :title="$t('secrets.secrets')" :description="$t('secrets.desc')" docs-url="docs/usage/secrets">
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
    <SecretListGrouped
      context="repo"
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
import Settings from '~/components/layout/Settings.vue';
import SecretDialog from '~/components/secrets/SecretDialog.vue';
import SecretListGrouped from '~/components/secrets/SecretListGrouped.vue';
import { Button } from '~/components/ui/button';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import { usePagination } from '~/compositions/usePaginate';
import { mergeRepoSecrets } from '~/lib/secret-scope';
import { WebhookEvents } from '~/lib/api/types';
import type { Secret, SecretWithOrg } from '~/lib/api/types';

const apiClient = useApiClient();
const notifications = useNotifications();
const i18n = useI18n();

const repo = requiredInject('repo');
const searchTerm = ref('');
const isDialogOpen = ref(false);
const selectedSecret = ref<Partial<SecretWithOrg>>();

const emptySecret = computed<Partial<SecretWithOrg>>(() => ({
  name: '',
  value: '',
  images: [],
  events: [WebhookEvents.Push],
  scope_type: 'repo',
  repo_id: repo.value.id,
  source: 'internal',
}));

async function loadSecrets(page: number, level: 'repo' | 'org' | 'global'): Promise<Secret[] | null> {
  switch (level) {
    case 'repo':
      return apiClient.getSecretList(repo.value.id, { page });
    case 'org':
      return apiClient.getOrgSecretList(repo.value.org_id, { page });
    case 'global':
      return apiClient.getGlobalSecretList({ page });
    default:
      throw new Error(`Unexpected level: ${level}`);
  }
}

const { resetPage, data: _secrets } = usePagination(loadSecrets, () => !isDialogOpen.value, {
  each: ['repo', 'org', 'global'],
});

const secrets = computed(() =>
  mergeRepoSecrets(_secrets.value, { repoName: repo.value.full_name, orgName: repo.value.owner }),
);

const { doSubmit: createSecret, isLoading: isSaving } = useAsyncAction(
  async (secret: Partial<Secret>) => {
    if (secret.id) {
      await apiClient.updateSecret(repo.value.id, secret);
    } else {
      await apiClient.createSecret(repo.value.id, secret);
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
  await apiClient.deleteSecret(repo.value.id, secret.name);
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
    await apiClient.deleteSecret(repo.value.id, secret.name);
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
    await apiClient.updateSecret(repo.value.id, {
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
  selectedSecret.value = cloneDeep(emptySecret.value);
  isDialogOpen.value = true;
}

function handleDialogClose() {
  selectedSecret.value = undefined;
  isDialogOpen.value = false;
}

useCrowTitle(computed(() => [i18n.t('secrets.secrets'), repo.value.full_name]));
</script>

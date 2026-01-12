<template>
  <SecretDialog
    v-model:open="secretDialog.isOpen.value"
    :secret="emptySecret"
    :is-saving="isSaving"
    :hide-trigger="true"
    @save="handleSave"
    @cancel="handleCancel"
  />
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';

import SecretDialog from '~/components/secrets/SecretDialog.vue';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useAuthentication from '~/compositions/useAuthentication';
import { useGlobalDialogs } from '~/compositions/useGlobalDialogs';
import useNotifications from '~/compositions/useNotifications';
import { WebhookEvents } from '~/lib/api/types';
import type { SecretWithOrg } from '~/lib/api/types';

const emptySecret: Partial<SecretWithOrg> = {
  name: '',
  value: '',
  images: [],
  events: [WebhookEvents.Push],
  scope_type: 'user',
  source: 'internal',
};

const { t } = useI18n();
const apiClient = useApiClient();
const notifications = useNotifications();
const { user } = useAuthentication();
const { secretDialog } = useGlobalDialogs();

const { doSubmit: handleSave, isLoading: isSaving } = useAsyncAction(
  async (secretData: Partial<SecretWithOrg>) => {
    if (!secretData || !user) {
      throw new Error("Unexpected: Can't get secret or user");
    }

    const secret = secretData;

    // Determine target org based on scope
    let targetOrgId = user.org_id;
    if (secret.scope_type === 'global') {
      // Use global endpoint
      await apiClient.createGlobalSecret(secret);
    } else if (secret.scope_type === 'repo') {
      // Repo secrets
      if (!secret.repo_id) {
        throw new Error('Repo ID not found for repo secret');
      }
      await apiClient.createSecret(secret.repo_id, secret);
    } else if (secret.scope_type === 'org' && secret.org_id) {
      targetOrgId = secret.org_id;
      await apiClient.createOrgSecret(targetOrgId, secret);
    } else {
      // User/personal org
      await apiClient.createOrgSecret(targetOrgId, secret);
    }

    notifications.notify({
      title: t('notify_title_success'),
      text: t('secrets.notify_text_created'),
      type: 'success',
    });

    secretDialog.close();
  },
  // eslint-disable-next-line promise/prefer-await-to-callbacks -- useAsyncAction API requires callback
  (error) => {
    notifications.notify({
      title: t('notify_title_error'),
      text: error instanceof Error ? error.message : t('unknown_error'),
      type: 'error',
    });
  },
);

function handleCancel() {
  secretDialog.close();
}
</script>

<template>
  <Scaffold fill-width>
    <template #title>
      {{ $t('secrets.all_secrets') }}
    </template>

    <template #headerActions>
      <div class="flex items-center gap-3 flex-wrap">
        <SecretDialog
          v-model:open="isDialogOpen"
          :secret="selectedSecret"
          :is-saving="isSaving"
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
      </div>
    </template>

    <div class="max-w-7xl mx-auto space-y-4">
      <SecretListGrouped
        :secrets="secrets"
        :is-deleting="isDeleting"
        :search-term="searchTerm"
        @edit="editSecret"
        @delete="deleteSecret"
        @update:search-term="searchTerm = $event"
        @update-events="updateSecretEvents"
      />
    </div>
  </Scaffold>
</template>

<script lang="ts" setup>
import { cloneDeep } from 'lodash';
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import SecretDialog from '~/components/secrets/SecretDialog.vue';
import SecretListGrouped from '~/components/secrets/SecretListGrouped.vue';
import { Button } from '~/components/ui/button';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useAuthentication from '~/compositions/useAuthentication';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import useNotifications from '~/compositions/useNotifications';
import { usePagination } from '~/compositions/usePaginate';
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

const apiClient = useApiClient();
const notifications = useNotifications();
const { t } = useI18n();
const { user } = useAuthentication();

const searchTerm = ref('');
const isDialogOpen = ref(false);
const selectedSecret = ref<Partial<SecretWithOrg>>();

async function loadSecrets(page: number) {
  return apiClient.getUserSecretList({
    scope: 'user,org,global,repo',
    search: searchTerm.value,
    page,
  });
}

const { resetPage, data: secrets } = usePagination(loadSecrets, () => true);

// Reload when search changes
watch(searchTerm, () => {
  resetPage();
});

const { doSubmit: createSecret, isLoading: isSaving } = useAsyncAction(async (secretData: Partial<SecretWithOrg>) => {
  if (!secretData || !user) {
    throw new Error("Unexpected: Can't get secret or user");
  }

  // Use the passed secretData instead of selectedSecret.value
  const secret = secretData;
  const isEditing = !!secret.id;

  // Determine target org based on scope
  let targetOrgId = user.org_id;
  if (secret.scope_type === 'global') {
    // Use global endpoint
    if (isEditing) {
      await apiClient.updateGlobalSecret(secret);
    } else {
      await apiClient.createGlobalSecret(secret);
    }
  } else if (secret.scope_type === 'repo') {
    // Repo secrets
    if (!secret.repo_id) {
      throw new Error('Repo ID not found for repo secret');
    }
    if (isEditing) {
      await apiClient.updateSecret(secret.repo_id, secret);
    } else {
      await apiClient.createSecret(secret.repo_id, secret);
    }
  } else if (secret.scope_type === 'org' && secret.org_id) {
    targetOrgId = secret.org_id;
    if (isEditing) {
      await apiClient.updateOrgSecret(targetOrgId, secret);
    } else {
      await apiClient.createOrgSecret(targetOrgId, secret);
    }
  } else {
    // User/personal org
    if (isEditing) {
      await apiClient.updateOrgSecret(targetOrgId, secret);
    } else {
      await apiClient.createOrgSecret(targetOrgId, secret);
    }
  }

  notifications.notify({
    title: t('notify_title_success'),
    text: isEditing ? t('secrets.notify_text_saved') : t('secrets.notify_text_created'),
    type: 'success',
  });

  isDialogOpen.value = false;
  selectedSecret.value = undefined;
  resetPage();
});

const { doSubmit: deleteSecret, isLoading: isDeleting } = useAsyncAction(async (secret: SecretWithOrg) => {
  if (!user) {
    throw new Error('Unexpected: User not found');
  }

  if (secret.scope_type === 'global') {
    await apiClient.deleteGlobalSecret(secret.name);
  } else if (secret.scope_type === 'repo') {
    if (!secret.repo_id) {
      throw new Error('Repo ID not found for repo secret');
    }
    await apiClient.deleteSecret(secret.repo_id, secret.name);
  } else {
    // org or user scope
    const targetOrgId = secret.org_id || user.org_id;
    await apiClient.deleteOrgSecret(targetOrgId, secret.name);
  }

  notifications.notify({
    title: t('notify_title_success'),
    text: t('secrets.notify_text_deleted'),
    type: 'success',
  });

  resetPage();
});

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

async function updateSecretEvents(secret: SecretWithOrg, events: WebhookEvents[]) {
  if (!user) {
    throw new Error('Unexpected: User not found');
  }

  // Extract only Secret fields for API update
  const secretUpdate = {
    id: secret.id,
    name: secret.name,
    value: secret.value,
    images: secret.images,
    events,
    source: secret.source || 'internal',
  };

  try {
    if (secret.scope_type === 'global') {
      await apiClient.updateGlobalSecret(secretUpdate);
    } else if (secret.scope_type === 'repo') {
      if (!secret.repo_id) {
        throw new Error('Repo ID not found for repo secret');
      }
      await apiClient.updateSecret(secret.repo_id, secretUpdate);
    } else {
      // org or user scope
      const targetOrgId = secret.org_id || user.org_id;
      await apiClient.updateOrgSecret(targetOrgId, secretUpdate);
    }

    notifications.notify({
      title: t('notify_title_success'),
      text: t('secrets.notify_text_saved'),
      type: 'success',
    });

    resetPage();
  } catch {
    notifications.notify({
      title: t('notify_title_error'),
      text: t('unknown_error'),
      type: 'error',
    });
  }
}

useCrowTitle(computed(() => [t('secrets.all_secrets')]));
</script>

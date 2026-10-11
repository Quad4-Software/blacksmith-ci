<template>
  <div class="space-y-4">
    <div>
      <h3 class="text-lg text-foreground">{{ $t('user.settings.sso.title') }}</h3>
      <p class="text-sm text-muted-foreground">{{ $t('user.settings.sso.desc', { name: config.auth.oidc.name }) }}</p>
    </div>
    <div v-if="loading" class="flex items-center text-sm text-muted-foreground">
      <Icon name="spinner" class="h-4 w-4 animate-spin" />
    </div>
    <div v-else-if="identity" class="flex items-center justify-between gap-4 rounded-md border border-border p-3">
      <div class="text-sm">
        <div class="font-medium text-foreground">{{ identity.username || identity.subject }}</div>
        <div v-if="identity.email" class="text-muted-foreground">{{ identity.email }}</div>
      </div>
      <div class="flex flex-col items-end gap-1">
        <Button variant="outline" size="sm" :disabled="unlinking || !canUnlink" @click="showUnlinkDialog = true">
          {{ $t('user.settings.sso.unlink') }}
        </Button>
        <p v-if="!canUnlink" class="max-w-xs text-right text-xs text-muted-foreground">
          {{ $t('user.settings.sso.unlink_needs_forge') }}
        </p>
      </div>
    </div>
    <Button v-else variant="outline" size="sm" as="a" :href="`${config.rootPath}/authorize/oidc`">
      {{ $t('user.settings.sso.link', { name: config.auth.oidc.name }) }}
    </Button>
  </div>

  <AlertDialog v-model:open="showUnlinkDialog">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('user.settings.sso.unlink_confirm_title') }}</AlertDialogTitle>
        <AlertDialogDescription>{{ $t('user.settings.sso.unlink_confirm_desc') }}</AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
        <AlertDialogAction :disabled="unlinking" @click.prevent="unlink">
          <Icon v-if="unlinking" name="spinner" class="mr-1.5 h-4 w-4 animate-spin" />
          {{ $t('user.settings.sso.unlink') }}
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
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
import { Button } from '~/components/ui/button';
import useApiClient from '~/compositions/useApiClient';
import useConfig from '~/compositions/useConfig';
import useNotifications from '~/compositions/useNotifications';
import type { UserIdentity } from '~/lib/api/types';

const config = useConfig();
const apiClient = useApiClient();
const notifications = useNotifications();
const route = useRoute();
const router = useRouter();
const { t } = useI18n();

const identities = ref<UserIdentity[]>([]);
const identity = computed(() => identities.value.find((i) => i.provider === 'oidc'));
const loading = ref(true);
const unlinking = ref(false);
const showUnlinkDialog = ref(false);
// Unlinking single sign-on must leave a way to sign in: a linked forge that
// forge sign-in is enabled for.
const canUnlink = computed(() => config.auth.forge_count !== 0 && config.auth.forge_login);

function queryErrorMessage(code: unknown): string | undefined {
  if (code === 'identity_taken') return t('user.settings.sso.identity_taken');
  if (code === 'identity_already_linked') return t('user.settings.sso.identity_already_linked');
  if (code === 'forge_sign_in_required') return t('user.settings.sso.forge_sign_in_required');
  if (code === 'oidc_error') return t('user.settings.sso.link_failed');
  if (code === 'oidc_access_denied') return t('user.settings.sso.link_access_denied');
  return undefined;
}

async function load() {
  loading.value = true;
  try {
    identities.value = (await apiClient.getUserIdentities()) ?? [];
  } finally {
    loading.value = false;
  }
}

async function unlink() {
  if (!identity.value) return;
  unlinking.value = true;
  try {
    await apiClient.deleteUserIdentity(identity.value.id);
    notifications.notify({
      title: t('notify_title_success'),
      text: t('user.settings.sso.unlink_success'),
      type: 'success',
    });
    showUnlinkDialog.value = false;
    await load();
  } catch (error) {
    notifications.notify({
      title: t('notify_title_error'),
      text: error instanceof Error && error.message !== '' ? error.message : t('user.settings.sso.unlink_failed'),
      type: 'error',
    });
  } finally {
    unlinking.value = false;
  }
}

onMounted(async () => {
  const error = queryErrorMessage(route.query.error);
  if (route.query.identity_linked === 'true') {
    notifications.notify({
      title: t('notify_title_success'),
      text: t('user.settings.sso.link_success'),
      type: 'success',
    });
    await router.replace({ query: {} });
  } else if (error) {
    notifications.notify({ title: t('notify_title_error'), text: error, type: 'error' });
    await router.replace({ query: {} });
  }
  await load();
});
</script>

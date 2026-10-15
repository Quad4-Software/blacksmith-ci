<template>
  <div class="space-y-4">
    <div>
      <h3 class="text-lg text-foreground">{{ $t('user.settings.sso.title') }}</h3>
      <p class="text-sm text-muted-foreground">{{ $t('user.settings.sso.desc') }}</p>
    </div>
    <div v-if="loading" class="flex items-center text-sm text-muted-foreground">
      <Icon name="spinner" class="h-4 w-4 animate-spin" />
    </div>
    <template v-else>
      <div v-for="provider in providers" :key="provider.key" :data-provider="provider.key">
        <div
          v-if="provider.identity"
          class="flex items-center justify-between gap-4 rounded-md border border-border p-3"
        >
          <div class="text-sm">
            <div class="text-xs text-muted-foreground">{{ provider.name }}</div>
            <div class="font-medium text-foreground">{{ provider.identity.username || provider.identity.subject }}</div>
            <div v-if="provider.identity.email" class="text-muted-foreground">{{ provider.identity.email }}</div>
          </div>
          <div class="flex flex-col items-end gap-1">
            <Button
              variant="outline"
              size="sm"
              :disabled="unlinking || !canUnlink"
              @click="unlinkTarget = provider.identity"
            >
              {{ $t('user.settings.sso.unlink') }}
            </Button>
            <p v-if="!canUnlink" class="max-w-xs text-right text-xs text-muted-foreground">
              {{ $t('user.settings.sso.unlink_needs_forge') }}
            </p>
          </div>
        </div>
        <Button
          v-else-if="provider.key === 'oidc'"
          variant="outline"
          size="sm"
          as="a"
          :href="`${config.rootPath}/authorize/oidc`"
        >
          {{ $t('user.settings.sso.link', { name: provider.name }) }}
        </Button>
        <Button v-else variant="outline" size="sm" @click="showLdapDialog = true">
          {{ $t('user.settings.sso.link', { name: provider.name }) }}
        </Button>
      </div>
    </template>
  </div>

  <LdapLinkDialog v-model:open="showLdapDialog" :name="config.auth.ldap.name" @linked="onLdapLinked" />

  <AlertDialog :open="unlinkTarget !== null" @update:open="(value) => !value && (unlinkTarget = null)">
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
import LdapLinkDialog from '~/components/user/LdapLinkDialog.vue';
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
const providers = computed(() => {
  const list: { key: 'oidc' | 'ldap'; name: string; identity?: UserIdentity }[] = [];
  if (config.auth.oidc.enabled) {
    list.push({
      key: 'oidc',
      name: config.auth.oidc.name,
      identity: identities.value.find((i) => i.provider === 'oidc'),
    });
  }
  if (config.auth.ldap.enabled) {
    list.push({
      key: 'ldap',
      name: config.auth.ldap.name,
      identity: identities.value.find((i) => i.provider === 'ldap'),
    });
  }
  return list;
});
const loading = ref(true);
const unlinking = ref(false);
const unlinkTarget = ref<UserIdentity | null>(null);
const showLdapDialog = ref(false);
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

async function onLdapLinked() {
  notifications.notify({
    title: t('notify_title_success'),
    text: t('user.settings.sso.link_success'),
    type: 'success',
  });
  await load();
}

async function unlink() {
  if (!unlinkTarget.value) return;
  unlinking.value = true;
  try {
    await apiClient.deleteUserIdentity(unlinkTarget.value.id);
    notifications.notify({
      title: t('notify_title_success'),
      text: t('user.settings.sso.unlink_success'),
      type: 'success',
    });
    unlinkTarget.value = null;
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

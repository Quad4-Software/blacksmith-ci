<!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
<template>
  <Scaffold enable-tabs :go-back="goBack">
    <template #title>
      <span>
        <router-link
          :to="{ name: 'org', params: { orgId: repo!.org_id } }"
          class="hover:underline hover:decoration-primary"
          >{{ repo.owner }}</router-link
        >
        /
        <router-link :to="{ name: 'repo' }" class="hover:underline hover:decoration-primary">{{
          repo!.name
          /* eslint-disable-next-line @intlify/vue-i18n/no-raw-text */
        }}</router-link>
        / {{ $t('repo.settings.settings') }}
      </span>
    </template>
    <template v-if="repoPermissions.admin">
      <Tab
        icon="settings-outline"
        icon-size="1rem"
        :to="{ name: 'repo-settings' }"
        :title="$t('repo.settings.general.general')"
      />
      <Tab icon="secret" icon-size="1rem" :to="{ name: 'repo-settings-secrets' }" :title="$t('secrets.secrets')" />
      <Tab
        icon="docker"
        icon-size="1rem"
        :to="{ name: 'repo-settings-registries' }"
        :title="$t('registries.registries')"
      />
      <Tab
        icon="cron"
        icon-size="1rem"
        :to="{ name: 'repo-settings-crons' }"
        :title="$t('repo.settings.crons.crons')"
      />
      <Tab
        icon="cache"
        icon-size="1rem"
        :to="{ name: 'repo-settings-caches' }"
        :title="$t('repo.settings.caches.caches')"
      />
      <Tab icon="tag" icon-size="1rem" :to="{ name: 'repo-settings-badge' }" :title="$t('repo.settings.badge.badge')" />
    </template>
    <Tab
      icon="email-outline"
      icon-size="1rem"
      :to="{ name: 'repo-settings-notifications' }"
      :title="$t('repo.settings.notifications.notifications')"
    />

    <router-view />
  </Scaffold>
</template>

<script lang="ts" setup>
import { onMounted, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';

import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import Tab from '~/components/layout/scaffold/Tab.vue';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import { useRouteBack } from '~/compositions/useRouteBack';

defineProps<{
  repoId: string;
}>();

const notifications = useNotifications();
const route = useRoute();
const router = useRouter();
const i18n = useI18n();

const repoPermissions = requiredInject('repo-permissions');
const repo = requiredInject('repo');

async function guard() {
  if (route.name === 'repo-settings-notifications' && repoPermissions.value.pull) {
    return;
  }
  if (!repoPermissions.value.admin) {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      type: 'error',
      duration: 9000,
      text: i18n.t('repo.settings.notify_text_not_allowed'),
    });
    await router.replace({ name: 'home' });
  }
}

onMounted(guard);
watch(() => route.name, guard);

const goBack = useRouteBack({ name: 'repo' });
</script>

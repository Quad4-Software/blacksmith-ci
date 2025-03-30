<!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
<template>
  <Scaffold enable-tabs :go-back="goBack">
    <template #title>
      <span>
        <router-link
          :to="{ name: 'org', params: { orgId: repo!.org_id } }"
          class="hover:underline hover:decoration-wp-custom-highlight-100"
          >{{ repo!.owner }}</router-link
        >
        /
        <router-link :to="{ name: 'repo' }" class="hover:underline hover:decoration-wp-custom-highlight-100">{{
          repo!.name
          /* eslint-disable-next-line @intlify/vue-i18n/no-raw-text */
        }}</router-link>
        / {{ $t('settings') }}
      </span>
    </template>
    <Tab
      icon="settings-outline"
      icon-size="1rem"
      :to="{ name: 'repo-settings' }"
      :title="$t('repo.settings.general.general')"
    />
    <Tab icon="secret" icon-size="1rem" :to="{ name: 'repo-settings-secrets' }" :title="$t('secrets.secrets')" />
    <Tab icon="docker" :to="{ name: 'repo-settings-registries' }" :title="$t('registries.registries')" />
    <Tab icon="cron" icon-size="1rem" :to="{ name: 'repo-settings-crons' }" :title="$t('repo.settings.crons.crons')" />
    <Tab icon="tag" icon-size="1rem" :to="{ name: 'repo-settings-badge' }" :title="$t('repo.settings.badge.badge')" />
    <Tab
      icon="toolbox"
      icon-size="1rem"
      :to="{ name: 'repo-settings-actions' }"
      :title="$t('repo.settings.actions.actions')"
    />

    <router-view />
  </Scaffold>
</template>

<script lang="ts" setup>
import { inject, onMounted } from 'vue';
import type { Ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import Tab from '~/components/layout/scaffold/Tab.vue';
import useNotifications from '~/compositions/useNotifications';
import { useRouteBack } from '~/compositions/useRouteBack';
import type { Repo, RepoPermissions } from '~/lib/api/types';

const notifications = useNotifications();
const router = useRouter();
const i18n = useI18n();

const repoPermissions = inject<Ref<RepoPermissions>>('repo-permissions');
if (!repoPermissions) {
  throw new Error('Unexpected: "repoPermissions" should be provided at this place');
}

const repo = inject<Ref<Repo>>('repo');
if (!repo) {
  throw new Error('Unexpected: "repo" should be provided at this place');
}

onMounted(async () => {
  if (!repoPermissions.value.admin) {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      type: 'error',
      duration: 9000,
      text: i18n.t('repo.settings.notify_text_not_allowed'),
    });
    await router.replace({ name: 'home' });
  }
});

const goBack = useRouteBack({ name: 'repo' });
</script>

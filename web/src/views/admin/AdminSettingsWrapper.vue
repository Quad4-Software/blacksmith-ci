<template>
  <Scaffold enable-tabs>
    <template #title>
      {{ $t('admin.settings.settings') }}
    </template>
    <template #titleSuffix>
      <div class="ml-2 flex items-center space-x-2">
        <!-- Dev version badge -->
        <a
          v-if="isDevVersion"
          :href="versionUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="relative flex shrink-0 cursor-pointer items-center overflow-hidden whitespace-nowrap rounded-md border border-wp-custom-highlight-100 px-2 py-0.5 shadow-xs transition-all duration-150 hover:border-wp-orange-300 hover:bg-wp-orange-300 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:border-wp-custom-hover-100 dark:hover:bg-wp-custom-hover-100 bg-wp-control-neutral-100 text-wp-text-100"
        >
          {{ currentVersion }}
        </a>
        <!-- Semver release badge -->
        <a
          v-else-if="isSemverVersion"
          :href="versionUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="relative flex shrink-0 cursor-pointer items-center overflow-hidden whitespace-nowrap rounded-md border border-wp-custom-highlight-100 bg-wp-control-neutral-100 px-2 py-0.5 text-sm shadow-xs transition-all duration-150 text-wp-text-100 hover:border-wp-orange-300 hover:bg-wp-orange-300 dark:hover:border-wp-custom-hover-100 dark:hover:bg-wp-custom-hover-100"
        >
          {{ currentVersion }}
        </a>
        <!-- Other version badge (non-clickable) -->
        <span
          v-else-if="currentVersion && currentVersion !== 'unknown'"
          class="relative flex shrink-0 items-center overflow-hidden whitespace-nowrap rounded-md border border-wp-control-neutral-300 bg-wp-control-neutral-100 px-2 py-0.5 text-sm shadow-xs text-wp-text-100"
        >
          {{ currentVersion }}
        </span>
        <!-- Update available badge -->
        <a
          v-if="version?.needsUpdate && version?.latest"
          :href="
            version.usesDev
              ? 'https://codeberg.org/crowci/crow/commits/branch/main'
              : `https://codeberg.org/crowci/crow/releases/tag/${version.latest}`
          "
          target="_blank"
          rel="noopener noreferrer"
          class="relative flex shrink-0 cursor-pointer items-center overflow-hidden whitespace-nowrap rounded-md border border-wp-background-400 bg-wp-background-100 px-2 py-0.5 text-sm transition-all duration-150 text-wp-text-100 hover:bg-wp-control-neutral-200 hover:text-wp-text-100 dark:border-wp-control-neutral-500 dark:hover:text-wp-text-200"
        >
          {{ version.latest?.replace(/^v/, '') }}
          {{ $t('admin.settings.version.update') }}
        </a>
      </div>
    </template>
    <Tab
      icon="progress-wrench-outline"
      icon-size="1rem"
      :to="{ name: 'admin-settings-maintenance' }"
      :title="$t('admin.settings.maintenance.maintenance')"
    />
    <Tab icon="secret" icon-size="1rem" :to="{ name: 'admin-settings-secrets' }" :title="$t('secrets.secrets')" />
    <Tab icon="docker" :to="{ name: 'admin-settings-registries' }" :title="$t('registries.registries')" />
    <Tab
      icon="repo"
      icon-size="1rem"
      :to="{ name: 'admin-settings-repos' }"
      :title="$t('admin.settings.repos.repos')"
    />
    <Tab
      icon="user"
      icon-size="1rem"
      :to="{ name: 'admin-settings-users' }"
      :title="$t('admin.settings.users.users')"
    />
    <Tab icon="org" icon-size="1rem" :to="{ name: 'admin-settings-orgs' }" :title="$t('admin.settings.orgs.orgs')" />
    <Tab
      icon="agent"
      icon-size="1rem"
      :to="{ name: 'admin-settings-agents' }"
      :title="$t('admin.settings.agents.agents')"
    />
    <Tab
      icon="tasks"
      icon-size="1rem"
      :to="{ name: 'admin-settings-queue' }"
      :title="$t('admin.settings.queue.queue')"
    />

    <router-view />
  </Scaffold>
</template>

<script lang="ts" setup>
import { computed, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import Tab from '~/components/layout/scaffold/Tab.vue';
import useAuthentication from '~/compositions/useAuthentication';
import useNotifications from '~/compositions/useNotifications';
import { useVersion } from '~/compositions/useVersion';

const notifications = useNotifications();
const router = useRouter();
const i18n = useI18n();
const { user } = useAuthentication();
const version = useVersion();

// Get current version from window
const currentVersion = computed(() => (window as any).CROW_VERSION || 'unknown');

// Check if this is a dev version with commit hash
const isDevVersion = computed(() => {
  const version = currentVersion.value;
  return version && version.startsWith('dev-') && version.length > 4;
});

// Check if this is a semver version (starts with v and has dots)
const isSemverVersion = computed(() => {
  const version = currentVersion.value;
  return version && version.match(/^v?\d+\.\d+\.\d+/);
});

// Generate appropriate URL based on version type
const versionUrl = computed(() => {
  if (isDevVersion.value) {
    return 'https://codeberg.org/crowci/crow/commits/branch/main';
  } else if (isSemverVersion.value) {
    const cleanVersion = currentVersion.value.startsWith('v') ? currentVersion.value : `v${currentVersion.value}`;
    return `https://codeberg.org/crowci/crow/releases/tag/${cleanVersion}`;
  }
  return '';
});

onMounted(async () => {
  if (!user?.admin) {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      type: 'error',
      duration: 9000,
      text: i18n.t('admin.settings.notify_text_not_allowed'),
    });
    await router.replace({ name: 'home' });
  }
});
</script>

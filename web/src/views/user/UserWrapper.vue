<template>
  <Scaffold enable-tabs>
    <template #title>{{ $t('user.settings.settings') }}</template>
    <template #titleSuffix>
      <div class="ml-2 flex items-center space-x-2">
        <!-- Dev version badge -->
        <a
          v-if="isDevVersion"
          :href="versionUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="relative flex shrink-0 cursor-pointer items-center overflow-hidden whitespace-nowrap rounded-md border border-primary px-2 py-0.5 shadow-xs transition-all duration-150 hover:border-primary/80 hover:bg-primary/20 disabled:cursor-not-allowed disabled:opacity-50 bg-muted text-foreground"
        >
          {{ currentVersion }}
        </a>
        <!-- Semver release badge -->
        <a
          v-else-if="isSemverVersion"
          :href="versionUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="relative flex shrink-0 cursor-pointer items-center overflow-hidden whitespace-nowrap rounded-md border border-primary bg-muted px-2 py-0.5 text-xs shadow-xs transition-all duration-150 text-foreground hover:border-primary/80 hover:bg-primary/20"
        >
          {{ currentVersion }}
        </a>
        <!-- Other version badge (non-clickable) -->
        <span
          v-else-if="currentVersion && currentVersion !== 'unknown'"
          class="relative flex shrink-0 items-center overflow-hidden whitespace-nowrap rounded-md border border-border bg-muted px-2 py-0.5 text-xs shadow-xs text-foreground"
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
          class="relative flex shrink-0 cursor-pointer items-center overflow-hidden whitespace-nowrap rounded-md border border-border bg-card px-2 py-0.5 text-sm transition-all duration-150 text-foreground hover:bg-accent"
        >
          {{ version.latest?.replace(/^v/, '') }} {{ $t('admin.settings.version.update') }}
        </a>
      </div>
    </template>
    <template #headerActions
      ><Button as-child
        ><a :href="`${address}/logout`">{{ $t('logout') }}</a></Button
      ></template
    >
    <Tab icon="settings-outline" icon-size="1rem" :to="{ name: 'user' }" :title="$t('user.settings.general.general')" />
    <Tab icon="secret" icon-size="1rem" :to="{ name: 'user-secrets' }" :title="$t('secrets.secrets')" />
    <Tab icon="docker" icon-size="1rem" :to="{ name: 'user-registries' }" :title="$t('registries.registries')" />
    <Tab
      icon="console"
      icon-size="1rem"
      :to="{ name: 'user-cli-and-api' }"
      :title="$t('user.settings.cli_and_api.cli_and_api')"
    />
    <Tab
      v-if="useConfig().userRegisteredAgents"
      icon="agent"
      icon-size="1rem"
      :to="{ name: 'user-agents' }"
      :title="$t('admin.settings.agents.agents')"
    />
    <Tab
      icon="agent"
      icon-size="1rem"
      :to="{ name: 'user-global-agents' }"
      :title="`${$t('admin.settings.agents.agents')} (Global)`"
    />
    <router-view />
  </Scaffold>
</template>

<script lang="ts" setup>
import { computed } from 'vue';

import { Button } from '~/components/ui/button';
import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import Tab from '~/components/layout/scaffold/Tab.vue';
import useConfig from '~/compositions/useConfig';
import { useVersion } from '~/compositions/useVersion';

const address = `${window.location.protocol}//${window.location.host}${useConfig().rootPath}`; // port is included in location.host
const version = useVersion();

// Get current version from window
const currentVersion = computed(() => (window as any).CROW_VERSION || 'unknown');

// Check if this is a dev version with commit hash
const isDevVersion = computed(() => {
  const versionStr = currentVersion.value;
  return versionStr && versionStr.startsWith('dev-') && versionStr.length > 4;
});

// Check if this is a semver version (starts with v and has dots)
const isSemverVersion = computed(() => {
  const versionStr = currentVersion.value;
  return versionStr && versionStr.match(/^v?\d+\.\d+\.\d+/);
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
</script>

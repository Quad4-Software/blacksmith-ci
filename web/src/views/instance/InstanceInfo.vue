<template>
  <Settings :title="$t('info')">
    <div class="flex flex-col items-center gap-4">
      <!-- <component :is="isLightTheme ? CrowLogoDark : CrowLogo" class="w-32 h-32 fill-muted-foreground" /> -->
      <!-- FIXME: theme-switch is working but the dark svg is somewhat not rendered correctly -->
      <component :is="CrowLogo" v-if="!isLightTheme" class="h-32 w-32 fill-muted-foreground" />
      <i18n-t keypath="running_version" tag="p" class="text-center text-xl">
        <span class="font-bold">{{ version?.current }}</span>
      </i18n-t>
      <div v-if="version?.needsUpdate">
        <i18n-t keypath="update_crow" tag="span" :arguments="[version.latest?.replace(/^v/, '')]">
          <a
            v-if="!version.usesDev"
            :href="`https://codeberg.org/crowci/crow/releases/tag/${version.latest}`"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ version.latest?.replace(/^v/, '') }}
          </a>
          <span v-else class="font-bold"> {{ version.latest?.replace(/^v/, '') }} </span>
        </i18n-t>
      </div>
    </div>
  </Settings>
</template>

<script lang="ts" setup>
import { onMounted, ref } from 'vue';

import CrowLogo from '~/assets/logo.svg?component';
import Settings from '~/components/layout/Settings.vue';
import { useVersion } from '~/compositions/useVersion';

const version = useVersion();
const isLightTheme = ref(false);

const updateTheme = () => {
  isLightTheme.value = document.documentElement.getAttribute('data-theme') === 'light';
};

onMounted(() => {
  updateTheme();
  const observer = new MutationObserver(updateTheme);
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
});
</script>

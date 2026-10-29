<template>
  <Settings :title="$t('info')">
    <div class="flex flex-col items-center gap-4">
      <CrowLogo class="fill-muted-foreground h-32 w-32" />

      <i18n-t keypath="running_version" tag="p" class="text-center text-xl">
        <span class="font-bold">{{ version?.current }}</span>
      </i18n-t>

      <Error v-if="version?.needsUpdate">
        <i18n-t keypath="update_crow" tag="span">
          <a
            v-if="!version.usesDev"
            :href="`https://github.com/Quad4-Software/blacksmith-ci/releases/tag/${version.latest}`"
            target="_blank"
            rel="noopener noreferrer"
            class="underline"
          >
            {{ version.latest?.replace(/^v/, '') }}
          </a>
          <span v-else>
            {{ version.latest?.replace(/^v/, '') }}
          </span>
        </i18n-t>
      </Error>
    </div>
  </Settings>
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

import CrowLogo from '~/assets/logo.svg?component';
import Error from '~/components/atomic/Error.vue';
import Settings from '~/components/layout/Settings.vue';
import { useVersion } from '~/compositions/useVersion';
import { useCrowTitle } from '~/compositions/useCrowTitle';

const version = useVersion();

const { t } = useI18n();
useCrowTitle(computed(() => [t('info'), t('admin.settings.settings')]));
</script>

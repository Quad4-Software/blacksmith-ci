<template>
  <div class="app m-auto flex h-full w-full flex-col bg-wp-background-200 dark:bg-wp-background-100">
    <router-view v-if="blank" />
    <template v-else>
      <Navbar />
      <main class="relative flex h-full min-h-0">
        <div id="scroll-component" class="flex grow flex-col overflow-y-auto"><router-view /></div>
        <transition name="slide-right">
          <PipelineFeedSidebar class="absolute bottom-0 right-0 top-0 w-full max-w-80 border-l shadow-md xl:max-w-96" />
        </transition>
      </main>
    </template>
    <notifications position="bottom right" width="340px" />
  </div>
</template>

<script lang="ts" setup>
import { computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute } from 'vue-router';

import Navbar from '~/components/layout/header/Navbar.vue';
import PipelineFeedSidebar from '~/components/pipeline-feed/PipelineFeedSidebar.vue';
import useApiClient from '~/compositions/useApiClient';
import useNotifications from '~/compositions/useNotifications';

const route = useRoute();
const apiClient = useApiClient();
const { notify } = useNotifications();
const i18n = useI18n();

apiClient.setErrorHandler(
  async ({
    status,
    message = '',
    config,
  }: {
    status?: number;
    message?: string;
    config?: { method?: string; url?: string };
  }) => {
    if (status === 404) {
      await notify({ title: i18n.t('errors.not_found'), type: 'error' });
      return;
    }

    // Helper to parse error messages that might be JSON.
    const parseErrorMessage = (msg: string): string => {
      const jsonStart = msg.indexOf('{');
      if (jsonStart === -1) return msg;
      try {
        const parsed = JSON.parse(msg.slice(jsonStart));
        return parsed?.error || msg;
      } catch (e) {
        console.error('Failed to parse JSON from string:', e);
        return msg;
      }
    };

    let errorText = parseErrorMessage(message.trim());

    // Extract endpoint information.
    const endpointMarker = 'Endpoint:';
    let extractedEndpoint = '';
    const markerIndex = errorText.indexOf(endpointMarker);
    if (markerIndex !== -1) {
      // Everything after the first occurrence of "Endpoint:" is considered the endpoint.
      extractedEndpoint = errorText.slice(markerIndex + endpointMarker.length).trim();
      // In case the extracted endpoint accidentally contains another "Endpoint:" (unlikely),
      // keep only the text up to that point.
      const extraMarkerIndex = extractedEndpoint.indexOf(endpointMarker);
      if (extraMarkerIndex !== -1) {
        extractedEndpoint = extractedEndpoint.slice(0, extraMarkerIndex).trim();
      }
      // Remove the endpoint portion from the error text.
      errorText = errorText.slice(0, markerIndex).trim();
    } else if (config) {
      const method = config.method ? config.method.toUpperCase() : '';
      extractedEndpoint = method && config.url ? `${method} ${config.url}` : config.url || '';
    }
    if (!extractedEndpoint) {
      extractedEndpoint = 'unknown endpoint';
    }

    await notify({
      title: `❌️ API error (${extractedEndpoint})`,
      text: errorText || i18n.t('unknown_error'),
      type: 'error',
      duration: -1,
    });
  },
);

const blank = computed(() => route.meta.blank);

const { locale } = useI18n();
watch(
  locale,
  () => {
    document.documentElement.setAttribute('lang', locale.value);
  },
  { immediate: true },
);
</script>

<style scoped>
.app {
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.slide-right-enter-active,
.slide-right-leave-active {
  transition: all 0.3s ease;
}
.slide-right-enter-from,
.slide-right-leave-to {
  transform: translate(100%, 0);
}
</style>

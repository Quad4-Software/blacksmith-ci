<template>
  <div class="app m-auto flex h-full w-full flex-col bg-wp-background-200 dark:bg-wp-background-100">
    <router-view v-if="blank" />
    <template v-else>
      <Navbar />
      <main class="relative flex h-full min-h-0">
        <div id="scroll-component" class="flex grow flex-col overflow-y-auto h-full min-h-0">
          <router-view />
        </div>
        <transition name="slide-right">
          <PipelineFeedSidebar class="absolute bottom-0 right-0 top-0 w-full max-w-80 border-l shadow-md xl:max-w-96" />
        </transition>
      </main>
    </template>
    <notifications ref="notificationsRef" position="bottom right" width="340px" :dangerously-set-inner-html="true" />
  </div>
</template>

<script lang="ts" setup>
import { computed, watch, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute } from 'vue-router';

import Navbar from '~/components/layout/header/Navbar.vue';
import PipelineFeedSidebar from '~/components/pipeline-feed/PipelineFeedSidebar.vue';
import useApiClient from '~/compositions/useApiClient';
import useNotifications from '~/compositions/useNotifications';
import { useAuthStore } from '~/store/auth';

const route = useRoute();
const apiClient = useApiClient();
const { notify } = useNotifications();
const i18n = useI18n();
const authStore = useAuthStore();
const notificationsRef = ref();

// Function to clear all notifications
const clearAllNotifications = () => {
  if (notificationsRef.value && typeof notificationsRef.value.clearAll === 'function') {
    notificationsRef.value.clearAll();
  }
};

// Expose the function globally so it can be called from router
(window as any).clearAllNotifications = clearAllNotifications;

// Listen for clear notifications event
window.addEventListener('clearNotifications', clearAllNotifications);

// Helper to parse error messages that might be JSON
const parseErrorMessage = (msg: string): string => {
  const jsonStart = msg.indexOf('{');
  if (jsonStart === -1) return msg;
  try {
    const parsed = JSON.parse(msg.slice(jsonStart));
    return parsed?.error || msg;
  } catch {
    return msg;
  }
};

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
    const url = config?.url || '';

    if (status === 404) {
      await notify({ title: i18n.t('errors.not_found'), type: 'error' });
      return;
    }

    // Handle 401 errors - session invalidation is handled by the API client,
    // this just provides user feedback before redirect
    if (status === 401) {
      // Skip notification if already redirecting to login
      if (authStore.isRedirecting) {
        return;
      }

      // Skip if on login/logout/authorize pages
      const currentPath = window.location.pathname;
      if (currentPath.includes('/login') || currentPath.includes('/authorize') || currentPath.includes('/logout')) {
        return;
      }

      // Show brief notification before redirect
      await notify({
        title: i18n.t('notify_title_error'),
        text: i18n.t('stale_token_error'),
        type: 'error',
        duration: 3000,
      });
      return;
    }

    // Only handle other errors for API endpoints
    if (!url.startsWith('/api/')) {
      return;
    }

    let errorText = parseErrorMessage(message.trim());

    // Extract endpoint information
    const endpointMarker = 'Endpoint:';
    let extractedEndpoint = '';
    const markerIndex = errorText.indexOf(endpointMarker);
    if (markerIndex !== -1) {
      extractedEndpoint = errorText.slice(markerIndex + endpointMarker.length).trim();
      const extraMarkerIndex = extractedEndpoint.indexOf(endpointMarker);
      if (extraMarkerIndex !== -1) {
        extractedEndpoint = extractedEndpoint.slice(0, extraMarkerIndex).trim();
      }
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

#scroll-component {
  scrollbar-gutter: stable;
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

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
    <notifications ref="notificationsRef" position="bottom right" width="340px" dangerously-set-inner-html="true" />
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
import useAuthentication from '~/compositions/useAuthentication';

const route = useRoute();
const apiClient = useApiClient();
const { notify } = useNotifications();
const i18n = useI18n();
const { isAuthenticated } = useAuthentication();
const notificationsRef = ref();

// Debouncing for stale token notifications to prevent multiple notifications
let staleTokenNotificationShown = false;
let staleTokenNotificationTimeout: number | null = null;
let lastLoginTime = 0;

// Function to clear all notifications
const clearAllNotifications = () => {
  if (notificationsRef.value && typeof notificationsRef.value.clearAll === 'function') {
    notificationsRef.value.clearAll();
  }
  // Reset stale token notification state immediately
  staleTokenNotificationShown = false;
  if (staleTokenNotificationTimeout) {
    clearTimeout(staleTokenNotificationTimeout);
    staleTokenNotificationTimeout = null;
  }
  // Mark the current time as login time to prevent immediate stale token notifications
  lastLoginTime = Date.now();
};

// Expose the function globally so it can be called from router
(window as any).clearAllNotifications = clearAllNotifications;

// Listen for clear notifications event
window.addEventListener('clearNotifications', clearAllNotifications);

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

    // Handle stale token errors (401 Unauthorized) - handle all 401 errors, not just API endpoints
    if (status === 401) {
      // Helper to parse error messages that might be JSON.
      const parseErrorMessage = (msg: string): string => {
        // Look for JSON content in the message, which might be after status text like "Unauthorized: "
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

      const errorText = parseErrorMessage(message.trim());

      // Check if this is a stale token error by looking for specific error messages
      const isStaleTokenError =
        errorText.includes('authorization has expired') ||
        errorText.includes('could not be refreshed') ||
        errorText.includes('token was already used') ||
        errorText.includes('unauthorized_client') ||
        errorText.includes('token expired and refresh token is not set') ||
        message.includes('TOKEN_REFRESH_FAILED');

      if (isStaleTokenError) {
        // Only show stale token error to the current authenticated user
        // This ensures that in a multi-user shared instance, only the affected user sees the message
        if (isAuthenticated) {
          // Don't show the error if we're on the login page or if we just logged in
          const currentPath = window.location.pathname;
          if (currentPath.includes('/login') || currentPath.includes('/authorize')) {
            return;
          }

          // Don't show stale token notifications immediately after login (within 5 seconds)
          const timeSinceLogin = Date.now() - lastLoginTime;
          if (timeSinceLogin < 5000) {
            return;
          }

          // Prevent multiple stale token notifications by debouncing
          if (staleTokenNotificationShown) {
            return;
          }

          staleTokenNotificationShown = true;

          // Clear any existing timeout
          if (staleTokenNotificationTimeout) {
            clearTimeout(staleTokenNotificationTimeout);
          }

          // Reset the flag after 10 seconds to allow future notifications
          staleTokenNotificationTimeout = window.setTimeout(() => {
            staleTokenNotificationShown = false;
            staleTokenNotificationTimeout = null;
          }, 10000);

          // For web users, the error is specific to their session
          // For API users, the error would be specific to their token
          // Since we're in the web UI, this is the current user's session
          await notify({
            title: i18n.t('notify_title_error'),
            text: `${i18n.t('stale_token_error')} <a href='/logout' class='text-wp-custom-highlight-100 underline'>Log in again</a>`,
            type: 'error',
            duration: 10000, // Show for 10 seconds
          });
        }
        return;
      }

      // For other 401 errors (not stale token), show a generic authentication error
      await notify({
        title: i18n.t('notify_title_error'),
        text: 'Authentication required. Please log in.',
        type: 'error',
        duration: 8000,
      });
      return;
    }

    // Only handle other errors for API endpoints
    if (!url.startsWith('/api/')) {
      return;
    }

    // Helper to parse error messages that might be JSON.
    const parseErrorMessage = (msg: string): string => {
      // Look for JSON content in the message, which might be after status text like "Unauthorized: "
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

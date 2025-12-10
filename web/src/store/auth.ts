import { defineStore } from 'pinia';
import { computed, ref } from 'vue';

import useConfig from '~/compositions/useConfig';
import useUserConfig from '~/compositions/useUserConfig';
import type { User } from '~/lib/api/types';

export const useAuthStore = defineStore('auth', () => {
  const config = useConfig();
  const userConfig = useUserConfig();

  // Reactive session state
  const isSessionExpired = ref(false);
  const isRedirecting = ref(false);

  // Resource registries for cleanup
  const eventSources = ref<Set<EventSource>>(new Set());
  const abortControllers = ref<Set<AbortController>>(new Set());

  // Computed auth state from server-injected config
  const user = computed<User | null>(() => config.user);
  const isAuthenticated = computed(() => !!user.value && !isSessionExpired.value);

  /**
   * Invalidates the current session and redirects to login.
   * - Aborts all in-flight API requests
   * - Closes all EventSource streams
   * - Redirects to login with return URL preserved
   */
  function invalidateSession(returnUrl?: string) {
    // Prevent multiple invalidations
    if (isSessionExpired.value) {
      return;
    }

    isSessionExpired.value = true;
    isRedirecting.value = true;

    // Cleanup all resources
    cleanupEventSources();
    abortAllRequests();

    // Get current path for return URL
    const currentPath =
      returnUrl !== undefined && returnUrl !== '' ? returnUrl : window.location.pathname + window.location.search;

    // Don't preserve URL if already on login/logout/authorize pages
    const preserveUrl =
      !currentPath.includes('/login') && !currentPath.includes('/logout') && !currentPath.includes('/authorize');

    if (preserveUrl) {
      // Store return URL using existing userConfig pattern
      userConfig.setUserConfig('redirectUrl', currentPath);
    }

    // Redirect to login page
    // Use window.location for a full page reload to get fresh server state
    const loginUrl = `${config.rootPath}/login`;
    window.location.href = loginUrl;
  }

  /**
   * Resets session state after successful login.
   * Called by router when redirectUrl is processed.
   */
  function resetSessionState() {
    isSessionExpired.value = false;
    isRedirecting.value = false;
  }

  // EventSource management
  function registerEventSource(es: EventSource) {
    eventSources.value.add(es);
  }

  function unregisterEventSource(es: EventSource) {
    eventSources.value.delete(es);
  }

  function cleanupEventSources() {
    eventSources.value.forEach((es) => {
      try {
        es.close();
      } catch {
        // Ignore errors when closing
      }
    });
    eventSources.value.clear();
  }

  // AbortController management
  function registerAbortController(ac: AbortController) {
    abortControllers.value.add(ac);
  }

  function removeAbortController(ac: AbortController) {
    abortControllers.value.delete(ac);
  }

  function abortAllRequests() {
    abortControllers.value.forEach((ac) => {
      try {
        ac.abort('Session expired');
      } catch {
        // Ignore errors when aborting
      }
    });
    abortControllers.value.clear();
  }

  return {
    // State
    user,
    isAuthenticated,
    isSessionExpired: computed(() => isSessionExpired.value),
    isRedirecting: computed(() => isRedirecting.value),

    // Actions
    invalidateSession,
    resetSessionState,
    registerEventSource,
    unregisterEventSource,
    registerAbortController,
    removeAbortController,
  };
});

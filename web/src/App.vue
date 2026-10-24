<template>
  <!-- Loading screen while i18n initializes -->
  <div v-if="!i18nReady" class="flex h-full w-full items-center justify-center bg-background">
    <div class="flex flex-col items-center gap-4">
      <component :is="logoComponent" class="h-12 w-12 animate-pulse" />
      <div class="h-1 w-32 overflow-hidden rounded-full bg-muted">
        <div class="h-full w-1/2 animate-loading-bar bg-primary" />
      </div>
    </div>
  </div>

  <div v-else class="app flex h-full w-full flex-col bg-background text-foreground">
    <router-view v-if="blank" v-slot="{ Component }" :key="$route.matched[0]?.path">
      <Transition name="fade" mode="out-in">
        <component :is="Component" />
      </Transition>
    </router-view>
    <div v-else class="flex h-full w-full">
      <SidebarProvider class="flex flex-1 overflow-hidden w-full">
        <AppSidebar class="hidden md:flex" />
        <SidebarInset class="flex flex-col overflow-y-auto overflow-x-hidden w-full">
          <!-- Navbar integrated with sidebar -->
          <header
            class="sticky top-0 z-50 flex h-12 shrink-0 items-center gap-2 bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/60 w-full"
          >
            <div class="flex items-center gap-2 w-full border-b">
              <!-- Left: Sidebar trigger (only show on desktop where sidebar exists) -->
              <div class="hidden md:flex h-12 w-12 shrink-0 items-center justify-center border-r">
                <SidebarTrigger>
                  <template #tooltip>
                    <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                    <div class="flex items-center gap-1">
                      <span>Toggle Sidebar</span>
                      <div class="flex items-center gap-0.5 ml-2">
                        <Kbd v-if="isMac">⌘</Kbd>
                        <Kbd v-else>Ctrl</Kbd>
                        <Kbd>B</Kbd>
                      </div>
                    </div>
                    <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                  </template>
                </SidebarTrigger>
              </div>

              <!-- Mobile: Sidebar trigger on left -->
              <div class="flex md:hidden h-12 w-12 shrink-0 items-center justify-center">
                <SidebarTrigger />
              </div>

              <!-- Center: Crow Logo (both mobile and desktop) -->
              <div class="flex flex-1 items-center justify-center">
                <router-link
                  :to="{ name: 'home' }"
                  class="flex h-8 w-8 items-center justify-center hover:opacity-80 transition-opacity"
                >
                  <img :src="logoSrc" alt="Crow CI" class="h-7 w-7" />
                </router-link>
              </div>

              <!-- Right section: Actions and user menu -->
              <div class="flex items-center gap-1 px-2 overflow-visible pr">
                <!-- Active pipelines -->
                <ActivePipelines v-if="user" />

                <!-- Forge auth status warning -->
                <ForgeAuthStatus v-if="user" />

                <!-- Failing watched pipelines -->
                <WatchBell v-if="user" />

                <!-- User menu or login button -->
                <DropdownMenu v-if="user">
                  <DropdownMenuTrigger as-child>
                    <Button variant="ghost" size="icon" class="relative rounded-full">
                      <Avatar class="h-8 w-8">
                        <AvatarImage v-if="user.avatar_url" :src="user.avatar_url" :alt="user.login" />
                        <AvatarFallback>{{ user.login?.charAt(0).toUpperCase() }}</AvatarFallback>
                      </Avatar>
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" class="w-56">
                    <DropdownMenuLabel>
                      <div class="flex flex-col space-y-1">
                        <p class="text-sm font-medium leading-none">{{ user.login }}</p>
                        <p class="text-xs leading-none text-muted-foreground">{{ user.email }}</p>
                      </div>
                    </DropdownMenuLabel>
                    <template v-if="switchAccounts.length > 1">
                      <DropdownMenuSeparator />
                      <DropdownMenuLabel class="text-xs font-normal text-muted-foreground">
                        {{ $t('account_switch.title') }}
                      </DropdownMenuLabel>
                      <DropdownMenuItem
                        v-for="account in switchAccounts"
                        :key="account.id"
                        as-child
                        :disabled="account.active"
                      >
                        <a
                          :href="account.active ? undefined : `${address}/switch-account?id=${account.id}`"
                          class="flex w-full cursor-pointer items-center"
                        >
                          <Avatar class="mr-2 h-5 w-5">
                            <AvatarImage v-if="account.avatar_url" :src="account.avatar_url" :alt="account.login" />
                            <AvatarFallback class="text-xs">{{
                              account.login?.charAt(0).toUpperCase()
                            }}</AvatarFallback>
                          </Avatar>
                          <span class="flex-1 truncate">{{ account.login }}</span>
                          <Icon v-if="account.active" name="ph:check" class="ml-2 size-4" />
                        </a>
                      </DropdownMenuItem>
                    </template>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem as-child>
                      <router-link
                        :to="{ name: 'login', query: { add: 'true' } }"
                        class="flex w-full cursor-pointer items-center"
                      >
                        <Icon name="ph:user-plus" class="mr-2 size-4" />
                        {{ $t('account_switch.add') }}
                      </router-link>
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <div class="flex items-center gap-1 px-2 py-1.5">
                      <Button
                        variant="ghost"
                        size="icon"
                        class="h-8 w-8"
                        :class="{ 'bg-accent': storeTheme === 'light' }"
                        :title="$t('user.settings.general.theme.light')"
                        @click.prevent.stop="storeTheme = 'light'"
                      >
                        <Icon name="ph:sun" class="size-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        class="h-8 w-8"
                        :class="{ 'bg-accent': storeTheme === 'dark' }"
                        :title="$t('user.settings.general.theme.dark')"
                        @click.prevent.stop="storeTheme = 'dark'"
                      >
                        <Icon name="ph:moon" class="size-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        class="h-8 w-8"
                        :class="{ 'bg-accent': storeTheme === 'auto' }"
                        :title="$t('user.settings.general.theme.auto')"
                        @click.prevent.stop="storeTheme = 'auto'"
                      >
                        <Icon name="ph:circle-half" class="size-4" />
                      </Button>
                    </div>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem as-child>
                      <router-link :to="{ name: 'user' }" class="flex w-full cursor-pointer items-center">
                        <Icon name="settings2" class="mr-2 size-4" />
                        {{ $t('user.settings.sidebar_title') }}
                      </router-link>
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem as-child>
                      <a :href="`${address}/logout`" class="flex w-full cursor-pointer items-center">
                        <Icon name="logout" class="mr-2 size-4" />
                        {{ switchAccounts.length > 1 ? $t('account_switch.logout_current') : $t('logout') }}
                      </a>
                    </DropdownMenuItem>
                    <DropdownMenuItem v-if="switchAccounts.length > 1" as-child>
                      <a :href="`${address}/logout?all=true`" class="flex w-full cursor-pointer items-center">
                        <Icon name="logout" class="mr-2 size-4" />
                        {{ $t('account_switch.logout_all') }}
                      </a>
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>

                <Button v-else as-child variant="default" size="sm">
                  <router-link :to="`/login?url=${route.fullPath}`">
                    {{ $t('login') }}
                  </router-link>
                </Button>
              </div>
            </div>
          </header>

          <!-- Main content area with PipelineFeedSidebar -->
          <div class="relative flex flex-1 min-h-0 overflow-hidden">
            <router-view v-slot="{ Component }" :key="$route.matched[0]?.path">
              <Transition name="fade" mode="out-in">
                <component :is="Component" />
              </Transition>
            </router-view>
            <PipelineFeedSidebar />
          </div>
        </SidebarInset>
      </SidebarProvider>
    </div>

    <Toaster
      position="bottom-right"
      :expand="false"
      theme="system"
      :close-button="true"
      close-button-position="top-right"
    />

    <LoadingOverlay />

    <!-- Global Command Palette and Dialogs -->
    <GlobalCommandPalette />
    <KeyboardShortcutsDialog />
    <AddSecretDialog />
    <AddRegistryDialog />
    <AddAgentDialog />
    <AddRepositoryDialog />
    <AddCronDialog />
  </div>
</template>

<script lang="ts" setup>
import 'vue-sonner/style.css';
import CrowLogoPng from '~/assets/logo-dark.png';
import CrowLogoSvg from '~/assets/logo-dark.svg?component';
import CrowLogoDarkPng from '~/assets/logo.png';
import CrowLogoDarkSvg from '~/assets/logo.svg?component';
import Icon from '~/components/atomic/Icon.vue';
import AddAgentDialog from '~/components/command-palette/AddAgentDialog.vue';
import AddCronDialog from '~/components/command-palette/AddCronDialog.vue';
import AddRegistryDialog from '~/components/command-palette/AddRegistryDialog.vue';
import AddRepositoryDialog from '~/components/command-palette/AddRepositoryDialog.vue';
import AddSecretDialog from '~/components/command-palette/AddSecretDialog.vue';
import GlobalCommandPalette from '~/components/command-palette/GlobalCommandPalette.vue';
import KeyboardShortcutsDialog from '~/components/keyboard-shortcuts/KeyboardShortcutsDialog.vue';
import AppSidebar from '~/components/layout/AppSidebar.vue';
import ActivePipelines from '~/components/layout/header/ActivePipelines.vue';
import ForgeAuthStatus from '~/components/layout/header/ForgeAuthStatus.vue';
import WatchBell from '~/components/layout/header/WatchBell.vue';
import LoadingOverlay from '~/components/layout/LoadingOverlay.vue';
import PipelineFeedSidebar from '~/components/pipeline-feed/PipelineFeedSidebar.vue';
import Avatar from '~/components/ui/avatar/Avatar.vue';
import AvatarFallback from '~/components/ui/avatar/AvatarFallback.vue';
import AvatarImage from '~/components/ui/avatar/AvatarImage.vue';
import { Button } from '~/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import Kbd from '~/components/ui/kbd/Kbd.vue';
import { SidebarInset, SidebarProvider, SidebarTrigger } from '~/components/ui/sidebar';
import { Toaster } from '~/components/ui/sonner';
import useApiClient from '~/compositions/useApiClient';
import useAuthentication from '~/compositions/useAuthentication';
import useConfig from '~/compositions/useConfig';
import { i18nReady } from '~/compositions/useI18n';
import { is404Suppressed } from '~/compositions/useNotificationControl';
import useNotifications from '~/compositions/useNotifications';
import { useTheme } from '~/compositions/useTheme';
import { computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute } from 'vue-router';
import { toast } from 'vue-sonner';

const BRACKETED_PREFIX_REGEX = /^\[[\w\s]+\]\s*/;

const route = useRoute();
const apiClient = useApiClient();
const { notify } = useNotifications();
const i18n = useI18n();
const { isAuthenticated, user } = useAuthentication();
const { storeTheme, theme } = useTheme();
const config = useConfig();
const address = `${window.location.protocol}//${window.location.host}${config.rootPath}`;

// Accounts the browser has authenticated with, for quick-switching without a
// full logout/login. Injected by the server into the page config.
const switchAccounts = computed(() => config.accounts);

const isDark = computed(() => theme.value === 'dark');

// Select the appropriate logo based on theme
const logoSrc = computed(() => (isDark.value ? CrowLogoPng : CrowLogoDarkPng));
const logoComponent = computed(() => (isDark.value ? CrowLogoDarkSvg : CrowLogoSvg));

// Detect OS for keyboard shortcut display
const isMac = computed(() => navigator.userAgent.includes('Mac'));

// Stale token notification debouncing state
let staleTokenNotificationShown = false;
let staleTokenNotificationTimeout: number | null = null;
let lastLoginTime = 0;

// Function to clear all notifications
const clearAllNotifications = () => {
  // Dismiss all toasts
  toast.dismiss();
  // Reset stale token notification state immediately
  staleTokenNotificationShown = false;
  if (staleTokenNotificationTimeout) {
    clearTimeout(staleTokenNotificationTimeout);
    staleTokenNotificationTimeout = null;
  }
  // Mark the current time as login time to prevent immediate stale token notifications
  lastLoginTime = Date.now();
};

// Listen for clear notifications event (dispatched from router)
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
      // Skip 404 notification if temporarily suppressed (e.g., after deleting a resource)
      if (is404Suppressed()) {
        return;
      }
      notify({ title: i18n.t('errors.not_found'), type: 'error' });
      return;
    }

    // Handle 401 errors - session invalidation is handled by the API client,
    // this just provides user feedback before redirect
    if (status === 401) {
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

          // Defer the notification to ensure Vue's render cycle has completed.
          // When all API calls fail with 401 during initial mount, vue-sonner's
          // Toaster component may not have finished mounting yet. Delaying ensures
          // the toast state change triggers a proper re-render.
          setTimeout(() => {
            notify({
              title: i18n.t('notify_title_error'),
              text: `${i18n.t('stale_token_error')} <a href='/logout' class='text-primary underline'>Log in again</a>`,
              type: 'error',
              duration: 10000, // Show for 10 seconds
            });
          }, 500);
        }
        return;
      }

      // For other 401 errors (not stale token):
      // Don't show toast for anonymous users - Login button is already visible
      if (!isAuthenticated) {
        return;
      }
      // Show a generic authentication error for authenticated users
      notify({
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

    // Strip technical prefixes like [compiler], [linter], etc. for cleaner user-facing messages
    errorText = errorText.replace(BRACKETED_PREFIX_REGEX, '');

    // Strip any "Endpoint:" suffix from the error message
    const endpointMarker = 'Endpoint:';
    const markerIndex = errorText.indexOf(endpointMarker);
    if (markerIndex !== -1) {
      errorText = errorText.slice(0, markerIndex).trim();
    }

    notify({
      title: i18n.t('notify_title_error'),
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

.fade-enter-active {
  transition:
    opacity 0.2s ease,
    transform 0.2s ease;
}
.fade-leave-active {
  transition:
    opacity 0.15s ease,
    transform 0.15s ease;
}
.fade-enter-from {
  opacity: 0;
  transform: translateY(4px);
}
.fade-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

@keyframes loading-bar {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(200%);
  }
}

.animate-loading-bar {
  animation: loading-bar 1s ease-in-out infinite;
}
</style>

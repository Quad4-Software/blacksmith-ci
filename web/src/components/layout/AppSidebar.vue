<template>
  <Sidebar side="left" collapsible="icon">
    <SidebarContent>
      <!-- Command Palette -->
      <SidebarGroup v-if="user" class="h-[2.45rem] pt-[6.0px] flex items-center">
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton :tooltip="$t('command_palette.title')" @click="openCommandPalette">
              <Icon name="console" class="size-4" />
              <span v-if="!isCollapsed">{{ $t('command_palette.title') }}</span>
              <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
              <div v-if="!isMobile && !isCollapsed" class="ml-auto flex items-center gap-0.5">
                <Kbd v-if="isMac">⌘</Kbd>
                <Kbd v-else>Ctrl</Kbd>
                <Kbd>K</Kbd>
              </div>
              <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarGroup>

      <SidebarSeparator v-if="user" />

      <!-- CI/CD group -->
      <Collapsible v-if="user" :default-open="true" class="group/collapsible">
        <SidebarGroup>
          <SidebarGroupLabel as-child>
            <CollapsibleTrigger>
              {{ $t('nav_ci_cd') }}
              <Icon
                name="chevron-down"
                class="ml-auto transition-transform group-data-[state=open]/collapsible:rotate-180"
              />
            </CollapsibleTrigger>
          </SidebarGroupLabel>
          <CollapsibleContent>
            <SidebarGroupContent>
              <SidebarMenu>
                <SidebarMenuItem>
                  <SidebarMenuButton as-child :tooltip="$t('repositories.title')" :is-active="isActive('repos')">
                    <router-link :to="{ name: 'repos' }">
                      <Icon name="repo" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('repositories.title') }}</span>
                      <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                      <span v-if="!isMobile && !isCollapsed" class="ml-auto text-xs text-muted-foreground">Alt+R</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('admin.settings.agents.agents')"
                    :is-active="isActive('agents')"
                  >
                    <router-link :to="{ name: 'agents' }">
                      <Icon name="agent" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('admin.settings.agents.agents') }}</span>
                      <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                      <span v-if="!isMobile && !isCollapsed" class="ml-auto text-xs text-muted-foreground">Alt+A</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('admin.settings.autoscalers.autoscalers')"
                    :is-active="isActive('autoscalers')"
                  >
                    <router-link :to="{ name: 'autoscalers' }">
                      <Icon name="autoscaler" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('admin.settings.autoscalers.autoscalers') }}</span>
                      <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                      <span v-if="!isMobile && !isCollapsed" class="ml-auto text-xs text-muted-foreground">Alt+U</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </CollapsibleContent>
        </SidebarGroup>
      </Collapsible>

      <!-- Credentials group -->
      <Collapsible v-if="user" :default-open="true" class="group/collapsible">
        <SidebarGroup>
          <SidebarGroupLabel as-child>
            <CollapsibleTrigger>
              {{ $t('nav_credentials') }}
              <Icon
                name="chevron-down"
                class="ml-auto transition-transform group-data-[state=open]/collapsible:rotate-180"
              />
            </CollapsibleTrigger>
          </SidebarGroupLabel>
          <CollapsibleContent>
            <SidebarGroupContent>
              <SidebarMenu>
                <SidebarMenuItem>
                  <SidebarMenuButton as-child :tooltip="$t('secrets.secrets')" :is-active="isActive('user-secrets')">
                    <router-link :to="{ name: 'user-secrets' }">
                      <Icon name="secret" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('secrets.secrets') }}</span>
                      <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                      <span v-if="!isMobile && !isCollapsed" class="ml-auto text-xs text-muted-foreground">Alt+S</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('registries.registries')"
                    :is-active="isActive('user-registries')"
                  >
                    <router-link :to="{ name: 'user-registries' }">
                      <Icon name="docker" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('registries.registries') }}</span>
                      <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                      <span v-if="!isMobile && !isCollapsed" class="ml-auto text-xs text-muted-foreground"
                        >Alt+Shift+R</span
                      >
                      <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('access_tokens.title')"
                    :is-active="isActive('user-access-tokens')"
                  >
                    <router-link :to="{ name: 'user-access-tokens' }">
                      <Icon name="secret" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('access_tokens.title') }}</span>
                      <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                      <span v-if="!isMobile && !isCollapsed" class="ml-auto text-xs text-muted-foreground">Alt+T</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </CollapsibleContent>
        </SidebarGroup>
      </Collapsible>

      <!-- General group -->
      <Collapsible v-if="user" :default-open="true" class="group/collapsible">
        <SidebarGroup>
          <SidebarGroupLabel as-child>
            <CollapsibleTrigger>
              {{ $t('nav_general') }}
              <Icon
                name="chevron-down"
                class="ml-auto transition-transform group-data-[state=open]/collapsible:rotate-180"
              />
            </CollapsibleTrigger>
          </SidebarGroupLabel>
          <CollapsibleContent>
            <SidebarGroupContent>
              <SidebarMenu>
                <SidebarMenuItem>
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('repo.settings.crons.crons')"
                    :is-active="isActive('user-crons')"
                  >
                    <router-link :to="{ name: 'user-crons' }">
                      <Icon name="ph:clock" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('repo.settings.crons.crons') }}</span>
                      <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                      <span v-if="!isMobile && !isCollapsed" class="ml-auto text-xs text-muted-foreground">Alt+C</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('user.watching.title')"
                    :is-active="isActive('user-watching')"
                  >
                    <router-link :to="{ name: 'user-watching' }">
                      <Icon name="ph:bell" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('user.watching.title') }}</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton as-child :tooltip="$t('metrics.title')" :is-active="isActive('user-metrics')">
                    <router-link :to="{ name: 'user-metrics' }">
                      <Icon name="graph" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('metrics.title') }}</span>
                      <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                      <span v-if="!isMobile && !isCollapsed" class="ml-auto text-xs text-muted-foreground">Alt+M</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('integrations.title')"
                    :is-active="isActive('user-integrations')"
                  >
                    <router-link :to="{ name: 'user-integrations' }">
                      <Icon name="ph:plug" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('integrations.title') }}</span>
                      <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                      <span v-if="!isMobile && !isCollapsed" class="ml-auto text-xs text-muted-foreground">Alt+I</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </CollapsibleContent>
        </SidebarGroup>
      </Collapsible>

      <SidebarSeparator v-if="canAny(adminAreaPermissions)" />

      <Collapsible v-if="canAny(adminAreaPermissions)" :default-open="true" class="group/collapsible">
        <SidebarGroup>
          <SidebarGroupLabel as-child>
            <CollapsibleTrigger>
              {{ $t('admin.settings.sidebar_title') }}
              <Icon
                name="chevron-down"
                class="ml-auto transition-transform group-data-[state=open]/collapsible:rotate-180"
              />
            </CollapsibleTrigger>
          </SidebarGroupLabel>
          <CollapsibleContent>
            <SidebarGroupContent>
              <SidebarMenu>
                <SidebarMenuItem v-if="canSee('admin-settings-maintenance')">
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('admin.settings.maintenance.maintenance')"
                    :is-active="isActive('admin-settings-maintenance')"
                  >
                    <router-link :to="{ name: 'admin-settings-maintenance' }">
                      <Icon name="progress-wrench-outline" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('admin.settings.maintenance.maintenance') }}</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="canSee('admin-settings-secrets')">
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('secrets.secrets')"
                    :is-active="isActive('admin-settings-secrets')"
                  >
                    <router-link :to="{ name: 'admin-settings-secrets' }">
                      <Icon name="secret" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('secrets.secrets') }}</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="canSee('admin-settings-registries')">
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('registries.registries')"
                    :is-active="isActive('admin-settings-registries')"
                  >
                    <router-link :to="{ name: 'admin-settings-registries' }">
                      <Icon name="docker" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('registries.registries') }}</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="canSee('admin-settings-repos')">
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('admin.settings.repos.repos')"
                    :is-active="isActive('admin-settings-repos')"
                  >
                    <router-link :to="{ name: 'admin-settings-repos' }">
                      <Icon name="repo" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('admin.settings.repos.repos') }}</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="canSee('admin-settings-entities')">
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('admin.settings.entities.title')"
                    :is-active="isActive('admin-settings-entities')"
                  >
                    <router-link :to="{ name: 'admin-settings-entities' }">
                      <Icon name="user" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('admin.settings.entities.title') }}</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="canSee('admin-settings-queue')">
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('admin.settings.queue.queue')"
                    :is-active="isActive('admin-settings-queue')"
                  >
                    <router-link :to="{ name: 'admin-settings-queue' }">
                      <Icon name="tasks" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('admin.settings.queue.queue') }}</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="canSee('admin-settings-caches')">
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('caches.admin.title')"
                    :is-active="isActive('admin-settings-caches')"
                  >
                    <router-link :to="{ name: 'admin-settings-caches' }">
                      <Icon name="cache" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('caches.admin.title') }}</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="canSee('admin-settings-notifications')">
                  <SidebarMenuButton
                    as-child
                    :tooltip="$t('admin.settings.notifications.notifications')"
                    :is-active="isActive('admin-settings-notifications')"
                  >
                    <router-link :to="{ name: 'admin-settings-notifications' }">
                      <Icon name="email-outline" class="size-4" />
                      <span v-if="!isCollapsed">{{ $t('admin.settings.notifications.notifications') }}</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </CollapsibleContent>
        </SidebarGroup>
      </Collapsible>
    </SidebarContent>

    <SidebarFooter class="border-t border-sidebar-border">
      <SidebarMenu>
        <SidebarMenuItem>
          <SidebarMenuButton as-child :tooltip="$t('docs')">
            <a href="https://docs.crowci.com/" target="_blank">
              <Icon name="ph:book-open" class="size-4" />
              <span v-if="!isCollapsed">{{ $t('docs') }}</span>
            </a>
          </SidebarMenuButton>
        </SidebarMenuItem>

        <SidebarMenuItem v-if="enableSwagger">
          <SidebarMenuButton as-child :tooltip="$t('api')">
            <a :href="apiUrl" target="_blank">
              <Icon name="ph:code" class="size-4" />
              <span v-if="!isCollapsed">{{ $t('api') }}</span>
            </a>
          </SidebarMenuButton>
        </SidebarMenuItem>

        <SidebarMenuItem v-if="user">
          <SidebarMenuButton as-child :tooltip="$t('logout')">
            <a :href="`${address}/logout`">
              <Icon name="logout" class="size-4" />
              <span v-if="!isCollapsed">{{ $t('logout') }}</span>
            </a>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>

      <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
      <a
        href="https://github.com/Quad4-Software/blacksmith-ci"
        target="_blank"
        class="block px-2 py-1.5 text-xs text-muted-foreground space-y-0.5 hover:underline group-data-[collapsible=icon]:hidden"
      >
        <div>Blacksmith {{ versionInfo?.current || config.version }}</div>
        <div v-if="config.buildDate" class="text-[0.65rem] opacity-75">
          {{ formattedBuildDate }}
        </div>
      </a>
      <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
    </SidebarFooter>
  </Sidebar>
</template>

<script setup lang="ts">
import Icon from '~/components/atomic/Icon.vue';
import { computed, watch } from 'vue';
import { useRoute } from 'vue-router';

import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '~/components/ui/collapsible';
import Kbd from '~/components/ui/kbd/Kbd.vue';
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarSeparator,
  useSidebar,
} from '~/components/ui/sidebar';
import useAuthentication from '~/compositions/useAuthentication';
import { useCommandPalette } from '~/compositions/useCommandPalette';
import useConfig from '~/compositions/useConfig';
import usePermissions, { adminAreaPermissions } from '~/compositions/usePermissions';
import { useVersion } from '~/compositions/useVersion';

const config = useConfig();
const { user } = useAuthentication();
const { canAny, canSee } = usePermissions();
const versionInfo = useVersion();
const { toggle: toggleCommandPalette } = useCommandPalette();
const { isMobile, state, setOpenMobile } = useSidebar();
const isCollapsed = computed(() => state.value === 'collapsed');
const route = useRoute();

// Check if the current route matches or starts with the given route name
function isActive(routeName: string): boolean {
  const currentName = route.name?.toString() || '';
  // Exact match or the current route starts with the given name (for nested routes)
  return currentName === routeName || currentName.startsWith(`${routeName}-`);
}

// Auto-close mobile sidebar on navigation
watch(
  () => route.fullPath,
  () => {
    if (isMobile.value) {
      setOpenMobile(false);
    }
  },
);

// Close mobile sidebar and open command palette
function openCommandPalette() {
  if (isMobile.value) {
    setOpenMobile(false);
  }
  toggleCommandPalette();
}
const apiUrl = `${config.rootPath ?? ''}/api/v1/docs`;
const address = `${window.location.protocol}//${window.location.host}${config.rootPath}`;
const { enableSwagger } = config;

// Detect OS for keyboard shortcut display
const isMac = computed(() => navigator.userAgent.includes('Mac'));

// Format build date if available
const formattedBuildDate = computed(() => {
  if (!config.buildDate) return '';
  try {
    const date = new Date(config.buildDate);
    return date.toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  } catch {
    return config.buildDate;
  }
});
</script>

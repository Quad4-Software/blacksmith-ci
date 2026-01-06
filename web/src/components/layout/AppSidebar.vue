<template>
  <Sidebar side="left" collapsible="offcanvas">
    <SidebarContent class="overflow-y-auto overflow-x-hidden">
      <!-- Command Palette -->
      <SidebarGroup v-if="user" class="h-[2.45rem] pt-[6.0px] flex items-center">
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton :tooltip="$t('command_palette.title')" @click="openCommandPalette">
              <Icon name="console" class="size-4" />
              <span>{{ $t('command_palette.title') }}</span>
              <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
              <div v-if="!isMobile" class="ml-auto flex items-center gap-0.5">
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

      <Collapsible :default-open="true" class="group/collapsible">
        <SidebarGroup>
          <SidebarGroupLabel as-child>
            <CollapsibleTrigger>
              {{ $t('navigation') }}
              <Icon
                name="chevron-down"
                class="ml-auto transition-transform group-data-[state=open]/collapsible:rotate-180"
              />
            </CollapsibleTrigger>
          </SidebarGroupLabel>
          <CollapsibleContent>
            <SidebarGroupContent>
              <SidebarMenu>
                <SidebarMenuItem v-if="user">
                  <SidebarMenuButton as-child :tooltip="$t('repositories.title')">
                    <router-link :to="{ name: 'repos' }">
                      <Icon name="repo" class="size-4" />
                      <span>{{ $t('repositories.title') }}</span>
                      <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                      <div v-if="!isMobile" class="ml-auto flex items-center gap-0.5">
                        <Kbd>Alt</Kbd>
                        <Kbd>R</Kbd>
                      </div>
                      <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="user">
                  <SidebarMenuButton as-child :tooltip="$t('secrets.secrets')">
                    <router-link :to="{ name: 'user-secrets' }">
                      <Icon name="secret" class="size-4" />
                      <span>{{ $t('secrets.secrets') }}</span>
                      <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                      <div v-if="!isMobile" class="ml-auto flex items-center gap-0.5">
                        <Kbd>Alt</Kbd>
                        <Kbd>S</Kbd>
                      </div>
                      <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="user">
                  <SidebarMenuButton as-child :tooltip="$t('registries.registries')">
                    <router-link :to="{ name: 'user-registries' }">
                      <Icon name="docker" class="size-4" />
                      <span>{{ $t('registries.registries') }}</span>
                      <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                      <div v-if="!isMobile" class="ml-auto flex items-center gap-0.5">
                        <Kbd>Alt</Kbd>
                        <Kbd>Shift</Kbd>
                        <Kbd>R</Kbd>
                      </div>
                      <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="user">
                  <SidebarMenuButton as-child :tooltip="$t('repo.settings.crons.crons')">
                    <router-link :to="{ name: 'user-crons' }">
                      <Icon name="lucide:clock" class="size-4" />
                      <span>{{ $t('repo.settings.crons.crons') }}</span>
                      <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                      <div v-if="!isMobile" class="ml-auto flex items-center gap-0.5">
                        <Kbd>Alt</Kbd>
                        <Kbd>C</Kbd>
                      </div>
                      <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="user">
                  <SidebarMenuButton as-child :tooltip="$t('access_tokens.title')">
                    <router-link :to="{ name: 'user-access-tokens' }">
                      <Icon name="secret" class="size-4" />
                      <span>{{ $t('access_tokens.title') }}</span>
                      <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                      <div v-if="!isMobile" class="ml-auto flex items-center gap-0.5">
                        <Kbd>Alt</Kbd>
                        <Kbd>T</Kbd>
                      </div>
                      <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="user">
                  <SidebarMenuButton as-child :tooltip="$t('metrics.title')">
                    <router-link :to="{ name: 'user-metrics' }">
                      <Icon name="graph" class="size-4" />
                      <span>{{ $t('metrics.title') }}</span>
                      <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                      <div v-if="!isMobile" class="ml-auto flex items-center gap-0.5">
                        <Kbd>Alt</Kbd>
                        <Kbd>M</Kbd>
                      </div>
                      <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="user">
                  <SidebarMenuButton as-child :tooltip="$t('admin.settings.agents.agents')">
                    <router-link :to="{ name: 'agents' }">
                      <Icon name="agent" class="size-4" />
                      <span>{{ $t('admin.settings.agents.agents') }}</span>
                      <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                      <div v-if="!isMobile" class="ml-auto flex items-center gap-0.5">
                        <Kbd>Alt</Kbd>
                        <Kbd>A</Kbd>
                      </div>
                      <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="user">
                  <SidebarMenuButton as-child :tooltip="$t('admin.settings.autoscalers.autoscalers')">
                    <router-link :to="{ name: 'autoscalers' }">
                      <Icon name="autoscaler" class="size-4" />
                      <span>{{ $t('admin.settings.autoscalers.autoscalers') }}</span>
                      <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                      <div v-if="!isMobile" class="ml-auto flex items-center gap-0.5">
                        <Kbd>Alt</Kbd>
                        <Kbd>U</Kbd>
                      </div>
                      <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem v-if="user">
                  <SidebarMenuButton as-child :tooltip="$t('integrations.title')">
                    <router-link :to="{ name: 'user-integrations' }">
                      <Icon name="lucide:plug-2" class="size-4" />
                      <span>{{ $t('integrations.title') }}</span>
                      <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                      <div v-if="!isMobile" class="ml-auto flex items-center gap-0.5">
                        <Kbd>Alt</Kbd>
                        <Kbd>I</Kbd>
                      </div>
                      <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </CollapsibleContent>
        </SidebarGroup>
      </Collapsible>

      <SidebarSeparator v-if="user?.admin" />

      <Collapsible v-if="user?.admin" :default-open="true" class="group/collapsible">
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
                <SidebarMenuItem>
                  <SidebarMenuButton as-child :tooltip="$t('admin.settings.maintenance.maintenance')">
                    <router-link :to="{ name: 'admin-settings-maintenance' }">
                      <Icon name="progress-wrench-outline" class="size-4" />
                      <span>{{ $t('admin.settings.maintenance.maintenance') }}</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem>
                  <SidebarMenuButton as-child :tooltip="$t('admin.settings.repos.repos')">
                    <router-link :to="{ name: 'admin-settings-repos' }">
                      <Icon name="repo" class="size-4" />
                      <span>{{ $t('admin.settings.repos.repos') }}</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem>
                  <SidebarMenuButton as-child :tooltip="$t('admin.settings.entities.title')">
                    <router-link :to="{ name: 'admin-settings-entities' }">
                      <Icon name="user" class="size-4" />
                      <span>{{ $t('admin.settings.entities.title') }}</span>
                    </router-link>
                  </SidebarMenuButton>
                </SidebarMenuItem>

                <SidebarMenuItem>
                  <SidebarMenuButton as-child :tooltip="$t('admin.settings.queue.queue')">
                    <router-link :to="{ name: 'admin-settings-queue' }">
                      <Icon name="tasks" class="size-4" />
                      <span>{{ $t('admin.settings.queue.queue') }}</span>
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
            <a href="https://crowci.dev/" target="_blank">
              <Icon name="lucide:book-open" class="size-4" />
              <span>{{ $t('docs') }}</span>
            </a>
          </SidebarMenuButton>
        </SidebarMenuItem>

        <SidebarMenuItem v-if="enableSwagger">
          <SidebarMenuButton as-child :tooltip="$t('api')">
            <a :href="apiUrl" target="_blank">
              <Icon name="lucide:code" class="size-4" />
              <span>{{ $t('api') }}</span>
            </a>
          </SidebarMenuButton>
        </SidebarMenuItem>

        <SidebarMenuItem v-if="user">
          <SidebarMenuButton as-child :tooltip="$t('logout')">
            <a :href="`${address}/logout`">
              <Icon name="logout" class="size-4" />
              <span>{{ $t('logout') }}</span>
            </a>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>

      <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
      <a
        v-if="isVersionTag"
        :href="`https://codeberg.org/crowci/crow/releases/tag/${versionInfo?.current || config.version}`"
        target="_blank"
        class="block px-2 py-1.5 text-xs text-muted-foreground space-y-0.5 hover:underline"
      >
        <div>Crow {{ versionInfo?.current || config.version }}</div>
        <div v-if="config.buildDate" class="text-[0.65rem] opacity-75">
          {{ formattedBuildDate }}
        </div>
      </a>
      <div v-else class="px-2 py-1.5 text-xs text-muted-foreground space-y-0.5">
        <div>Crow {{ versionInfo?.current || config.version }}</div>
        <div v-if="config.buildDate" class="text-[0.65rem] opacity-75">
          {{ formattedBuildDate }}
        </div>
      </div>
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
import { useVersion } from '~/compositions/useVersion';

const config = useConfig();
const { user } = useAuthentication();
const versionInfo = useVersion();
const { toggle: toggleCommandPalette } = useCommandPalette();
const { isMobile, setOpenMobile } = useSidebar();
const route = useRoute();

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
const apiUrl = `${config.rootPath ?? ''}/swagger/index.html`;
const address = `${window.location.protocol}//${window.location.host}${config.rootPath}`;
const { enableSwagger } = config;

// Detect OS for keyboard shortcut display
const isMac = computed(() => navigator.userAgent.includes('Mac'));

// Check if version is a tag (starts with 'v' and contains numbers, not 'dev')
const isVersionTag = computed(() => {
  const version = versionInfo.value?.current || config.version || '';
  return version.match(/^v\d+\.\d+/) && !version.includes('dev');
});

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

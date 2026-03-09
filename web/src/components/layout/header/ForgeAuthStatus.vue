<template>
  <DropdownMenu v-if="expiredForges.length > 0">
    <DropdownMenuTrigger as-child>
      <button
        class="flex items-center justify-center gap-1.5 px-2 py-1.5 mr-2 rounded-md transition-colors bg-destructive/10 hover:bg-destructive/20"
        :title="tooltipText"
      >
        <Icon name="lucide:heart-pulse" class="size-4 text-destructive" />
        <span class="text-xs font-semibold leading-none text-destructive tabular-nums">
          {{ expiredForges.length }}
        </span>
      </button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end" class="w-72">
      <DropdownMenuLabel class="flex items-center gap-2">
        <Icon name="lucide:heart-pulse" class="size-4 text-destructive" />
        {{ $t('navbar.forge_auth_expired') }}
      </DropdownMenuLabel>
      <DropdownMenuSeparator />
      <div class="max-h-48 overflow-y-auto">
        <div v-for="forge in expiredForges" :key="forge.id" class="px-2 py-2">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2 min-w-0">
              <img
                v-if="getForgeCustomIconUrl(forge)"
                :src="getForgeCustomIconUrl(forge)!"
                class="size-4 shrink-0"
                alt=""
              />
              <Icon v-else :name="getForgeIconName(forge)" class="size-4 shrink-0 text-muted-foreground" />
              <span class="text-sm truncate">{{ forge.forge_login }}</span>
            </div>
            <Button as-child variant="outline" size="sm" class="shrink-0 h-7 text-xs">
              <a :href="getReconnectUrl(forge.forge_id)">
                <Icon name="lucide:refresh-cw" class="size-3 mr-1" />
                {{ $t('user.settings.forges.reconnect') }}
              </a>
            </Button>
          </div>
        </div>
      </div>
      <DropdownMenuSeparator />
      <DropdownMenuItem as-child>
        <router-link :to="{ name: 'user' }" class="flex w-full cursor-pointer items-center justify-center text-xs">
          {{ $t('navbar.forge_auth_view_all') }}
        </router-link>
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
import type { IconNames } from '~/components/atomic/Icon.types';
import { Button } from '~/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import useApiClient from '~/compositions/useApiClient';
import useConfig from '~/compositions/useConfig';
import type { UserForge } from '~/lib/api/types';

const { t } = useI18n();
const route = useRoute();
const apiClient = useApiClient();
const config = useConfig();

const userForges = ref<UserForge[]>([]);

const expiredForges = computed(() => userForges.value.filter((uf) => uf.status === 'expired'));

const tooltipText = computed(() => {
  return t('navbar.forge_auth_expired_tooltip', { count: expiredForges.value.length });
});

function isValidUrl(str: string): boolean {
  try {
    return Boolean(new URL(str));
  } catch {
    return false;
  }
}

function getForgeIconName(userForge: UserForge): IconNames {
  // Check for custom icon first
  if (userForge.forge_icon) {
    // If it's not a URL, treat it as a predefined icon name
    if (!isValidUrl(userForge.forge_icon)) {
      const customIcon = userForge.forge_icon.toLowerCase();
      const validIcons: Record<string, IconNames> = {
        codeberg: 'codeberg',
        codefloe: 'codefloe',
        github: 'github',
        gitlab: 'gitlab',
        gitea: 'gitea',
        forgejo: 'forgejo',
        bitbucket: 'bitbucket',
        repo: 'repo',
      };
      if (validIcons[customIcon]) {
        return validIcons[customIcon];
      }
    }
  }

  const forgeType = userForge.forge_type?.toLowerCase() || '';
  const forgeName = userForge.forge_name?.toLowerCase() || '';

  if (forgeName.includes('codeberg')) return 'codeberg';
  if (forgeName.includes('codefloe')) return 'codefloe';

  const typeToIcon: Record<string, IconNames> = {
    github: 'github',
    gitlab: 'gitlab',
    gitea: 'gitea',
    forgejo: 'forgejo',
    bitbucket: 'bitbucket',
    'bitbucket-dc': 'bitbucket',
    addon: 'repo',
  };
  return typeToIcon[forgeType] || 'repo';
}

function getForgeCustomIconUrl(userForge: UserForge): string | null {
  if (userForge.forge_icon && isValidUrl(userForge.forge_icon)) {
    return userForge.forge_icon;
  }
  return null;
}

const getReconnectUrl = (forgeId: number): string => {
  const redirectTo = encodeURIComponent(route.fullPath);
  return `${config.rootPath}/authorize?forge_id=${forgeId}&reconnect=true&redirect_to=${redirectTo}`;
};

const loadForges = async () => {
  try {
    userForges.value = await apiClient.getUserForges();
  } catch {
    // Silently fail - user might not be authenticated or API error
    userForges.value = [];
  }
};

onMounted(async () => {
  await loadForges();
});
</script>

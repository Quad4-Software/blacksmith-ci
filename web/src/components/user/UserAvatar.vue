<template>
  <HoverCard :open-delay="200" @update:open="handleOpenChange">
    <HoverCardTrigger as-child>
      <img :src="avatarUrl" :alt="username" class="rounded-md object-cover cursor-pointer" :class="sizeClass" />
    </HoverCardTrigger>
    <HoverCardContent class="w-80" side="top">
      <div v-if="isLoading" class="flex items-center justify-center py-4">
        <Icon name="loading" class="h-5 w-5 animate-spin" />
      </div>
      <div v-else class="flex gap-4 relative">
        <img :src="avatarUrl" :alt="username" class="h-12 w-12 rounded-md object-cover" />
        <div class="space-y-2 flex-1">
          <div class="flex items-center gap-2">
            <a
              v-if="forgeProfileUrl"
              :href="forgeProfileUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="text-sm font-semibold hover:underline"
            >
              {{ username }}
            </a>
            <h4 v-else class="text-sm font-semibold">{{ username }}</h4>
            <Badge v-if="userData?.admin" variant="default" class="text-xs">
              {{ $t('admin.settings.users.admin.admin') }}
            </Badge>
          </div>
          <p v-if="userData?.description" class="text-sm text-muted-foreground">
            {{ userData.description }}
          </p>
          <div v-if="userData?.timezone" class="flex items-center gap-1">
            <Icon name="lucide:clock" class="h-3 w-3 opacity-70" />
            <span class="text-xs text-muted-foreground">{{ userData.timezone }}</span>
          </div>
          <div v-if="orgs && orgs.length > 0" class="flex flex-wrap gap-1 pt-1">
            <Badge v-for="org in orgs" :key="org.id" variant="secondary" class="text-xs">
              {{ org.name }}
            </Badge>
          </div>
        </div>
        <Icon v-if="forgeData" :name="getForgeIconName(forgeData)" class="absolute top-0 right-0 h-4 w-4 opacity-70" />
      </div>
    </HoverCardContent>
  </HoverCard>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import { Badge } from '~/components/ui/badge';
import { HoverCard, HoverCardContent, HoverCardTrigger } from '~/components/ui/hover-card';
import useApiClient from '~/compositions/useApiClient';
import type { User, Forge } from '~/lib/api/types';
import type { IconNames } from '~/components/atomic/Icon.types';

interface Org {
  id: number;
  name: string;
}

const props = withDefaults(
  defineProps<{
    avatarUrl: string;
    username: string;
    description?: string;
    timezone?: string;
    orgs?: Org[];
    size?: 'sm' | 'md' | 'lg';
  }>(),
  {
    size: 'md',
  },
);

const apiClient = useApiClient();
const userData = ref<User | null>(null);
const forgeData = ref<Forge | null>(null);
const isLoading = ref(false);
const hasFetched = ref(false);

// Compute forge profile URL
const forgeProfileUrl = computed(() => {
  if (!forgeData.value || !props.username) return null;

  const forgeUrl = forgeData.value.url.replace(/\/$/, ''); // Remove trailing slash
  return `${forgeUrl}/${props.username}`;
});

async function handleOpenChange(open: boolean) {
  if (open && !hasFetched.value) {
    isLoading.value = true;
    try {
      userData.value = await apiClient.getUser(props.username);

      // Fetch forge data if user has a forge_id
      if (userData.value?.forge_id) {
        try {
          forgeData.value = await apiClient.getForge(userData.value.forge_id);
        } catch (error) {
          console.error('Failed to fetch forge data:', error);
        }
      }

      hasFetched.value = true;
    } catch (error) {
      console.error('Failed to fetch user data:', error);
    } finally {
      isLoading.value = false;
    }
  }
}

// Map forge types to local icon names (same logic as RepoItem)
function getForgeIconName(_forge: Forge): IconNames {
  // TEMP: Hardcoded for inspection
  return 'gitea';
}

const sizeClass = computed(() => {
  switch (props.size) {
    case 'sm':
      return 'w-5 h-5';
    case 'md':
      return 'w-6 h-6';
    case 'lg':
      return 'w-8 h-8';
    default:
      return 'w-6 h-6';
  }
});
</script>

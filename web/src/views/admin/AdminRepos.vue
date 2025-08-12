<template>
  <Settings :title="$t('admin.settings.repos.repos')" :description="$t('admin.settings.repos.desc')">
    <template #headerActions>
      <Button
        start-icon="heal"
        :is-loading="isRepairingRepos"
        :text="$t('admin.settings.repos.repair.repair')"
        @click="repairRepos"
      />
    </template>
    <div class="space-y-4 text-wp-text-100">
      <!-- Filter controls -->
      <div v-if="repos && repos.length > 0" class="mb-4 space-y-2">
        <div class="flex flex-wrap gap-2">
          <!-- Search input -->
          <div class="flex-1 min-w-[200px]">
            <TextField
              v-model="searchQuery"
              :placeholder="$t('admin.settings.repos.search')"
              type="text"
              class="h-10 shadow-xs"
            />
          </div>

          <!-- Clear filters button -->
          <Button
            v-if="hasActiveFilters"
            :text="$t('admin.settings.repos.clear_filters')"
            color="gray"
            @click="clearFilters"
          />
        </div>
      </div>

      <ListItem
        v-for="repo in filteredRepos"
        :key="repo.id"
        class="items-center gap-2 bg-wp-background-200 dark:bg-wp-background-100!"
      >
        <div class="flex items-center gap-2">
          <span>{{ repo.full_name }}</span>
          <!-- Forge profile link with local icon -->
          <a
            v-if="getForgeForRepo(repo) && repo.forge_url"
            :href="repo.forge_url"
            :title="`View ${repo.full_name} on ${getForgeHostname(getForgeForRepo(repo))}`"
            target="_blank"
            class="inline-flex items-center justify-center h-6 w-6 hover:opacity-80 transition-opacity"
          >
            <Icon :name="getForgeIconName(getForgeForRepo(repo)!)" class="h-4 w-4 text-wp-text-100" />
          </a>
        </div>
        <div class="ml-auto flex items-center">
          <Badge
            v-if="!repo.active"
            class="md:display-unset mr-2 hidden"
            :label="$t('admin.settings.repos.disabled')"
          />
          <IconButton
            icon="chevron-right"
            :title="$t('admin.settings.repos.view')"
            class="h-8 w-8"
            :to="{ name: 'repo', params: { repoId: repo.id } }"
          />
          <IconButton
            icon="settings-outline"
            :title="$t('admin.settings.repos.settings')"
            class="h-8 w-8"
            :to="{ name: 'repo-settings', params: { repoId: repo.id } }"
          />
        </div>
      </ListItem>
      <div v-if="filteredRepos.length === 0 && repos && repos.length > 0" class="ml-2">
        {{ $t('admin.settings.repos.no_match') }}
      </div>
      <div v-if="repos?.length === 0" class="ml-2">{{ $t('admin.settings.repos.none') }}</div>
    </div>
  </Settings>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Badge from '~/components/atomic/Badge.vue';
import Button from '~/components/atomic/Button.vue';
import Icon from '~/components/atomic/Icon.vue';
import IconButton from '~/components/atomic/IconButton.vue';
import ListItem from '~/components/atomic/ListItem.vue';
import TextField from '~/components/form/TextField.vue';
import Settings from '~/components/layout/Settings.vue';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useNotifications from '~/compositions/useNotifications';
import { usePagination } from '~/compositions/usePaginate';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { Repo, Forge } from '~/lib/api/types';
import type { IconNames } from '~/components/atomic/Icon.vue';

const apiClient = useApiClient();
const notifications = useNotifications();
const i18n = useI18n();

async function loadRepos(page: number): Promise<Repo[] | null> {
  return apiClient.getAllRepos({ page });
}

const { data: repos } = usePagination(loadRepos);

// Load forges for forge favicon URLs
const forges = ref<Forge[]>([]);

function getForgeForRepo(repo: Repo): Forge | null {
  return forges.value.find((f) => f.id === repo.forge_id) || null;
}

// Map forge types to local icon names
function getForgeIconName(forge: Forge): IconNames {
  // Handle special cases where forge URL might indicate specific forge instances
  if (forge.url.toLowerCase().includes('codeberg')) {
    return 'codeberg';
  }
  if (forge.url.toLowerCase().includes('codefloe')) {
    return 'codefloe';
  }

  // Map standard forge types to icon names
  const typeToIcon: Record<string, IconNames> = {
    github: 'github',
    gitlab: 'gitlab',
    gitea: 'gitea',
    forgejo: 'forgejo',
    bitbucket: 'bitbucket',
    'bitbucket-dc': 'bitbucket',
    addon: 'repo', // fallback for unknown types
  };

  return typeToIcon[forge.type] || 'repo';
}

function getForgeHostname(forge: Forge | null): string {
  if (!forge?.url) return '';
  try {
    const url = new URL(forge.url);
    return url.hostname;
  } catch {
    return forge.url;
  }
}

// Load forges on component mount
onMounted(async () => {
  try {
    forges.value = (await apiClient.getForges()) ?? [];
  } catch (error) {
    console.warn('Failed to load forges:', error);
  }
});

// Filter state
const searchQuery = ref('');

// Check if any filters are active
const hasActiveFilters = computed(() => {
  return searchQuery.value !== '';
});

// Clear all filters
function clearFilters() {
  searchQuery.value = '';
}

// Filtered repos based on search query
const filteredRepos = computed(() => {
  if (!repos.value) return [];

  return repos.value.filter((repo) => {
    // Search query - search in full_name, owner, and name
    if (searchQuery.value) {
      const query = searchQuery.value.toLowerCase();
      const fullNameMatch = repo.full_name?.toLowerCase().includes(query) || false;
      const ownerMatch = repo.owner?.toLowerCase().includes(query) || false;
      const nameMatch = repo.name?.toLowerCase().includes(query) || false;

      if (!fullNameMatch && !ownerMatch && !nameMatch) {
        return false;
      }
    }

    return true;
  });
});

const { doSubmit: repairRepos, isLoading: isRepairingRepos } = useAsyncAction(async () => {
  await apiClient.repairAllRepos();
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('admin.settings.repos.repair.notify_text_success'),
    type: 'success',
    duration: 6000,
  });
});

useCrowTitle(computed(() => [i18n.t('admin.settings.repos.repos'), i18n.t('admin.settings.settings')]));
</script>

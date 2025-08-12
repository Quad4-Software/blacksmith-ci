<template>
  <Settings :title="$t('admin.settings.orgs.orgs')" :description="$t('admin.settings.orgs.desc')">
    <div class="space-y-4 text-wp-text-100">
      <!-- Filter controls -->
      <div v-if="orgs && orgs.length > 0" class="mb-4 space-y-2">
        <div class="flex flex-wrap gap-2">
          <!-- Search input -->
          <div class="flex-1 min-w-[200px]">
            <TextField
              v-model="searchQuery"
              :placeholder="$t('admin.settings.orgs.search')"
              type="text"
              class="h-10 shadow-xs"
            />
          </div>

          <!-- Clear filters button -->
          <Button
            v-if="hasActiveFilters"
            :text="$t('admin.settings.orgs.clear_filters')"
            color="gray"
            @click="clearFilters"
          />
        </div>
      </div>

      <ListItem
        v-for="org in filteredOrgs"
        :key="org.id"
        class="items-center gap-2 bg-wp-background-200! dark:bg-wp-background-100!"
      >
        <span>{{ org.name }}</span>
        <!-- Forge profile link with local icon -->
        <a
          v-if="primaryForge && org.name"
          :href="getOrgForgeUrl(org)"
          :title="`View ${org.name} on ${primaryForge.url}`"
          target="_blank"
          class="inline-flex items-center justify-center h-6 w-6 hover:opacity-80 transition-opacity"
        >
          <Icon :name="getForgeIconName(primaryForge)" class="h-4 w-4 text-wp-text-100" />
        </a>
        <IconButton
          icon="chevron-right"
          :title="$t('admin.settings.orgs.view')"
          class="ml-auto h-8 w-8"
          :to="{ name: 'org', params: { orgId: org.id } }"
        />
        <IconButton
          icon="settings-outline"
          :title="$t('admin.settings.orgs.org_settings')"
          class="h-8 w-8"
          :to="{ name: 'org-settings', params: { orgId: org.id } }"
        />
        <IconButton
          icon="trash"
          :title="$t('admin.settings.orgs.delete_org')"
          class="ml-2 h-8 w-8 hover:text-wp-error-100"
          :is-loading="isDeleting"
          @click="deleteOrg(org)"
        />
      </ListItem>
      <div v-if="filteredOrgs.length === 0 && orgs && orgs.length > 0" class="ml-2">
        {{ $t('admin.settings.orgs.no_match') }}
      </div>
      <div v-if="orgs?.length === 0" class="ml-2">{{ $t('admin.settings.orgs.none') }}</div>
    </div>
  </Settings>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Button from '~/components/atomic/Button.vue';
import Icon from '~/components/atomic/Icon.vue';
import ListItem from '~/components/atomic/ListItem.vue';
import TextField from '~/components/form/TextField.vue';
import Settings from '~/components/layout/Settings.vue';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useNotifications from '~/compositions/useNotifications';
import { usePagination } from '~/compositions/usePaginate';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { Org, Forge } from '~/lib/api/types';
import type { IconNames } from '~/components/atomic/Icon.vue';

const apiClient = useApiClient();
const notifications = useNotifications();
const { t } = useI18n();

// Load forges for forge URL construction
const forges = ref<Forge[]>([]);
const primaryForge = computed(() => forges.value[0] || null);

async function loadOrgs(page: number): Promise<Org[] | null> {
  return apiClient.getOrgs({ page });
}

const { resetPage, data: orgs } = usePagination(loadOrgs);

// Load forges on component mount
onMounted(async () => {
  try {
    forges.value = (await apiClient.getForges()) ?? [];
    // Note: Skip fetchFaviconFromHead due to CORS issues, rely on fallback extensions
  } catch (error) {
    console.warn('Failed to load forges:', error);
  }
});

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

function getOrgForgeUrl(org: Org): string {
  if (!primaryForge.value?.url || !org.name) return '';
  try {
    const baseUrl = primaryForge.value.url.endsWith('/') ? primaryForge.value.url.slice(0, -1) : primaryForge.value.url;
    return `${baseUrl}/${org.name}`;
  } catch {
    return '';
  }
}

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

// Filtered orgs based on search query
const filteredOrgs = computed(() => {
  if (!orgs.value) return [];

  return orgs.value.filter((org) => {
    // Search query - search in name
    if (searchQuery.value) {
      const query = searchQuery.value.toLowerCase();
      const nameMatch = org.name?.toLowerCase().includes(query) || false;

      if (!nameMatch) {
        return false;
      }
    }

    return true;
  });
});

const { doSubmit: deleteOrg, isLoading: isDeleting } = useAsyncAction(async (_org: Org) => {
  // eslint-disable-next-line no-alert
  if (!confirm(t('admin.settings.orgs.delete_confirm'))) {
    return;
  }

  await apiClient.deleteOrg(_org);
  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.orgs.notify_text_deleted'),
    type: 'success',
    duration: 6000,
  });
  resetPage();
});

useCrowTitle(computed(() => [t('admin.settings.orgs.orgs'), t('admin.settings.settings')]));
</script>

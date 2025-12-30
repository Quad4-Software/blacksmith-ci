<template>
  <Dialog v-model:open="repositoryDialog.isOpen.value">
    <DialogContent class="sm:max-w-[700px] max-h-[85vh] flex flex-col">
      <DialogHeader>
        <DialogTitle>{{ $t('repo.add') }}</DialogTitle>
        <DialogDescription>
          {{ $t('repo.add_desc') }}
        </DialogDescription>
      </DialogHeader>

      <div class="flex-1 flex flex-col min-h-0 mt-4">
        <!-- Search and filter controls -->
        <div class="flex flex-col sm:flex-row gap-2 mb-4">
          <SearchInput v-model="search" :placeholder="$t('search')" class="flex-1" />
          <Select v-model="ownerFilter">
            <SelectTrigger class="w-full sm:w-[180px]">
              <SelectValue :placeholder="$t('repo.enable.filter.all_owners')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ $t('repo.enable.filter.all_owners') }}</SelectItem>
              <SelectItem v-for="owner in owners" :key="owner" :value="owner">{{ owner }}</SelectItem>
            </SelectContent>
          </Select>
          <Select v-model="statusFilter">
            <SelectTrigger class="w-full sm:w-[160px]">
              <SelectValue :placeholder="$t('repo.enable.filter.all')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ $t('repo.enable.filter.all') }}</SelectItem>
              <SelectItem value="disabled">{{ $t('repo.enable.filter.not_enabled') }}</SelectItem>
              <SelectItem value="enabled">{{ $t('repo.enable.filter.enabled') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <!-- Selection actions -->
        <div v-if="notEnabledFilteredRepos.length > 0" class="flex items-center justify-between mb-3 px-1">
          <Button v-if="!isAllSelected" variant="ghost" size="sm" class="h-7 px-2 text-xs" @click="selectAll">
            {{ $t('repo.enable.select_all', { count: notEnabledFilteredRepos.length }) }}
          </Button>
          <Button v-else variant="ghost" size="sm" class="h-7 px-2 text-xs" @click="clearSelection">
            {{ $t('repo.enable.clear_selection') }}
          </Button>
          <span v-if="selectedRepos.size > 0" class="text-sm text-muted-foreground">
            {{ $t('repo.enable.selected_count', { count: selectedRepos.size }) }}
          </span>
        </div>

        <!-- Repository list -->
        <div class="flex-1 overflow-y-auto space-y-1 min-h-0">
          <template v-if="repos !== undefined && filteredRepos.length > 0">
            <div
              v-for="repo in filteredRepos"
              :key="repo.forge_remote_id"
              class="flex items-center gap-3 p-3 rounded-md border border-border hover:bg-accent/50 transition-colors"
              :class="{ 'bg-accent/30': selectedRepos.has(repo.forge_remote_id) }"
            >
              <!-- Checkbox for disabled repos -->
              <Checkbox
                v-if="!repo.active"
                :checked="selectedRepos.has(repo.forge_remote_id)"
                @update:checked="toggleRepoSelection(repo)"
              />
              <div v-else class="w-4" />

              <!-- Repo info -->
              <div class="flex-1 min-w-0">
                <span class="text-sm text-foreground truncate block">{{ repo.full_name }}</span>
              </div>

              <!-- Status / Action -->
              <div class="flex items-center gap-2 shrink-0">
                <Badge v-if="repo.active" variant="secondary" class="text-xs">
                  {{ $t('repo.enable.enabled') }}
                </Badge>
                <Button
                  v-else
                  size="sm"
                  variant="outline"
                  :disabled="isActivatingRepo"
                  @click.stop="activateSingleRepo(repo)"
                >
                  <Icon
                    v-if="isActivatingRepo && activatingRepoIds.has(repo.forge_remote_id)"
                    name="spinner"
                    class="mr-1.5 h-3 w-3 animate-spin"
                  />
                  {{ $t('repo.enable.enable') }}
                </Button>
              </div>
            </div>
          </template>
          <div v-else-if="loading" class="flex justify-center p-8">
            <Icon name="spinner" class="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
          <div v-else-if="repos !== undefined && repos.length === 0" class="text-center p-8 text-muted-foreground">
            {{ $t('no_repos') }}
          </div>
          <div v-else-if="filteredRepos.length === 0" class="text-center p-8 text-muted-foreground">
            {{ $t('no_search_results') }}
          </div>
        </div>
      </div>

      <!-- Footer actions -->
      <div class="flex items-center justify-between gap-2 mt-4 pt-4 border-t border-border">
        <div class="text-sm text-muted-foreground">
          <template v-if="repos">
            {{
              $t('repo.enable.total_repos', {
                total: filteredRepos.length,
                not_enabled: notEnabledFilteredRepos.length,
              })
            }}
          </template>
        </div>
        <div class="flex gap-2">
          <Button variant="outline" @click="handleClose">
            {{ $t('close') }}
          </Button>
          <Button v-if="selectedRepos.size > 0" :disabled="isActivatingRepo" @click="activateSelectedRepos">
            <Icon v-if="isActivatingRepo" name="spinner" class="mr-2 h-4 w-4 animate-spin" />
            {{ $t('repo.enable.enable_selected', { count: selectedRepos.size }) }}
          </Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import SearchInput from '~/components/ui/input/SearchInput.vue';
import useApiClient from '~/compositions/useApiClient';
import { useGlobalDialogs } from '~/compositions/useGlobalDialogs';
import useNotifications from '~/compositions/useNotifications';
import type { Repo } from '~/lib/api/types';

const { t } = useI18n();
const router = useRouter();
const apiClient = useApiClient();
const notifications = useNotifications();
const { repositoryDialog } = useGlobalDialogs();

const repos = ref<Repo[]>();
const search = ref('');
const ownerFilter = ref('all');
const statusFilter = ref('all');
const loading = ref(false);
const selectedRepos = ref<Set<string>>(new Set());
const activatingRepoIds = ref<Set<string>>(new Set());
const isActivatingRepo = ref(false);

// Computed: unique owners from repos
const owners = computed(() => {
  if (!repos.value) return [];
  const ownerSet = new Set<string>();
  for (const repo of repos.value) {
    ownerSet.add(repo.owner);
  }
  return Array.from(ownerSet).sort();
});

// Computed: filtered repos based on search, owner and status
const filteredRepos = computed(() => {
  if (!repos.value) return [];

  let filtered = repos.value;

  // Apply owner filter
  if (ownerFilter.value !== 'all') {
    filtered = filtered.filter((r) => r.owner === ownerFilter.value);
  }

  // Apply status filter
  if (statusFilter.value === 'disabled') {
    filtered = filtered.filter((r) => !r.active);
  } else if (statusFilter.value === 'enabled') {
    filtered = filtered.filter((r) => r.active);
  }

  // Apply search filter
  if (search.value.trim()) {
    const query = search.value.toLowerCase();
    filtered = filtered.filter((r) => r.full_name.toLowerCase().includes(query));
  }

  return filtered;
});

// Computed: not-enabled repos in current filtered view
const notEnabledFilteredRepos = computed(() => filteredRepos.value.filter((r) => !r.active));

// Computed: check if all not-enabled filtered repos are selected
const isAllSelected = computed(
  () =>
    notEnabledFilteredRepos.value.length > 0 &&
    notEnabledFilteredRepos.value.every((r) => selectedRepos.value.has(r.forge_remote_id)),
);

// Load repos when dialog opens
watch(
  () => repositoryDialog.isOpen.value,
  async (isOpen) => {
    if (isOpen) {
      // Reset state
      search.value = '';
      ownerFilter.value = 'all';
      statusFilter.value = 'all';
      selectedRepos.value = new Set();

      if (!repos.value) {
        loading.value = true;
        try {
          repos.value = await apiClient.getRepoList({ all: true });
        } catch {
          notifications.notify({
            title: t('notify_title_error'),
            text: t('unknown_error'),
            type: 'error',
          });
        } finally {
          loading.value = false;
        }
      }
    }
  },
);

function toggleRepoSelection(repo: Repo) {
  const newSet = new Set(selectedRepos.value);
  if (newSet.has(repo.forge_remote_id)) {
    newSet.delete(repo.forge_remote_id);
  } else {
    newSet.add(repo.forge_remote_id);
  }
  selectedRepos.value = newSet;
}

function selectAll() {
  const newSet = new Set(selectedRepos.value);
  for (const repo of notEnabledFilteredRepos.value) {
    newSet.add(repo.forge_remote_id);
  }
  selectedRepos.value = newSet;
}

function clearSelection() {
  selectedRepos.value = new Set();
}

async function activateSingleRepo(repo: Repo) {
  isActivatingRepo.value = true;
  activatingRepoIds.value.add(repo.forge_remote_id);

  try {
    const activatedRepo = await apiClient.activateRepo(repo.forge_remote_id);

    // Update local state
    if (repos.value) {
      const index = repos.value.findIndex((r) => r.forge_remote_id === repo.forge_remote_id);
      if (index !== -1) {
        repos.value[index] = { ...repos.value[index], active: true, id: activatedRepo.id };
      }
    }

    notifications.notify({
      title: t('notify_title_success'),
      text: t('repo.enable.notify_text_enabled'),
      type: 'success',
    });

    // Navigate to the repo
    repositoryDialog.close();
    await router.push({ name: 'repo', params: { repoId: activatedRepo.id } });
  } catch {
    notifications.notify({
      title: t('notify_title_error'),
      text: t('repo.enable.notify_text_error'),
      type: 'error',
    });
  } finally {
    activatingRepoIds.value.delete(repo.forge_remote_id);
    isActivatingRepo.value = false;
  }
}

async function activateSelectedRepos() {
  if (selectedRepos.value.size === 0) return;

  isActivatingRepo.value = true;
  const reposToActivate = repos.value?.filter((r) => selectedRepos.value.has(r.forge_remote_id)) ?? [];

  let successCount = 0;
  let failedCount = 0;
  let lastActivatedRepo: Repo | null = null;

  for (const repo of reposToActivate) {
    activatingRepoIds.value.add(repo.forge_remote_id);

    try {
      const activatedRepo = await apiClient.activateRepo(repo.forge_remote_id);

      // Update local state
      if (repos.value) {
        const index = repos.value.findIndex((r) => r.forge_remote_id === repo.forge_remote_id);
        if (index !== -1) {
          repos.value[index] = { ...repos.value[index], active: true, id: activatedRepo.id };
        }
      }

      successCount++;
      lastActivatedRepo = activatedRepo;
    } catch {
      failedCount++;
    } finally {
      activatingRepoIds.value.delete(repo.forge_remote_id);
    }
  }

  // Clear selection
  selectedRepos.value = new Set();
  isActivatingRepo.value = false;

  // Show notification
  if (successCount > 0) {
    notifications.notify({
      title: t('notify_title_success'),
      text: t('repo.enable.notify_text_enabled_multiple', { count: successCount }),
      type: 'success',
    });
  }

  if (failedCount > 0) {
    notifications.notify({
      title: t('notify_title_error'),
      text: t('repo.enable.notify_text_error_multiple', { count: failedCount }),
      type: 'error',
    });
  }

  // Navigate to the last activated repo if only one was selected, otherwise stay
  if (successCount === 1 && lastActivatedRepo) {
    repositoryDialog.close();
    await router.push({ name: 'repo', params: { repoId: lastActivatedRepo.id } });
  }
}

function handleClose() {
  search.value = '';
  ownerFilter.value = 'all';
  statusFilter.value = 'all';
  selectedRepos.value = new Set();
  repositoryDialog.close();
}
</script>

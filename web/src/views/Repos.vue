<template>
  <Scaffold fluid-content>
    <template #title>
      {{ $t('repositories.title') }}
    </template>

    <template v-if="isAuthenticated" #headerActions>
      <Button size="sm" @click="repositoryDialog.open()">
        <Icon name="plus" class="mr-2 h-4 w-4" />
        {{ $t('repo.add') }}
      </Button>
    </template>

    <div class="max-w-6xl mx-auto px-3 py-4 sm:px-4 space-y-4 w-full overflow-y-auto flex-1">
      <!-- Toolbar with Search and Filters -->
      <div class="flex items-center justify-center gap-2 flex-wrap">
        <SearchInput v-model="search" :placeholder="$t('search')" class="w-[300px] min-w-[80px]" />
        <RepoSortButton v-model:sort-by="sortBy" />
        <!-- Only show filter button for authenticated users -->
        <RepoFilterButton
          v-if="isAuthenticated"
          :owners="organizationOwners"
          :selected-owners="selectedOwners"
          :show-all-repos="showAllRepos"
          :active-filter-count="activeFilterCount"
          @toggle-owner="toggleOwner"
          @toggle-show-all="showAllRepos = !showAllRepos"
          @clear-filters="clearAllFilters"
        />
      </div>

      <Transition name="fade" mode="out-in">
        <!-- Authenticated: show pinned + unpinned sections -->
        <div v-if="isAuthenticated && search === '' && filteredRepos.length > 0" class="flex flex-col gap-6 w-full">
          <!-- Pinned Repos Section -->
          <div v-if="pinnedRepos.length > 0" class="flex flex-col gap-4 w-full">
            <div>
              <h2 class="text-foreground text-lg">{{ $t('repositories.pinned.title') }}</h2>
              <span class="text-muted-foreground">{{ $t('repositories.pinned.desc') }}</span>
            </div>
            <draggable
              :model-value="pinnedRepos"
              item-key="id"
              handle=".drag-handle"
              ghost-class="opacity-50"
              animation="200"
              class="flex flex-col gap-3 w-full"
              @update:model-value="handleReorder"
            >
              <template #item="{ element: repo }">
                <RepoItem
                  :repo="repo"
                  :is-pinned="true"
                  :can-pin="true"
                  :show-drag-handle="true"
                  @toggle-pin="handleTogglePin(repo.id)"
                />
              </template>
            </draggable>
          </div>

          <!-- All Repos Section -->
          <div class="flex flex-col gap-3 w-full">
            <div>
              <h2 class="text-lg text-foreground">{{ $t('repositories.all.title') }}</h2>
            </div>
            <div class="flex flex-col gap-3 w-full">
              <RepoItem
                v-for="repo in reposLastActivity"
                :key="repo.id"
                :repo="repo"
                :is-pinned="isPinned(repo.id)"
                :can-pin="canPin"
                @toggle-pin="handleTogglePin(repo.id)"
              />
            </div>
          </div>
        </div>

        <!-- Anonymous or search active: simple list -->
        <div v-else class="flex flex-col w-full">
          <div v-if="reposLastActivity.length > 0" class="flex flex-col gap-3 w-full">
            <RepoItem
              v-for="repo in reposLastActivity"
              :key="repo.id"
              :repo="repo"
              :is-pinned="isAuthenticated && isPinned(repo.id)"
              :can-pin="isAuthenticated && canPin"
              @toggle-pin="handleTogglePin(repo.id)"
            />
          </div>
          <!-- Empty state: login prompt for anonymous, no search results for authenticated -->
          <div v-else-if="!isAuthenticated" class="flex flex-col items-center justify-center py-12 text-center">
            <span class="text-lg text-foreground mb-4">{{ $t('repositories.no_public') }}</span>
            <Button as-child>
              <router-link :to="`/login?url=${route.fullPath}`">
                {{ $t('login') }}
              </router-link>
            </Button>
          </div>
          <span v-else class="text-center text-lg text-foreground">{{ $t('no_search_results') }}</span>
        </div>
      </Transition>
    </div>
  </Scaffold>
</template>

<script lang="ts" setup>
import { useStorage } from '@vueuse/core';
import { computed, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute } from 'vue-router';
import draggable from 'vuedraggable';

import Icon from '~/components/atomic/Icon.vue';
import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import RepoItem from '~/components/repo/RepoItem.vue';
import RepoFilterButton from '~/components/repo/RepoFilterButton.vue';
import RepoSortButton from '~/components/repo/RepoSortButton.vue';
import { Button } from '~/components/ui/button';
import SearchInput from '~/components/ui/input/SearchInput.vue';
import useAuthentication from '~/compositions/useAuthentication';
import useRepos from '~/compositions/useRepos';
import { useRepoSearch } from '~/compositions/useRepoSearch';
import { useRepoFilters } from '~/compositions/useRepoFilters';
import { usePinnedRepos } from '~/compositions/usePinnedRepos';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import { useGlobalDialogs } from '~/compositions/useGlobalDialogs';
import useNotifications from '~/compositions/useNotifications';
import { useRepoStore } from '~/store/repos';

const route = useRoute();
const { isAuthenticated } = useAuthentication();
const { repositoryDialog } = useGlobalDialogs();

const repoStore = useRepoStore();
const notifications = useNotifications();

const { sortReposByLastActivity, repoWithLastPipeline } = useRepos();
const { selectedOwners, showAllRepos, activeFilterCount, filterReposByOwner, toggleOwner, clearAllFilters } =
  useRepoFilters();
const { isPinned, canPin, togglePin, getPinnedRepos, getUnpinnedRepos, reorderPins } = usePinnedRepos();

// Sort state with localStorage persistence
const sortBy = useStorage<string>('crow:repos-sort-by', 'last_pipeline');

const allRepos = computed(() => repoStore.ownedRepos.map((r) => repoWithLastPipeline(r)));

// Extract unique owners from all repos (including user's own repos)
const organizationOwners = computed(() => {
  const owners = new Set<string>();
  allRepos.value.forEach((repo) => {
    owners.add(repo.owner);
  });
  return Array.from(owners).sort();
});

// Apply owner filter
const filteredRepos = computed(() => filterReposByOwner(allRepos.value));

// Apply search to all filtered repos
const search = ref('');
const { searchedRepos } = useRepoSearch(filteredRepos, search);

// Separate pinned and unpinned repos (only when not searching)
const pinnedRepos = computed(() => (search.value === '' ? getPinnedRepos(searchedRepos.value || []) : []));
const unpinnedRepos = computed(() =>
  search.value === '' ? getUnpinnedRepos(searchedRepos.value || []) : searchedRepos.value || [],
);

// Sort repos based on selected sort option
const reposLastActivity = computed(() => {
  const repos = unpinnedRepos.value || [];

  switch (sortBy.value) {
    case 'alphabetical':
      return [...repos].sort((a, b) => a.name.localeCompare(b.name));
    case 'status':
      // Sort by pipeline status (failed first, then running, then success, then no pipeline)
      return [...repos].sort((a, b) => {
        const statusOrder: Record<string, number> = {
          failure: 0,
          error: 1,
          killed: 2,
          running: 3,
          pending: 4,
          success: 5,
          skipped: 6,
        };
        const aStatus = a.last_pipeline?.status || 'zzz'; // No pipeline goes last
        const bStatus = b.last_pipeline?.status || 'zzz';
        const aOrder = statusOrder[aStatus] ?? 99;
        const bOrder = statusOrder[bStatus] ?? 99;
        return aOrder - bOrder;
      });
    case 'last_pipeline':
    default:
      return sortReposByLastActivity(repos);
  }
});

// Reload repos when showAllRepos filter changes
watch(showAllRepos, async (newValue) => {
  await repoStore.loadRepos(newValue);
});

const { t } = useI18n();

onMounted(async () => {
  await repoStore.loadRepos(showAllRepos.value);
});

// Handle pin toggle with error notifications
function handleTogglePin(repoId: number) {
  const result = togglePin(repoId);
  if (!result.success && result.error) {
    notifications.notify({
      title: t('repositories.pin_limit_reached'),
      text: result.error,
      type: 'error',
    });
  }
}

// Handle drag-and-drop reorder of pinned repos
function handleReorder(reorderedRepos: typeof pinnedRepos.value) {
  const newOrder = reorderedRepos.map((repo) => repo.id);
  reorderPins(newOrder);
}

useCrowTitle(computed(() => [t('repositories.title')]));
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: all 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>

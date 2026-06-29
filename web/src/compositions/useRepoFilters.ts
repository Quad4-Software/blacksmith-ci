import { useStorage } from '@vueuse/core';
import { computed } from 'vue';
import type { Repo } from '~/lib/api/types';

export interface RepoFilters {
  selectedOwners: string[];
  showAllRepos: boolean;
  showPublicRepos: boolean;
}

const DEFAULT_FILTERS: RepoFilters = {
  selectedOwners: [],
  showAllRepos: false,
  showPublicRepos: false,
};

export function useRepoFilters() {
  // Persist filter state to localStorage
  const selectedOwners = useStorage<string[]>('crow:repo-filter-owners', DEFAULT_FILTERS.selectedOwners);
  const showAllRepos = useStorage<boolean>('crow:repo-filter-show-all', DEFAULT_FILTERS.showAllRepos);
  const showPublicRepos = useStorage<boolean>('crow:repo-filter-show-public', DEFAULT_FILTERS.showPublicRepos);

  // Check if any filters are active
  const hasActiveFilters = computed(() => selectedOwners.value.length > 0);

  const activeFilterCount = computed(() => {
    let count = 0;
    if (selectedOwners.value.length > 0) {
      count += selectedOwners.value.length;
    }
    if (showAllRepos.value) {
      count += 1;
    }
    if (showPublicRepos.value) {
      count += 1;
    }
    return count;
  });

  // Filter repos by owner
  function filterReposByOwner(repos: Repo[]): Repo[] {
    if (selectedOwners.value.length === 0) {
      return repos;
    }
    return repos.filter((repo) => selectedOwners.value.includes(repo.owner));
  }

  // Toggle owner in filter
  function toggleOwner(owner: string) {
    const index = selectedOwners.value.indexOf(owner);
    if (index > -1) {
      // Use immutable filter for proper reactivity
      selectedOwners.value = selectedOwners.value.filter((o) => o !== owner);
    } else {
      // Use spread for proper reactivity
      selectedOwners.value = [...selectedOwners.value, owner];
    }
  }

  // Clear owner filters
  function clearOwnerFilters() {
    selectedOwners.value = [];
  }

  // Clear all filters
  function clearAllFilters() {
    selectedOwners.value = [];
    showAllRepos.value = false;
    showPublicRepos.value = false;
  }

  return {
    selectedOwners,
    showAllRepos,
    showPublicRepos,
    hasActiveFilters,
    activeFilterCount,
    filterReposByOwner,
    toggleOwner,
    clearOwnerFilters,
    clearAllFilters,
  };
}

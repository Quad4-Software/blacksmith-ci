import { useStorage } from '@vueuse/core';
import { computed } from 'vue';
import type { Repo } from '~/lib/api/types';

const MAX_PINNED_REPOS = 4;

export function usePinnedRepos() {
  // Persist pinned repo IDs to localStorage
  const pinnedRepoIds = useStorage<number[]>('crow:pinned-repos', []);

  // Check if repo is pinned
  function isPinned(repoId: number): boolean {
    return pinnedRepoIds.value.includes(repoId);
  }

  // Check if user can pin more repos
  const canPin = computed(() => pinnedRepoIds.value.length < MAX_PINNED_REPOS);

  // Toggle pin/unpin with validation
  function togglePin(repoId: number): { success: boolean; error?: string } {
    const index = pinnedRepoIds.value.indexOf(repoId);

    if (index > -1) {
      // Unpin - use immutable filter for proper reactivity
      pinnedRepoIds.value = pinnedRepoIds.value.filter((id) => id !== repoId);
      return { success: true };
    } else {
      // Pin
      if (pinnedRepoIds.value.length >= MAX_PINNED_REPOS) {
        return {
          success: false,
          error: 'Maximum 4 repositories can be pinned',
        };
      }
      // Use spread for proper reactivity
      pinnedRepoIds.value = [...pinnedRepoIds.value, repoId];
      return { success: true };
    }
  }

  // Get pinned repos from a list of repos, preserving pin order
  function getPinnedRepos(repos: Repo[]): Repo[] {
    const repoMap = new Map(repos.map((r) => [r.id, r]));
    const pinned: Repo[] = [];
    const validPinnedIds: number[] = [];

    // Preserve pin order and collect valid IDs
    pinnedRepoIds.value.forEach((pinnedId) => {
      const repo = repoMap.get(pinnedId);
      if (repo) {
        pinned.push(repo);
        validPinnedIds.push(pinnedId);
      }
    });

    // Clean up orphaned IDs (repos that no longer exist or user lost access to)
    if (validPinnedIds.length !== pinnedRepoIds.value.length) {
      pinnedRepoIds.value = validPinnedIds;
    }

    return pinned;
  }

  // Get unpinned repos from a list of repos
  function getUnpinnedRepos(repos: Repo[]): Repo[] {
    const pinnedSet = new Set(pinnedRepoIds.value);
    return repos.filter((repo) => !pinnedSet.has(repo.id));
  }

  // Reorder pinned repos by updating the ID array
  function reorderPins(newOrder: number[]) {
    pinnedRepoIds.value = newOrder;
  }

  return {
    pinnedRepoIds: computed(() => pinnedRepoIds.value),
    isPinned,
    canPin,
    togglePin,
    getPinnedRepos,
    getUnpinnedRepos,
    reorderPins,
  };
}

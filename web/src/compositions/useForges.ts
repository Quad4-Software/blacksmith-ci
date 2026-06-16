import { ref, computed } from 'vue';
import type { Forge } from '~/lib/api/types';
import useApiClient from '~/compositions/useApiClient';

// Global forge data - loaded once at app startup
const forges = ref<Forge[]>([]);
const isLoading = ref(true);
const isLoaded = ref(false);

let loadPromise: Promise<void> | null = null;

export function useForges() {
  const apiClient = useApiClient();

  // Load forges data once globally
  async function loadForges(): Promise<void> {
    if (loadPromise) {
      return loadPromise;
    }

    loadPromise = (async () => {
      try {
        isLoading.value = true;
        const data = await apiClient.getForges(undefined, { silent: true });
        forges.value = data ?? [];
        isLoaded.value = true;
      } catch (error) {
        console.warn('Failed to load forges:', error);
        forges.value = [];
      } finally {
        isLoading.value = false;
      }
    })();

    return loadPromise;
  }

  // Get forge by ID
  function getForgeById(id: number): Forge | null {
    return forges.value.find((f) => f.id === id) ?? null;
  }

  // Get primary forge (first one)
  const primaryForge = computed(() => (forges.value.length > 0 ? forges.value[0] : null));

  // Auto-load if not already loaded
  if (!isLoaded.value && !loadPromise) {
    void loadForges();
  }

  return {
    forges: computed(() => forges.value),
    isLoading: computed(() => isLoading.value),
    isLoaded: computed(() => isLoaded.value),
    primaryForge,
    getForgeById,
    loadForges,
  };
}

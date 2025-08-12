<template>
  <router-link
    v-if="repo"
    :to="{ name: 'repo', params: { repoId: repo.id } }"
    class="flex cursor-pointer flex-col overflow-hidden rounded-md border border-wp-background-400 bg-wp-background-100 p-3 shadow-xs hover:bg-wp-background-300 hover:shadow-xs dark:bg-wp-background-200 dark:hover:bg-wp-background-300"
  >
    <div class="grid grid-cols-[auto_1fr] items-center gap-y-4">
      <div class="flex items-center gap-2 text-lg text-wp-text-100">
        <span>{{ `${repo.owner} / ${repo.name}` }}</span>
        <!-- Forge profile link with local icon -->
        <a
          v-if="forge && repo.forge_url"
          :href="repo.forge_url"
          :title="`View ${repo.owner}/${repo.name} on ${getForgeUrl(forge)}`"
          target="_blank"
          class="inline-flex items-center justify-center h-6 w-6 hover:opacity-80 transition-opacity"
        >
          <Icon :name="getForgeIconName(forge)" class="h-4 w-4 text-wp-text-100" />
        </a>
      </div>

      <div class="ml-auto text-wp-text-100">
        <div
          v-if="repo.visibility === RepoVisibility.Private"
          :title="`${$t('repo.visibility.visibility')}: ${$t(`repo.visibility.private.private`)}`"
        >
          <Icon name="visibility-private" size="1.3rem" />
        </div>

        <div
          v-else-if="repo.visibility === RepoVisibility.Internal"
          :title="`${$t('repo.visibility.visibility')}: ${$t(`repo.visibility.internal.internal`)}`"
        >
          <Icon name="visibility-internal" />
        </div>
      </div>

      <div class="col-span-2 flex w-full gap-x-4 text-wp-text-100">
        <template v-if="lastPipeline">
          <div class="flex min-w-0 flex-1 items-center gap-x-1">
            <PipelineStatusIcon v-if="lastPipeline" :status="lastPipeline.status" />
            <span class="overflow-hidden text-ellipsis whitespace-nowrap pl-1">{{ shortMessage }}</span>
          </div>

          <div class="ml-auto flex shrink-0 items-center gap-x-1">
            <Icon name="startDate" size="1rem" /> <span>{{ startDate }}</span>
          </div>
        </template>
        <div v-else class="flex gap-x-2">
          <span>{{ $t('repo.pipeline.no_pipelines') }}</span>
        </div>
      </div>
    </div>
  </router-link>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import useApiClient from '~/compositions/useApiClient';
import usePipeline from '~/compositions/usePipeline';
import type { Repo, Forge } from '~/lib/api/types';
import { RepoVisibility } from '~/lib/api/types';
import type { IconNames } from '~/components/atomic/Icon.vue';

const props = defineProps<{
  repo: Repo;
}>();

const apiClient = useApiClient();

// Load forges for favicon URLs
const forges = ref<Forge[]>([]);
const forge = computed(() => forges.value.find((f) => f.id === props.repo.forge_id) || null);

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

function getForgeUrl(forge: Forge): string {
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

const lastPipeline = computed(() => props.repo.last_pipeline);
const { startDate, shortMessage } = usePipeline(lastPipeline);
</script>

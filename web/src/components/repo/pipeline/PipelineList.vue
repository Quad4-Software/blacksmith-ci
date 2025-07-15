<template>
  <div v-if="pipelines || infiniteScroll" class="space-y-4">
    <PipelineItem
      v-for="pipeline in displayPipelines"
      :key="pipeline.id"
      :to="{
        name: 'repo-pipeline',
        params: { pipelineId: pipeline.number },
      }"
      :pipeline="pipeline"
    />

    <!-- Loading indicator for infinite scroll -->
    <div v-if="infiniteScroll && loading" class="flex justify-center items-center py-8">
      <div class="w-8 h-8">
        <PipelineProgressSpinner />
      </div>
      <span class="ml-3 text-wp-text-100">{{ $t('repo.pipeline.loading') }}</span>
    </div>

    <!-- Scroll trigger element -->
    <div v-if="infiniteScroll && hasMore" ref="scrollTrigger" class="h-4" />

    <Panel v-if="displayPipelines.length === 0 && !loading">
      <span class="text-wp-text-100">{{ $t('repo.pipeline.no_pipelines') }}</span>
    </Panel>
  </div>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue';

import Panel from '~/components/layout/Panel.vue';
import PipelineItem from '~/components/repo/pipeline/PipelineItem.vue';
import PipelineProgressSpinner from '~/components/repo/pipeline/PipelineProgressSpinner.vue';
import { usePagination } from '~/compositions/usePaginate';
import useApiClient from '~/compositions/useApiClient';
import useNotifications from '~/compositions/useNotifications';
import type { Pipeline, Repo } from '~/lib/api/types';
import { usePipelineStore } from '~/store/pipelines';

const props = defineProps<{
  pipelines?: Pipeline[] | undefined;
  repo?: Repo;
  infiniteScroll?: boolean;
}>();

const apiClient = useApiClient();
const pipelineStore = usePipelineStore();
const notifications = useNotifications();
const scrollTrigger = ref<HTMLElement | null>(null);

// Use infinite scroll if enabled and repo is provided
const infiniteScrollData =
  props.infiniteScroll && props.repo
    ? usePagination<Pipeline>(
        async (page) => {
          try {
            const pipelines = await apiClient.getPipelineList(props.repo!.id, { page, perPage: 25 });
            // Store pipelines in the store for consistency
            pipelines.forEach((pipeline) => {
              pipelineStore.setPipeline(props.repo!.id, pipeline);
            });
            return pipelines;
          } catch (error) {
            notifications.notify({ title: 'Failed to load pipelines', type: 'error' });
            console.error('Error loading pipelines:', error);
            return [];
          }
        },
        () => true,
        { scrollElement: scrollTrigger, pageSize: 25 },
      )
    : null;

const { data, loading, hasMore } = infiniteScrollData || { data: ref([]), loading: ref(false), hasMore: ref(false) };

// Display either provided pipelines or infinite scroll data
const displayPipelines = computed(() => {
  if (props.infiniteScroll && infiniteScrollData) {
    return data.value;
  }
  return props.pipelines || [];
});
</script>

<template>
  <Sheet :open="isOpen" @update:open="(open) => !open && close()">
    <SheetContent side="right" class="w-full sm:max-w-xs p-0 gap-0">
      <SheetHeader class="px-4 py-3 border-b border-border gap-0">
        <SheetTitle>{{ $t('pipeline_feed') }}</SheetTitle>
        <SheetDescription class="sr-only">{{ $t('pipeline_feed') }}</SheetDescription>
      </SheetHeader>
      <div class="flex flex-col overflow-y-auto h-[calc(100vh-4rem)]">
        <!-- Active pipelines -->
        <router-link
          v-for="pipeline in activePipelines"
          :key="pipeline.id"
          :to="{
            name: 'repo-pipeline',
            params: { repoId: pipeline.repo_id, pipelineId: pipeline.number },
          }"
          class="flex w-full border-b border-border px-4 py-3 hover:bg-accent transition-colors"
          @click="close"
        >
          <PipelineFeedItem :pipeline="pipeline" />
        </router-link>
        <!-- Separator between active and completed -->
        <div
          v-if="activePipelines.length > 0 && completedPipelines.length > 0"
          class="border-b-2 border-border bg-muted/30"
        />
        <!-- Completed pipelines -->
        <router-link
          v-for="pipeline in completedPipelines"
          :key="pipeline.id"
          :to="{
            name: 'repo-pipeline',
            params: { repoId: pipeline.repo_id, pipelineId: pipeline.number },
          }"
          class="flex w-full border-b border-border px-4 py-3 hover:bg-accent transition-colors"
          @click="close"
        >
          <PipelineFeedItem :pipeline="pipeline" />
        </router-link>
        <div v-if="sortedPipelines.length === 0" class="flex items-center justify-center h-32">
          <span class="text-sm text-muted-foreground">{{ $t('repo.pipeline.no_pipelines') }}</span>
        </div>
      </div>
    </SheetContent>
  </Sheet>
</template>

<script lang="ts" setup>
import { computed, watch } from 'vue';
import { useRoute } from 'vue-router';

import PipelineFeedItem from '~/components/pipeline-feed/PipelineFeedItem.vue';
import { Sheet, SheetContent } from '~/components/ui/sheet';
import SheetDescription from '~/components/ui/sheet/SheetDescription.vue';
import SheetHeader from '~/components/ui/sheet/SheetHeader.vue';
import SheetTitle from '~/components/ui/sheet/SheetTitle.vue';
import usePipelineFeed from '~/compositions/usePipelineFeed';

const route = useRoute();
const pipelineFeed = usePipelineFeed();
const { close, isOpen, sortedPipelines } = pipelineFeed;

// Close sidebar when route changes (handles mobile navigation)
watch(
  () => route.fullPath,
  () => {
    if (isOpen.value) {
      close();
    }
  },
);

const activePipelines = computed(() =>
  sortedPipelines.value.filter((p) => ['pending', 'running', 'started'].includes(p.status)),
);

const completedPipelines = computed(() =>
  sortedPipelines.value.filter((p) => !['pending', 'running', 'started'].includes(p.status)),
);
</script>

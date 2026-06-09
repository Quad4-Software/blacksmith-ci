<template>
  <Sheet :open="isOpen" @update:open="(open) => !open && close()">
    <SheetContent side="right" class="w-full sm:max-w-xs p-0 gap-0">
      <SheetHeader class="px-4 py-3 border-b border-border gap-0">
        <SheetTitle>{{ $t('pipeline_feed') }}</SheetTitle>
        <SheetDescription class="sr-only">{{ $t('pipeline_feed') }}</SheetDescription>
      </SheetHeader>
      <div class="flex flex-col overflow-y-auto h-[calc(100vh-4rem)]">
        <!-- Pending workflow approvals -->
        <template v-if="pendingWorkflowApprovals.length > 0">
          <div
            class="px-4 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground bg-muted/40 border-b border-border"
          >
            {{ $t('navbar.approvals.section_title') }}
          </div>
          <router-link
            v-for="item in pendingWorkflowApprovals"
            :key="item.workflow.id"
            :to="{
              name: 'repo-pipeline',
              params: { repoId: item.repo_id, pipelineId: item.pipeline_number },
            }"
            class="flex w-full border-b border-border px-4 py-3 hover:bg-accent transition-colors"
            @click="close"
          >
            <div class="flex w-full gap-3 items-start text-foreground">
              <Icon name="lucide:lock" class="size-4 shrink-0 mt-1 text-yellow-500" />
              <div class="flex min-w-0 flex-col flex-1">
                <span class="font-medium text-foreground truncate" :title="item.workflow.name">
                  {{ item.workflow.name }}
                </span>
                <span class="text-xs text-muted-foreground truncate" :title="item.repo_full_name">
                  {{ item.repo_full_name }}
                </span>
                <span class="text-xs text-muted-foreground tabular-nums">
                  {{ $t('navbar.approvals.pipeline_ref', { number: item.pipeline_number }) }}
                </span>
              </div>
            </div>
          </router-link>
        </template>
        <!-- Separator between approvals and pipelines -->
        <div
          v-if="pendingWorkflowApprovals.length > 0 && sortedPipelines.length > 0"
          class="border-b-2 border-border bg-muted/30"
        />
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
        <div
          v-if="sortedPipelines.length === 0 && pendingWorkflowApprovals.length === 0"
          class="flex items-center justify-center h-32"
        >
          <span class="text-sm text-muted-foreground">{{ $t('repo.pipeline.no_pipelines') }}</span>
        </div>
      </div>
    </SheetContent>
  </Sheet>
</template>

<script lang="ts" setup>
import { computed, watch } from 'vue';
import { useRoute } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
import PipelineFeedItem from '~/components/pipeline-feed/PipelineFeedItem.vue';
import { Sheet, SheetContent } from '~/components/ui/sheet';
import SheetDescription from '~/components/ui/sheet/SheetDescription.vue';
import SheetHeader from '~/components/ui/sheet/SheetHeader.vue';
import SheetTitle from '~/components/ui/sheet/SheetTitle.vue';
import usePipelineFeed from '~/compositions/usePipelineFeed';

const route = useRoute();
const pipelineFeed = usePipelineFeed();
const { close, isOpen, sortedPipelines, pendingWorkflowApprovals } = pipelineFeed;

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

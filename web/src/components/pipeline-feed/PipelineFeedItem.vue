<template>
  <div v-if="pipeline" class="flex w-full gap-3 items-start text-foreground">
    <PipelineStatusIcon :status="pipeline.status" class="w-4 h-4 shrink-0 mt-1" />
    <div class="flex min-w-0 flex-col flex-1">
      <router-link
        :to="{
          name: 'repo',
          params: { repoId: pipeline.repo_id },
        }"
        class="font-medium hover:underline text-foreground mb-1"
        @click.stop
      >
        <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
        <span class="text-muted-foreground">{{ repo?.owner }}</span
        ><span class="text-muted-foreground">/</span><span>{{ repo?.name }}</span>
        <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
      </router-link>
      <span
        class="overflow-hidden text-ellipsis whitespace-nowrap text-sm text-muted-foreground mb-2"
        :title="message"
        >{{ shortMessage }}</span
      >
      <!-- Event type row with action icons -->
      <div class="flex items-center gap-2 text-xs text-muted-foreground mb-2">
        <div class="flex items-center gap-1.5 min-w-0">
          <Icon v-if="pipeline.event === 'pull_request'" name="pull-request" class="w-4 h-4 shrink-0" />
          <Icon
            v-else-if="pipeline.event === 'pull_request_closed'"
            name="pull-request-closed"
            class="w-4 h-4 shrink-0"
          />
          <Icon
            v-else-if="pipeline.event === 'pull_request_edited'"
            name="pull-request-edited"
            class="w-4 h-4 shrink-0"
          />
          <Icon v-else-if="pipeline.event === 'deployment'" name="deployment" class="w-4 h-4 shrink-0" />
          <Icon
            v-else-if="pipeline.event === 'tag' || pipeline.event === 'release'"
            name="tag"
            class="w-4 h-4 shrink-0"
          />
          <Icon v-else-if="pipeline.event === 'cron'" name="cron" class="w-4 h-4 shrink-0" />
          <Icon v-else-if="pipeline.event === 'manual'" name="manual-pipeline" class="w-4 h-4 shrink-0" />
          <Icon v-else name="branch" class="w-4 h-4 shrink-0" />
          <span class="truncate">{{ prettyRef }}</span>
        </div>
        <!-- Action icons -->
        <div class="flex items-center gap-1 ml-auto shrink-0">
          <button
            v-if="isActive"
            class="p-1 rounded hover:bg-destructive/10 text-destructive transition-colors"
            :disabled="isCanceling"
            :title="$t('repo.pipeline.actions.cancel')"
            @click.stop.prevent="cancelPipeline"
          >
            <Icon v-if="isCanceling" name="spinner" class="w-4 h-4 animate-spin" />
            <Icon v-else name="lucide:square" class="w-4 h-4" />
          </button>
          <button
            v-else
            class="p-1 rounded hover:bg-primary/10 text-primary transition-colors"
            :disabled="isRestarting"
            :title="$t('repo.pipeline.actions.restart')"
            @click.stop.prevent="restartPipeline"
          >
            <Icon v-if="isRestarting" name="spinner" class="w-4 h-4 animate-spin" />
            <Icon v-else name="lucide:refresh-cw" class="w-4 h-4" />
          </button>
        </div>
      </div>
      <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
        <div class="flex items-center gap-1.5" :title="created">
          <Icon name="startDate" class="w-4 h-4" />
          <span>{{ startDate }}</span>
        </div>
        <div class="flex items-center gap-1.5">
          <Icon name="duration" class="w-4 h-4" />
          <span>{{ duration }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed, toRef } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useNotifications from '~/compositions/useNotifications';
import usePipeline from '~/compositions/usePipeline';
import type { PipelineFeed } from '~/lib/api/types';
import { usePipelineStore } from '~/store/pipelines';
import { useRepoStore } from '~/store/repos';

const props = defineProps<{
  pipeline: PipelineFeed;
}>();

const repoStore = useRepoStore();
const pipelineStore = usePipelineStore();
const apiClient = useApiClient();
const notifications = useNotifications();
const { t } = useI18n();

const pipeline = toRef(props, 'pipeline');
const repo = repoStore.getRepo(computed(() => pipeline.value.repo_id));

const { startDate, duration, shortMessage, message, prettyRef, created } = usePipeline(pipeline);

const isActive = computed(() => ['pending', 'running', 'started'].includes(pipeline.value.status));

const { doSubmit: cancelPipelineAction, isLoading: isCanceling } = useAsyncAction(async () => {
  await apiClient.cancelPipeline(pipeline.value.repo_id, pipeline.value.number);
  notifications.notify({
    title: t('notify_title_success'),
    text: t('repo.pipeline.actions.notify_text_cancel_success'),
    type: 'success',
  });
  await pipelineStore.loadPipelineFeed();
});

const { doSubmit: restartPipelineAction, isLoading: isRestarting } = useAsyncAction(async () => {
  await apiClient.restartPipeline(pipeline.value.repo_id, pipeline.value.number.toString(), {
    fork: true,
  });
  notifications.notify({
    title: t('notify_title_success'),
    text: t('repo.pipeline.actions.notify_text_restart_success'),
    type: 'success',
  });
  await pipelineStore.loadPipelineFeed();
});

async function cancelPipeline() {
  await cancelPipelineAction();
}

async function restartPipeline() {
  await restartPipelineAction();
}
</script>

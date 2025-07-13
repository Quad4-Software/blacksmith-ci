<!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
<template>
  <Scaffold
    v-if="pipeline && repo"
    enable-tabs
    :go-back="goBack"
    :fluid-content="route.name === 'repo-pipeline' || route.name === 'repo-pipeline-graph'"
    full-width-header
  >
    <template #title>
      <span>
        <router-link
          :to="{ name: 'org', params: { orgId: repo.org_id } }"
          class="hover:underline hover:decoration-wp-custom-highlight-100"
          >{{ repo.owner }}</router-link
        >
        / <router-link :to="{ name: 'repo' }" class="hover:underline">{{ repo.name }}</router-link>
      </span>
    </template>
    <template #headerActions>
      <div class="flex w-full items-center justify-between gap-2 px-1 md:px-2">
        <div class="flex min-w-0 content-start gap-2">
          <PipelineStatusIcon :status="pipeline.status" class="flex shrink-0" />
          <span class="shrink-0 text-center">{{ $t('repo.pipeline.pipeline', { pipelineId }) }}</span>
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          <span class="hidden md:inline-block">-</span>
          <span class="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap" :title="message">{{
            shortMessage
          }}</span>
        </div>
        <template v-if="repoPermissions!.push && pipeline.status !== 'blocked'">
          <div class="flex content-start gap-x-2">
            <Button
              v-if="pipeline.status === 'pending' || pipeline.status === 'running'"
              class="shrink-0 py-0!"
              :text="$t('repo.pipeline.actions.cancel')"
              :is-loading="isCancelingPipeline"
              @click="cancelPipeline"
            />
            <Button
              class="shrink-0 py-0!"
              :text="$t('repo.pipeline.actions.restart')"
              :is-loading="isRestartingPipeline"
              @click="restartPipeline"
            />
            <Button
              v-if="pipeline.status === 'success' && repo.allow_deploy"
              class="shrink-0 py-0!"
              :text="$t('repo.pipeline.actions.deploy')"
              @click="showDeployPipelinePopup = true"
            />
            <DeployPipelinePopup
              :pipeline-number="pipelineId"
              :open="showDeployPipelinePopup"
              @close="showDeployPipelinePopup = false"
            />
          </div>
        </template>
      </div>
    </template>
    <template #tabActions>
      <div class="flex flex-wrap gap-4 pl-2 md:pl-0 md:flex-nowrap">
        <div class="flex shrink-0 items-center gap-2" :title="$t('repo.pipeline.created', { created })">
          <Icon name="startDate" size="1rem" /> <span>{{ startDate }}</span>
        </div>

        <div class="flex shrink-0 items-center gap-2" :title="$t('repo.pipeline.duration')">
          <Icon name="duration" size="1rem" /> <span>{{ duration }}</span>
        </div>
      </div>
    </template>
    <Tab icon="tasks" icon-size="1rem" :to="{ name: 'repo-pipeline' }" :title="$t('repo.pipeline.tasks')" />
    <Tab
      v-if="pipeline.errors && pipeline.errors.length > 0"
      :to="{ name: 'repo-pipeline-errors' }"
      icon="alert"
      :title="pipeline.errors.some((e) => !e.is_warning) ? $t('repo.pipeline.errors') : $t('repo.pipeline.warnings')"
      :count="pipeline.errors?.length"
      :icon-class="pipeline.errors.some((e) => !e.is_warning) ? 'text-wp-error-100' : 'text-wp-state-warn-100'"
    />
    <Tab
      icon="file-cog-outline"
      icon-size="1rem"
      :to="{ name: 'repo-pipeline-config' }"
      :title="$t('repo.pipeline.config')"
    />
    <Tab icon="graph" icon-size="1rem" :to="{ name: 'repo-pipeline-graph' }" :title="$t('repo.pipeline.graph.title')" />
    <Tab
      v-if="pipeline.changed_files && pipeline.changed_files.length > 0"
      :to="{ name: 'repo-pipeline-changed-files' }"
      icon="file-edit-outline"
      icon-size="1rem"
      :title="$t('repo.pipeline.files')"
      :count="pipeline.changed_files?.length"
    />
    <Tab
      v-if="repoPermissions && repoPermissions.push"
      icon="bug-outline"
      icon-size="1rem"
      :to="{ name: 'repo-pipeline-debug' }"
      :title="$t('repo.pipeline.debug.title')"
    />
    <router-view />
  </Scaffold>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, ref, toRef, watch } from 'vue';
import type { Ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';

import Button from '~/components/atomic/Button.vue';
import Icon from '~/components/atomic/Icon.vue';
import DeployPipelinePopup from '~/components/layout/popups/DeployPipelinePopup.vue';
import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import Tab from '~/components/layout/scaffold/Tab.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useFavicon } from '~/compositions/useFavicon';
import { provide, requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import usePipeline from '~/compositions/usePipeline';
import { useRouteBack } from '~/compositions/useRouteBack';
import { usePipelineStore } from '~/store/pipelines';
import type { Pipeline, PipelineConfig } from '~/lib/api/types';

const props = defineProps<{
  repoId: string;
  pipelineId: string;
}>();

const apiClient = useApiClient();
const route = useRoute();
const router = useRouter();
const notifications = useNotifications();
const favicon = useFavicon();
const i18n = useI18n();

const pipelineStore = usePipelineStore();
const pipelineId = toRef(props, 'pipelineId');
const _repoId = toRef(props, 'repoId');
const repositoryId = computed(() => Number.parseInt(_repoId.value, 10));
const repo = requiredInject('repo');
const repoPermissions = requiredInject('repo-permissions');

const pipeline = pipelineStore.getPipeline(repositoryId, pipelineId);
const { startDate, duration, created, message, shortMessage } = usePipeline(pipeline);
provide('pipeline', pipeline as Ref<Pipeline>); // can't be undefined because of v-if in template

const pipelineConfigs = ref<PipelineConfig[]>();
provide('pipeline-configs', pipelineConfigs);

watch(
  pipeline,
  () => {
    favicon.updateStatus(pipeline.value?.status);
  },
  { immediate: true },
);

const showDeployPipelinePopup = ref(false);

async function loadPipeline(): Promise<void> {
  if (!repo) {
    throw new Error('Unexpected: Repo is undefined');
  }

  await pipelineStore.loadPipeline(repo.value.id, Number.parseInt(pipelineId.value, 10));

  if (!pipeline.value?.number) {
    throw new Error('Unexpected: Pipeline number not found');
  }

  pipelineConfigs.value = await apiClient.getPipelineConfig(repo.value.id, pipeline.value.number);
}

const { doSubmit: cancelPipeline, isLoading: isCancelingPipeline } = useAsyncAction(async () => {
  if (!repo) {
    throw new Error('Unexpected: Repo is undefined');
  }

  if (!pipeline.value?.number) {
    throw new Error('Unexpected: Pipeline number not found');
  }

  await apiClient.cancelPipeline(repo.value.id, pipeline.value.number);
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('repo.pipeline.actions.notify_text_cancel_success'),
    type: 'success',
    duration: 6000,
  });
});

const { doSubmit: restartPipeline, isLoading: isRestartingPipeline } = useAsyncAction(async () => {
  if (!repo) {
    throw new Error('Unexpected: Repo is undefined');
  }

  const newPipeline = await apiClient.restartPipeline(repo.value.id, pipelineId.value, {
    fork: true,
  });
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('repo.pipeline.actions.notify_text_restart_success'),
    type: 'success',
    duration: 6000,
  });
  await router.push({
    name: 'repo-pipeline',
    params: { pipelineId: newPipeline.number },
  });
});

onMounted(loadPipeline);
watch([repositoryId, pipelineId], loadPipeline);
onBeforeUnmount(() => {
  favicon.updateStatus('default');
});

const goBack = useRouteBack({ name: 'repo' });
</script>

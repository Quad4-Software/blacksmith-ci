<template>
  <div
    class="flex items-center justify-center"
    :title="title || $t('repo.pipeline.status.status', { status: statusDescriptions[status] })"
  >
    <Icon
      v-if="service || !(status === 'running' || status === 'started')"
      :name="service ? 'mdi-wind-power-outline' : `status-${status}`"
      :size="size"
      :class="{
        'text-destructive': pipelineStatusColors[status] === 'red', // error, declined
        'text-muted-foreground': pipelineStatusColors[status] === 'gray', // blocked, killed, skipped
        'text-green-500': pipelineStatusColors[status] === 'green', // success
        'text-primary': pipelineStatusColors[status] === 'orange', // running, started
        'text-yellow-500': pipelineStatusColors[status] === 'yellow', // pending
      }"
    />
    <div v-else class="flex items-center justify-center" :style="{ width: size, height: size }">
      <PipelineProgressSpinner />
    </div>
  </div>
</template>

<script lang="ts" setup>
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import PipelineProgressSpinner from './PipelineProgressSpinner.vue';
import type { PipelineStatus } from '~/lib/api/types';

import { pipelineStatusColors } from './pipeline-status';

withDefaults(
  defineProps<{
    status: PipelineStatus;
    service?: boolean;
    title?: string;
    size?: string;
  }>(),
  {
    size: '1.5rem',
  },
);

const { t } = useI18n();

const statusDescriptions = {
  blocked: t('repo.pipeline.status.blocked'),
  declined: t('repo.pipeline.status.declined'),
  error: t('repo.pipeline.status.error'),
  failure: t('repo.pipeline.status.failure'),
  killed: t('repo.pipeline.status.killed'),
  pending: t('repo.pipeline.status.pending'),
  running: t('repo.pipeline.status.running'),
  skipped: t('repo.pipeline.status.skipped'),
  started: t('repo.pipeline.status.started'),
  success: t('repo.pipeline.status.success'),
} satisfies {
  // eslint-disable-next-line no-unused-vars
  [_ in PipelineStatus]: string;
};
</script>

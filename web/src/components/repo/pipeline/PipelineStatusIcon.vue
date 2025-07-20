<template>
  <div
    class="flex items-center justify-center"
    :title="$t('repo.pipeline.status.status', { status: statusDescriptions[status] })"
  >
    <Icon
      v-if="service || !(status === 'running' || status === 'started')"
      :name="service ? 'mdi-wind-power-outline' : `status-${status}`"
      size="1.5rem"
      :class="{
        'text-wp-error-100': pipelineStatusColors[status] === 'red', // error, declined
        'text-wp-text-alt-100': pipelineStatusColors[status] === 'gray', // blocked, killed, skipped
        'text-wp-state-ok-100': pipelineStatusColors[status] === 'green', // success
        'text-wp-custom-highlight-100': pipelineStatusColors[status] === 'orange', // running, started
        'text-wp-state-pending-100': pipelineStatusColors[status] === 'yellow', // pending
      }"
    />
    <div v-else class="w-5 h-5">
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

defineProps<{
  status: PipelineStatus;
  service?: boolean;
}>();

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

<template>
  <button
    class="flex items-center justify-center gap-2 px-2 py-1.5 mr-2 rounded-md transition-colors"
    :class="[totalCount === 0 ? 'text-muted-foreground hover:bg-accent' : 'bg-accent/50 hover:bg-accent']"
    :title="tooltipText"
    @click="toggle"
  >
    <!-- Status indicators -->
    <div class="flex items-center gap-3">
      <!-- Running indicator -->
      <div v-if="runningCount > 0" class="flex items-center gap-1.5">
        <Spinner class="size-4 text-primary" />
        <span class="text-xs font-semibold leading-none text-primary tabular-nums">
          {{ runningCount }}
        </span>
      </div>

      <!-- Pending indicator -->
      <div v-if="pendingCount > 0" class="flex items-center gap-1.5">
        <Icon name="status-pending" class="size-4 text-yellow-500" />
        <span class="text-xs font-semibold leading-none text-yellow-600 tabular-nums">
          {{ pendingCount }}
        </span>
      </div>

      <!-- Awaiting pipeline-level approval indicator -->
      <div v-if="blockedCount > 0" class="flex items-center gap-1.5">
        <Icon name="status-blocked" class="size-4 text-muted-foreground" />
        <span class="text-xs font-semibold leading-none text-muted-foreground tabular-nums">
          {{ blockedCount }}
        </span>
      </div>

      <!-- Awaiting workflow-level approval indicator -->
      <div v-if="workflowApprovalCount > 0" class="flex items-center gap-1.5">
        <Icon name="lucide:lock" class="size-4 text-yellow-500" />
        <span class="text-xs font-semibold leading-none text-yellow-600 tabular-nums">
          {{ workflowApprovalCount }}
        </span>
      </div>

      <!-- Default icon when no pipelines -->
      <Icon v-if="totalCount === 0" name="pipelines" class="size-4" />
    </div>
  </button>
</template>

<script lang="ts" setup>
import { computed, onMounted, toRef } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import { Spinner } from '~/components/ui/spinner';
import usePipelineFeed from '~/compositions/usePipelineFeed';

const { t } = useI18n();
const pipelineFeed = usePipelineFeed();
const activePipelines = toRef(pipelineFeed, 'activePipelines');
const pendingWorkflowApprovals = toRef(pipelineFeed, 'pendingWorkflowApprovals');
const { toggle } = pipelineFeed;

const runningCount = computed(
  () => activePipelines.value.filter((p) => p.status === 'running' || p.status === 'started').length,
);

const pendingCount = computed(() => activePipelines.value.filter((p) => p.status === 'pending').length);

const blockedCount = computed(() => activePipelines.value.filter((p) => p.status === 'blocked').length);

// Workflow-level approval gates the current user can act on. The endpoint
// already filters by approver permission, so the array length is the badge
// count. State is owned by usePipelineFeed so the sidebar can render the list.
const workflowApprovalCount = computed(() => pendingWorkflowApprovals.value.length);

const totalCount = computed(
  () => runningCount.value + pendingCount.value + blockedCount.value + workflowApprovalCount.value,
);

const tooltipText = computed(() => {
  if (totalCount.value === 0) {
    return t('pipeline_feed');
  }

  const parts: string[] = [];
  if (runningCount.value > 0) {
    parts.push(`${runningCount.value} ${t('repo.pipeline.status.running').toLowerCase()}`);
  }
  if (pendingCount.value > 0) {
    parts.push(`${pendingCount.value} ${t('repo.pipeline.status.pending').toLowerCase()}`);
  }
  if (blockedCount.value > 0) {
    parts.push(`${blockedCount.value} ${t('navbar.approvals.pipelines').toLowerCase()}`);
  }
  if (workflowApprovalCount.value > 0) {
    parts.push(`${workflowApprovalCount.value} ${t('navbar.approvals.workflows').toLowerCase()}`);
  }
  return `${t('pipeline_feed')}: ${parts.join(', ')}`;
});

onMounted(async () => {
  await pipelineFeed.load();
});
</script>

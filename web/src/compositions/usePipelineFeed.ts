import { computed, ref, toRef } from 'vue';

import useApiClient from '~/compositions/useApiClient';
import useUserConfig from '~/compositions/useUserConfig';
import type { PendingApprovalItem } from '~/lib/api/types';
import { useAuthStore } from '~/store/auth';
import { usePipelineStore } from '~/store/pipelines';

const userConfig = useUserConfig();

// Module-scoped state so every caller (navbar badge + sidebar list) sees the
// same array without each invocation triggering its own fetch.
const pendingWorkflowApprovals = ref<PendingApprovalItem[]>([]);

function toggle() {
  userConfig.setUserConfig('isPipelineFeedOpen', !userConfig.userConfig.value.isPipelineFeedOpen);
}

function close() {
  userConfig.setUserConfig('isPipelineFeedOpen', false);
}

export default () => {
  const pipelineStore = usePipelineStore();
  const authStore = useAuthStore();
  const apiClient = useApiClient();

  // Reference the store getter inside the computed so it stays reactive to
  // auth changes; destructuring would capture a plain boolean snapshot.
  const isOpen = computed(() => userConfig.userConfig.value.isPipelineFeedOpen && authStore.isAuthenticated);

  const sortedPipelines = toRef(pipelineStore, 'pipelineFeed');
  const activePipelines = toRef(pipelineStore, 'activePipelines');

  async function loadWorkflowApprovals() {
    if (!authStore.isAuthenticated) {
      pendingWorkflowApprovals.value = [];
      return;
    }
    try {
      const { approvals } = await apiClient.getUserPendingApprovals({ silent: true });
      pendingWorkflowApprovals.value = approvals ?? [];
    } catch (error) {
      console.error(error);
    }
  }

  async function load() {
    await Promise.all([pipelineStore.loadPipelineFeed(), loadWorkflowApprovals()]);
  }

  return {
    toggle,
    close,
    isOpen,
    sortedPipelines,
    activePipelines,
    pendingWorkflowApprovals,
    loadWorkflowApprovals,
    load,
  };
};

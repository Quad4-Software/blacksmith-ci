<template>
  <div class="flex flex-col h-full p-6">
    <div v-if="!pipeline" class="flex-1 flex items-center justify-center">
      <div class="text-center">
        <p class="text-lg text-muted-foreground">{{ t('repo.pipeline.loading') }}</p>
      </div>
    </div>
    <div
      v-else-if="!pipeline.workflows || pipeline.workflows.length === 0"
      class="flex-1 flex items-center justify-center"
    >
      <div class="text-center">
        <p class="text-lg text-muted-foreground">{{ t('repo.pipeline.no_pipeline_steps') }}</p>
      </div>
    </div>
    <div v-else class="flex-1 min-h-0 border border-border rounded-lg overflow-hidden bg-card relative">
      <!-- Loading overlay -->
      <div
        v-if="isDAGLoading"
        class="absolute inset-0 z-50 bg-card/80 backdrop-blur-sm flex items-center justify-center"
      >
        <div class="flex flex-col items-center gap-4">
          <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
          <p class="text-sm text-muted-foreground">{{ t('repo.pipeline.graph.loading') }}</p>
        </div>
      </div>

      <!-- Skeleton loader -->
      <div v-if="showSkeleton" class="absolute inset-0 p-4 z-40">
        <div class="h-full flex flex-col gap-4">
          <!-- Search and filters skeleton -->
          <div class="bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 p-3 rounded-t-lg">
            <div class="flex justify-between items-center">
              <div class="flex gap-3 flex-1">
                <div class="h-8 bg-gray-200 dark:bg-gray-700 rounded w-64 animate-pulse"></div>
                <div class="h-8 bg-gray-200 dark:bg-gray-700 rounded w-32 animate-pulse"></div>
                <div class="h-8 bg-gray-200 dark:bg-gray-700 rounded w-32 animate-pulse"></div>
              </div>
            </div>
          </div>

          <!-- Progress bar skeleton -->
          <div class="px-4 py-2 bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
            <div class="h-2 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
          </div>

          <!-- DAG skeleton -->
          <div class="flex-1 p-8 relative">
            <!-- Simulated workflow groups -->
            <div
              class="absolute top-8 left-8 w-80 h-48 border-2 border-dashed border-gray-200 dark:border-gray-700 rounded-lg animate-pulse"
            >
              <div
                class="absolute top-4 left-4 bg-gray-200 dark:bg-gray-700 rounded-full px-3 py-1 text-sm h-6 w-32"
              ></div>
            </div>
            <div
              class="absolute top-8 left-96 w-80 h-48 border-2 border-dashed border-gray-200 dark:border-gray-700 rounded-lg animate-pulse"
            >
              <div
                class="absolute top-4 left-4 bg-gray-200 dark:bg-gray-700 rounded-full px-3 py-1 text-sm h-6 w-32"
              ></div>
            </div>

            <!-- Simulated step nodes -->
            <div
              class="absolute top-20 left-16 w-32 h-20 bg-gray-100 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg animate-pulse shadow-sm"
            ></div>
            <div
              class="absolute top-44 left-16 w-32 h-20 bg-gray-100 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg animate-pulse shadow-sm"
            ></div>
            <div
              class="absolute top-20 left-104 w-32 h-20 bg-gray-100 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg animate-pulse shadow-sm"
            ></div>
            <div
              class="absolute top-44 left-104 w-32 h-20 bg-gray-100 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg animate-pulse shadow-sm"
            ></div>

            <!-- Simulated edges -->
            <div class="absolute top-30 left-48 w-16 h-0.5 bg-gray-200 dark:bg-gray-700 animate-pulse"></div>
            <div class="absolute top-54 left-48 w-16 h-0.5 bg-gray-200 dark:bg-gray-700 animate-pulse"></div>
          </div>
        </div>
      </div>

      <PipelineDAG
        :pipeline="pipeline"
        :selected-step-id="selectedStepId"
        :class="{ 'opacity-0': isDAGLoading || showSkeleton }"
        @update:selected-step-id="handleStepSelection"
        @dag-ready="handleDAGReady"
        @dag-loading="handleDAGLoading"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';
import PipelineDAG from '~/components/repo/pipeline/PipelineDAG.vue';
import { requiredInject } from '~/compositions/useInjectProvide';

const route = useRoute();
const router = useRouter();
const { t } = useI18n();

const pipeline = requiredInject('pipeline');

const selectedStepId = computed(() => {
  const stepId = route.params.stepId as string;
  return stepId ? Number.parseInt(stepId, 10) : null;
});

const handleStepSelection = (stepId: number) => {
  router.replace({
    name: 'repo-pipeline',
    params: { ...route.params, stepId: stepId.toString() },
  });
};

// Loading states
const isDAGLoading = ref(false);
const showSkeleton = ref(true);
const hasDAGLoaded = ref(false);

// Handle DAG loading events
const handleDAGLoading = (loading: boolean) => {
  isDAGLoading.value = loading;
};

const handleDAGReady = () => {
  isDAGLoading.value = false;
  hasDAGLoaded.value = true;

  // Hide skeleton after a short delay to ensure smooth transition
  setTimeout(() => {
    showSkeleton.value = false;
  }, 200);
};

// Watch pipeline changes to show skeleton for new data
watch(
  () => pipeline.value,
  (newPipeline, oldPipeline) => {
    // Show skeleton when pipeline changes (new data being loaded)
    if (newPipeline && oldPipeline && newPipeline.id !== oldPipeline.id) {
      showSkeleton.value = true;
      hasDAGLoaded.value = false;
    }
    // Show skeleton initially when pipeline becomes available
    else if (newPipeline && !hasDAGLoaded.value) {
      showSkeleton.value = true;
    }
  },
  { immediate: true },
);
</script>

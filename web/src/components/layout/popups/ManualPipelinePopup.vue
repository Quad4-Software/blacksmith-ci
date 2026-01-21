<template>
  <Dialog :open="open" @update:open="(val) => !val && $emit('close')">
    <DialogContent class="sm:w-fit sm:min-w-80 sm:max-w-2xl">
      <DialogHeader class="text-left">
        <DialogTitle>{{ $t('repo.manual_pipeline.title') }}</DialogTitle>
        <DialogDescription>
          {{ $t('repo.manual_pipeline.desc') }}
        </DialogDescription>
      </DialogHeader>

      <!-- Branch selector below header -->
      <div class="flex items-center gap-2 text-sm pb-2 border-b">
        <Select v-model="payload.branch" :disabled="loading">
          <SelectTrigger class="h-8 w-auto min-w-40">
            <SelectValue :placeholder="$t('repo.manual_pipeline.select_branch')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="branch in branches" :key="branch" :value="branch">
              {{ branch }}
              <span v-if="branch === repo.default_branch" class="ml-1 text-muted-foreground">{{
                $t('repo.manual_pipeline.default_branch_suffix')
              }}</span>
            </SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div v-if="loading" class="flex justify-center py-8">
        <Icon name="spinner" class="h-6 w-6 animate-spin" />
      </div>

      <form v-else class="space-y-4" @submit.prevent="triggerManualPipeline">
        <!-- Workflow selection -->
        <div class="space-y-2">
          <div v-if="workflowsLoading" class="flex justify-center py-8">
            <Icon name="spinner" class="h-5 w-5 animate-spin" />
          </div>
          <WorkflowSelector
            v-else
            :workflows="workflows"
            :selected-workflows="selectedWorkflows"
            :warnings="workflowWarnings"
            @update:selected-workflows="selectedWorkflows = $event"
            @quick-start="handleQuickStart"
          />
        </div>

        <!-- Additional variables (collapsible) -->
        <Collapsible v-model:open="showVariables" class="border rounded-md">
          <CollapsibleTrigger
            class="flex w-full items-center justify-between p-3 text-sm font-medium hover:bg-accent transition-colors"
          >
            <span>{{ $t('repo.manual_pipeline.variables.title') }}</span>
            <Icon :name="showVariables ? 'chevron-up' : 'chevron-down'" class="h-4 w-4" />
          </CollapsibleTrigger>
          <CollapsibleContent class="px-3 pb-3">
            <p class="text-sm text-muted-foreground mb-3">{{ $t('repo.manual_pipeline.variables.desc') }}</p>
            <KeyValueEditor
              v-model="variables"
              :key-placeholder="$t('repo.manual_pipeline.variables.name')"
              :value-placeholder="$t('repo.manual_pipeline.variables.value')"
              :delete-title="$t('repo.manual_pipeline.variables.delete')"
              @update:is-valid="isVariablesValid = $event"
            />
          </CollapsibleContent>
        </Collapsible>

        <DialogFooter class="sm:justify-start">
          <Button type="submit" :disabled="!isFormValid || isSubmitting">
            <Icon v-if="isSubmitting" name="spinner" class="mr-1 h-4 w-4 animate-spin" />
            <Icon v-else name="play" class="mr-1 h-4 w-4" />
            {{ $t('repo.manual_pipeline.trigger', selectedWorkflows.size) }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
import KeyValueEditor from '~/components/form/KeyValueEditor.vue';
import WorkflowSelector from '~/components/form/WorkflowSelector.vue';
import { Button } from '~/components/ui/button';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '~/components/ui/collapsible';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import useApiClient from '~/compositions/useApiClient';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import { PipelineFilteredError } from '~/lib/api/client';
import { usePaginate } from '~/compositions/usePaginate';
import type { ManualWorkflow } from '~/lib/api/types';

const props = defineProps<{
  open: boolean;
}>();

const emit = defineEmits<{
  (event: 'close'): void;
}>();

const apiClient = useApiClient();
const notifications = useNotifications();
const i18n = useI18n();
const router = useRouter();

const repo = requiredInject('repo');

const branches = ref<string[]>([]);
const payload = ref<{ branch: string }>({
  branch: '',
});

const workflows = ref<ManualWorkflow[]>([]);
const selectedWorkflows = ref<Set<string>>(new Set());
const workflowWarnings = ref<string[]>([]);
const workflowsLoading = ref(false);

const loading = ref(false);
const isSubmitting = ref(false);
const hasFetchedBranches = ref(false);

// Variables for evaluate filter
const showVariables = ref(false);
const variables = ref<Record<string, string>>({});
const isVariablesValid = ref(true);

const isFormValid = computed(() => {
  return payload.value.branch !== '' && selectedWorkflows.value.size > 0 && isVariablesValid.value;
});

// Fetch branches when dialog opens
watch(
  () => props.open,
  async (isOpen) => {
    if (isOpen && !hasFetchedBranches.value) {
      loading.value = true;
      try {
        const data = await usePaginate((page) => apiClient.getRepoBranches(repo.value.id, { page }));
        // Sort branches: default branch first, then alphabetically
        const defaultBranch = repo.value.default_branch;
        branches.value = data.sort((a, b) => {
          if (a === defaultBranch) return -1;
          if (b === defaultBranch) return 1;
          return a.localeCompare(b);
        });
        // Set default branch
        payload.value.branch = defaultBranch || data[0] || '';
        hasFetchedBranches.value = true;
      } catch (error) {
        console.error('Failed to fetch branches:', error);
        notifications.notify({
          title: i18n.t('notify_title_error'),
          text: i18n.t('repo.manual_pipeline.fetch_branches_failed'),
          type: 'error',
        });
      } finally {
        loading.value = false;
      }
    }
  },
  { immediate: true },
);

// Fetch workflows when branch changes
watch(
  () => payload.value.branch,
  async (branch) => {
    if (!branch) {
      workflows.value = [];
      workflowWarnings.value = [];
      selectedWorkflows.value = new Set();
      return;
    }

    workflowsLoading.value = true;
    try {
      const response = await apiClient.getRepoManualWorkflows(repo.value.id, branch);
      workflows.value = response.workflows;
      workflowWarnings.value = response.warnings || [];
      // Reset selection when workflows change
      selectedWorkflows.value = new Set();
    } catch (error) {
      console.error('Failed to fetch workflows:', error);
      workflows.value = [];
      workflowWarnings.value = [];
      notifications.notify({
        title: i18n.t('notify_title_error'),
        text: i18n.t('repo.manual_pipeline.fetch_workflows_failed'),
        type: 'error',
      });
    } finally {
      workflowsLoading.value = false;
    }
  },
  { immediate: true },
);

async function triggerManualPipeline() {
  if (selectedWorkflows.value.size === 0) {
    return;
  }

  isSubmitting.value = true;

  try {
    const pipeline = await apiClient.createPipeline(repo.value.id, {
      branch: payload.value.branch,
      variables: variables.value,
      workflows: Array.from(selectedWorkflows.value),
    });

    emit('close');

    // Reset form for next use
    selectedWorkflows.value = new Set();
    variables.value = {};
    showVariables.value = false;

    await router.push({
      name: 'repo-pipeline',
      params: {
        pipelineId: pipeline.number,
      },
    });
  } catch (error: unknown) {
    emit('close');

    if (error instanceof PipelineFilteredError) {
      // All steps were filtered out by `when` conditions
      const helpUrl = 'https://crowci.dev/dev/usage/workflow-syntax/#when-step';
      notifications.notify({
        title: i18n.t('notify_title_error'),
        type: 'error',
        duration: 9000,
        text: `${i18n.t('repo.manual_pipeline.steps_filtered_out')} <a href='${helpUrl}' target='_blank' class='text-primary underline'>${i18n.t('repo.manual_pipeline.when_condition_help')}</a>`,
      });
    } else {
      // Show the actual error message from the server
      const errorMessage = error instanceof Error ? error.message : String(error);
      notifications.notify({
        title: i18n.t('notify_title_error'),
        type: 'error',
        duration: 9000,
        text: errorMessage,
      });
    }
    console.error('Failed to trigger pipeline:', error);
  } finally {
    isSubmitting.value = false;
  }
}

async function handleQuickStart(workflowName: string) {
  isSubmitting.value = true;

  try {
    const pipeline = await apiClient.createPipeline(repo.value.id, {
      branch: payload.value.branch,
      variables: variables.value,
      workflows: [workflowName],
    });

    emit('close');

    // Reset form for next use
    variables.value = {};
    showVariables.value = false;

    await router.push({
      name: 'repo-pipeline',
      params: {
        pipelineId: pipeline.number,
      },
    });
  } catch (error: unknown) {
    emit('close');

    if (error instanceof PipelineFilteredError) {
      notifications.notify({
        title: i18n.t('notify_title_error'),
        type: 'error',
        text: i18n.t('repo.manual_pipeline.steps_filtered_out'),
      });
    } else {
      // Show the actual error message from the server
      const errorMessage = error instanceof Error ? error.message : String(error);
      notifications.notify({
        title: i18n.t('notify_title_error'),
        type: 'error',
        duration: 9000,
        text: errorMessage,
      });
    }
    console.error('Failed to trigger pipeline:', error);
  } finally {
    isSubmitting.value = false;
  }
}
</script>

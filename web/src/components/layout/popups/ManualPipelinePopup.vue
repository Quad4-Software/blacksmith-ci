<template>
  <Dialog :open="open" @update:open="(val) => !val && $emit('close')">
    <!-- No `min-w` here: `w-full` already fills up to `sm:max-w-5xl`, while a min
         width would push the dialog past the viewport between 640px and 784px. -->
    <DialogContent class="sm:max-w-5xl max-h-[85vh] overflow-y-auto">
      <DialogHeader class="text-left">
        <DialogTitle>{{ $t('repo.manual_pipeline.title') }}</DialogTitle>
        <DialogDescription>
          {{ $t('repo.manual_pipeline.desc') }}
        </DialogDescription>
      </DialogHeader>

      <!-- Branch/tag selector below header -->
      <div class="flex items-center gap-2 text-sm pb-2 border-b">
        <Popover v-model:open="refPickerOpen">
          <PopoverTrigger as-child>
            <Button
              variant="outline"
              role="combobox"
              :aria-expanded="refPickerOpen"
              :disabled="loading"
              class="h-8 w-auto min-w-40 justify-between"
            >
              <span class="flex min-w-0 items-center gap-1.5">
                <Icon
                  v-if="selectedRef"
                  :name="selectedRef.type === 'tag' ? 'tag' : 'branch'"
                  class="h-3.5 w-3.5 shrink-0 opacity-70"
                />
                <span class="truncate">{{ selectedRef?.name || $t('repo.manual_pipeline.select_ref') }}</span>
              </span>
              <Icon name="ph:arrows-down-up" class="ml-2 h-4 w-4 shrink-0 opacity-50" />
            </Button>
          </PopoverTrigger>
          <PopoverContent
            class="w-auto min-w-[var(--reka-popover-trigger-width)] max-w-[min(24rem,90vw)] p-0"
            align="start"
          >
            <Command>
              <CommandInput v-model="refSearch" :placeholder="$t('repo.manual_pipeline.select_ref')" />
              <CommandList>
                <CommandEmpty>{{ $t('no_search_results') }}</CommandEmpty>
                <CommandGroup :heading="$t('repo.manual_pipeline.branches')">
                  <CommandItem
                    v-for="branch in branches"
                    :key="`branch:${branch}`"
                    :value="branch"
                    :title="branch"
                    class="cursor-pointer"
                    @select="selectRef('branch', branch)"
                  >
                    <Icon
                      name="ph:check"
                      class="mr-2 h-4 w-4"
                      :class="isSelected('branch', branch) ? 'opacity-100' : 'opacity-0'"
                    />
                    <span class="truncate">{{ branch }}</span>
                    <span v-if="branch === repo.default_branch" class="ml-1 shrink-0 text-muted-foreground">{{
                      $t('repo.manual_pipeline.default_branch_suffix')
                    }}</span>
                  </CommandItem>
                </CommandGroup>
                <!-- Omitted entirely when the forge cannot list tags, so the
                     picker degrades to branches only instead of erroring. -->
                <CommandGroup v-if="tags.length > 0" :heading="$t('repo.manual_pipeline.tags')">
                  <CommandItem
                    v-for="tag in tags"
                    :key="`tag:${tag}`"
                    :value="tag"
                    :title="tag"
                    class="cursor-pointer"
                    @select="selectRef('tag', tag)"
                  >
                    <Icon
                      name="ph:check"
                      class="mr-2 h-4 w-4"
                      :class="isSelected('tag', tag) ? 'opacity-100' : 'opacity-0'"
                    />
                    <span class="truncate">{{ tag }}</span>
                  </CommandItem>
                </CommandGroup>
              </CommandList>
            </Command>
          </PopoverContent>
        </Popover>
      </div>

      <div v-if="loading" class="flex justify-center items-center py-8 sm:h-[60vh]">
        <Icon name="spinner" class="h-6 w-6 animate-spin" />
      </div>

      <form v-else class="space-y-4" @submit.prevent="triggerManualPipeline">
        <!-- Fixed body height so the dialog doesn't resize when a workflow with many
             variables is selected; each column scrolls internally instead. -->
        <div class="flex flex-col sm:flex-row gap-4 sm:gap-6 sm:h-[60vh]">
          <!-- Left: workflow selection -->
          <div class="sm:w-2/5 sm:border-r sm:pr-4 sm:overflow-y-auto">
            <div
              v-if="workflowsLoading"
              class="flex flex-col items-center justify-center gap-3 px-4 py-8 text-center"
              role="status"
              aria-live="polite"
            >
              <Icon name="spinner" class="h-5 w-5 animate-spin text-primary" />
              <div class="space-y-1">
                <p class="text-sm font-medium">{{ $t('repo.manual_pipeline.parsing_workflows') }}</p>
                <p v-if="showExtendedWorkflowLoading" class="text-xs text-muted-foreground">
                  {{ workflowProgressText }}
                </p>
              </div>
              <Progress
                v-if="showExtendedWorkflowLoading && hasWorkflowProgressTotal"
                :model-value="workflowProgressPercent"
                class="h-1.5 max-w-48"
              />
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

          <!-- Right: variables (top divider on mobile where columns stack) -->
          <div class="flex-1 min-w-0 border-t pt-4 sm:border-t-0 sm:pt-0 sm:overflow-y-auto">
            <ManualVariablesPanel
              v-model="variableValues"
              :grouped="grouped"
              :has-selection="selectedWorkflows.size > 0"
            />
          </div>
        </div>

        <DialogFooter class="sm:justify-start">
          <Button type="submit" :disabled="!isFormValid || isSubmitting">
            <Icon v-if="isSubmitting" name="spinner" class="mr-1 h-4 w-4 animate-spin" />
            <Icon v-else name="play" class="mr-1 h-4 w-4" />
            {{ $t('repo.manual_pipeline.trigger') }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script lang="ts" setup>
import Icon from '~/components/atomic/Icon.vue';
import WorkflowSelector from '~/components/form/WorkflowSelector.vue';
import { getSelectedWorkflowDefault, groupManualVariables } from '~/components/layout/popups/manualVariableGroups';
import ManualVariablesPanel from '~/components/layout/popups/ManualVariablesPanel.vue';
import { Button } from '~/components/ui/button';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '~/components/ui/command';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import { Progress } from '~/components/ui/progress';
import useApiClient from '~/compositions/useApiClient';
import { requiredInject } from '~/compositions/useInjectProvide';
import { useLoadingOverlay } from '~/compositions/useLoadingOverlay';
import useNotifications from '~/compositions/useNotifications';
import { usePaginate } from '~/compositions/usePaginate';
import { PipelineFilteredError } from '~/lib/api/client';
import type { ManualVariable, ManualWorkflow, ManualWorkflowsProgress } from '~/lib/api/types';
import { computed, onScopeDispose, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

const props = defineProps<{
  open: boolean;
}>();

const emit = defineEmits<{
  (event: 'close'): void;
}>();

const apiClient = useApiClient();
const notifications = useNotifications();
const overlay = useLoadingOverlay();
const i18n = useI18n();
const router = useRouter();

const repo = requiredInject('repo');

type RefKind = 'branch' | 'tag';
// A branch and a tag may share a name, so the kind has to be tracked alongside it.
interface SelectedRef {
  type: RefKind;
  name: string;
}

const branches = ref<string[]>([]);
const tags = ref<string[]>([]);
const refPickerOpen = ref(false);
const refSearch = ref('');
const selectedRef = ref<SelectedRef | null>(null);

const workflows = ref<ManualWorkflow[]>([]);
const selectedWorkflows = ref<Set<string>>(new Set());
const workflowWarnings = ref<string[]>([]);
const workflowsLoading = ref(false);
const workflowLoadingSeconds = ref(0);
const showExtendedWorkflowLoading = computed(() => workflowLoadingSeconds.value >= 3);
const workflowProgress = ref<ManualWorkflowsProgress>({ stage: 'fetching', current: 0, total: 0 });
const hasWorkflowProgressTotal = computed(() => Number(workflowProgress.value.total ?? 0) > 0);
const workflowProgressPercent = computed(() => {
  if (!workflowProgress.value.total) return 0;
  return (Number(workflowProgress.value.current ?? 0) / workflowProgress.value.total) * 100;
});
const workflowProgressText = computed(() => {
  const progress = workflowProgress.value;
  if (progress.stage === 'parsing' && progress.total) {
    return i18n.t('repo.manual_pipeline.parsing_workflows_progress', {
      current: progress.current ?? 0,
      total: progress.total,
    });
  }
  if (progress.stage === 'preprocessing') {
    return i18n.t('repo.manual_pipeline.preprocessing_workflows');
  }
  return i18n.t('repo.manual_pipeline.fetching_workflows');
});
let workflowLoadingIntervalId: ReturnType<typeof setInterval> | null = null;

const loading = ref(false);
const isSubmitting = ref(false);
const hasFetchedRefs = ref(false);

watch(workflowsLoading, (isLoading) => {
  if (workflowLoadingIntervalId !== null) {
    clearInterval(workflowLoadingIntervalId);
    workflowLoadingIntervalId = null;
  }

  workflowLoadingSeconds.value = 0;
  if (isLoading) {
    workflowLoadingIntervalId = setInterval(() => {
      workflowLoadingSeconds.value += 1;
    }, 1000);
  }
});

onScopeDispose(() => {
  if (workflowLoadingIntervalId !== null) {
    clearInterval(workflowLoadingIntervalId);
  }
});

// Variables derived from the selected workflows
const variableValues = ref<Record<string, string>>({});

const grouped = computed(() => groupManualVariables(workflows.value, selectedWorkflows.value));

const requiredVariablesValid = computed(() =>
  grouped.value.all.every((v) => !v.required || (variableValues.value[v.name] ?? '') !== ''),
);

const isFormValid = computed(() => {
  return selectedRef.value !== null && selectedWorkflows.value.size > 0 && requiredVariablesValid.value;
});

// The request carries either a branch or a tag, never both: the server rejects
// an ambiguous pair with a 400.
const refPayload = computed(() =>
  selectedRef.value?.type === 'tag' ? { tag: selectedRef.value.name } : { branch: selectedRef.value?.name ?? '' },
);

function isSelected(type: RefKind, name: string): boolean {
  return selectedRef.value?.type === type && selectedRef.value.name === name;
}

function selectRef(type: RefKind, name: string): void {
  selectedRef.value = { type, name };
  refPickerOpen.value = false;
}

watch(refPickerOpen, (isOpen) => {
  if (!isOpen) refSearch.value = '';
});

// Seed defaults when the selection changes, preserving values the user already entered.
watch(grouped, (g) => {
  const next: Record<string, string> = {};
  for (const v of g.all) {
    next[v.name] = variableValues.value[v.name] ?? getSelectedWorkflowDefault(v);
  }
  variableValues.value = next;
});

// Tags are optional per forge: a forge without tag support answers 501, and
// repos simply may not have any. Neither is an error, so failures leave the
// list empty and the picker shows branches only.
async function fetchTags(): Promise<string[]> {
  try {
    return await usePaginate((page) => apiClient.getRepoTags(repo.value.id, { page }));
  } catch (error) {
    console.warn('Tags unavailable for this repo:', error);
    return [];
  }
}

// Fetch branches and tags when dialog opens
watch(
  () => props.open,
  async (isOpen) => {
    if (isOpen && !hasFetchedRefs.value) {
      loading.value = true;
      try {
        const [data, tagData] = await Promise.all([
          usePaginate((page) => apiClient.getRepoBranches(repo.value.id, { page })),
          fetchTags(),
        ]);
        // Sort branches: default branch first, then alphabetically
        const defaultBranch = repo.value.default_branch;
        branches.value = data.sort((a, b) => {
          if (a === defaultBranch) return -1;
          if (b === defaultBranch) return 1;
          return a.localeCompare(b);
        });
        tags.value = tagData;
        // Start on the default branch
        const initialBranch = defaultBranch || data[0] || '';
        selectedRef.value = initialBranch ? { type: 'branch', name: initialBranch } : null;
        hasFetchedRefs.value = true;
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

// Fetch workflows when the selected ref changes
watch(
  selectedRef,
  async (target) => {
    if (!target) {
      workflows.value = [];
      workflowWarnings.value = [];
      selectedWorkflows.value = new Set();
      return;
    }

    workflowsLoading.value = true;
    workflowProgress.value = { stage: 'fetching', current: 0, total: 0 };
    try {
      const response = await apiClient.getRepoManualWorkflowsWithProgress(
        repo.value.id,
        refPayload.value,
        (progress) => {
          workflowProgress.value = progress;
        },
      );
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

function buildPayloadFromVariables(vars: ManualVariable[]): Record<string, string> {
  const result: Record<string, string> = {};
  for (const v of vars) {
    // Fall back to the selected-workflow default so quick-start works even
    // when the workflow was never seeded into variableValues.
    const value = variableValues.value[v.name] ?? getSelectedWorkflowDefault(v);
    if (value === '' && !v.required && !v.default) continue;
    result[v.name] = value;
  }
  return result;
}

function buildVariablePayload(): Record<string, string> {
  return buildPayloadFromVariables(grouped.value.all);
}

async function triggerManualPipeline() {
  if (selectedWorkflows.value.size === 0) {
    return;
  }

  isSubmitting.value = true;

  try {
    const pipeline = await overlay.withOverlay(i18n.t('loading_overlay.creating_pipeline'), () =>
      apiClient.createPipeline(repo.value.id, {
        ...refPayload.value,
        variables: buildVariablePayload(),
        workflows: [...selectedWorkflows.value],
      }),
    );

    emit('close');

    selectedWorkflows.value = new Set();
    variableValues.value = {};

    await router.push({
      name: 'repo-pipeline',
      params: {
        pipelineId: pipeline.number,
      },
    });
  } catch (error: unknown) {
    emit('close');

    if (error instanceof PipelineFilteredError) {
      const helpUrl = 'https://crowci.dev/dev/usage/workflow-syntax/#when-step';
      notifications.notify({
        title: i18n.t('notify_title_error'),
        type: 'error',
        duration: 9000,
        text: `${i18n.t('repo.manual_pipeline.steps_filtered_out')} <a href='${helpUrl}' target='_blank' class='text-primary underline'>${i18n.t('repo.manual_pipeline.when_condition_help')}</a>`,
      });
    } else {
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

  const workflow = workflows.value.find((w) => w.name === workflowName);

  try {
    const pipeline = await overlay.withOverlay(i18n.t('loading_overlay.creating_pipeline'), () =>
      apiClient.createPipeline(repo.value.id, {
        ...refPayload.value,
        variables: buildPayloadFromVariables(workflow?.variables ?? []),
        workflows: [workflowName],
      }),
    );

    emit('close');

    variableValues.value = {};

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

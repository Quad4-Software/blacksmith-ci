<template>
  <div v-if="canRun" class="inline-flex shrink-0">
    <Button size="sm" class="h-8 rounded-r-none" @click="$emit('configure')">
      <Icon name="play" class="mr-1 h-4 w-4" />
      {{ $t('repo.manual_pipeline.trigger') }}
    </Button>
    <Popover v-model:open="open">
      <PopoverTrigger as-child>
        <Button
          size="sm"
          class="h-8 w-8 rounded-l-none border-l border-primary-foreground/30 px-0"
          :aria-label="$t('repo.manual_pipeline.quick_run_menu')"
          :title="$t('repo.manual_pipeline.quick_run_menu')"
        >
          <Icon name="chevron-down" class="h-4 w-4" />
        </Button>
      </PopoverTrigger>
      <PopoverContent :collision-padding="12" align="start" class="w-80 max-w-[calc(100vw-2rem)] p-1">
        <div class="space-y-1 border-b px-2 py-2">
          <p class="text-sm font-medium">{{ $t('repo.manual_pipeline.quick_run_menu') }}</p>
          <p class="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Icon name="branch" class="h-3.5 w-3.5 shrink-0" />
            <span class="truncate" :title="branch">{{ branch }}</span>
          </p>
          <p class="text-xs text-muted-foreground">{{ $t('repo.manual_pipeline.uses_defaults') }}</p>
        </div>
        <div class="max-h-[min(50vh,var(--reka-popover-content-available-height))] overflow-y-auto py-1">
          <Button
            v-for="name in names"
            :key="name"
            variant="ghost"
            class="h-auto min-h-9 w-full justify-start gap-2 px-2 text-left font-normal"
            :disabled="running !== null"
            :aria-label="runLabel(name)"
            :title="runLabel(name)"
            @click="run(name)"
          >
            <Icon
              :name="running === name ? 'spinner' : 'play'"
              class="h-4 w-4 shrink-0"
              :class="{ 'animate-spin': running === name }"
            />
            <span class="truncate">{{ name }}</span>
          </Button>
          <Icon v-if="loadingNames" name="spinner" class="mx-auto my-2 h-4 w-4 animate-spin" />
          <p v-else-if="!names.length" class="px-2 py-2 text-sm text-muted-foreground">
            {{ $t('repo.manual_pipeline.no_workflows') }}
          </p>
        </div>
      </PopoverContent>
    </Popover>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';
import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import useApiClient from '~/compositions/useApiClient';
import useNotifications from '~/compositions/useNotifications';
import type { Repo } from '~/lib/api/types';

const props = defineProps<{ repo: Repo; canRun: boolean; branch: string }>();
defineEmits<{ configure: [] }>();
const api = useApiClient();
const notifications = useNotifications();
const router = useRouter();
const { t } = useI18n();
const open = ref(false);
const names = ref<string[]>([]);
const loadingNames = ref(false);
const running = ref<string | null>(null);

watch(
  () => props.repo.id,
  async (id, _, onCleanup) => {
    let stale = false;
    onCleanup(() => {
      stale = true;
    });
    loadingNames.value = true;
    names.value = [];
    open.value = false;
    try {
      const result = await api.getWorkflowNames(id, { silent: true });
      if (!stale) names.value = result;
    } catch {
      // A repository without workflow history has no shortcuts to show.
    } finally {
      if (!stale) loadingNames.value = false;
    }
  },
  { immediate: true },
);

function runLabel(name: string) {
  return t('repo.manual_pipeline.run_workflow', { name, branch: props.branch });
}

async function run(name: string) {
  if (!props.canRun || running.value !== null) return;
  const repoId = props.repo.id;
  const branch = props.branch;
  running.value = name;
  try {
    const config = await api.getRepoManualWorkflows(repoId, { branch });
    const workflow = config.workflows.find((workflow) => workflow.name === name);
    const variables = Object.fromEntries(
      (workflow?.variables ?? [])
        .filter((variable) => variable.default !== undefined)
        .map((variable) => [variable.name, variable.default!]),
    );
    const pipeline = await api.createPipeline(repoId, { branch, workflows: [name], variables });
    open.value = false;
    await router.push({ name: 'repo-pipeline', params: { repoId, pipelineId: pipeline.number } });
  } catch (error) {
    notifications.notify({
      title: t('notify_title_error'),
      text: error instanceof Error ? error.message : String(error),
      type: 'error',
    });
  } finally {
    running.value = null;
  }
}
</script>

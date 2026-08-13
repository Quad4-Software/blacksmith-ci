<template>
  <Panel>
    <div v-if="isLoading" class="flex items-center justify-center py-10" aria-live="polite">
      <Spinner class="text-primary" />
      <span class="ml-3 text-sm text-muted-foreground">{{ $t('repo.pipeline.loading') }}</span>
    </div>

    <div v-else class="flex w-full flex-col gap-2">
      <details
        v-for="file in files"
        :key="file.path"
        class="group overflow-hidden rounded-md border border-border bg-background"
      >
        <summary
          class="flex cursor-pointer list-none items-center gap-3 px-3 py-2.5 text-sm hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring [&::-webkit-details-marker]:hidden"
        >
          <Icon
            name="chevron-right"
            class="h-5 w-5 shrink-0 text-muted-foreground transition-transform group-open:rotate-90"
          />
          <Icon name="file" class="shrink-0 text-muted-foreground" />
          <span class="min-w-0 truncate font-mono" :title="file.path">{{ file.path }}</span>
          <span v-if="file.additions || file.deletions" class="ml-auto flex shrink-0 gap-2 font-mono text-xs">
            <span class="flex items-center text-green-600 dark:text-green-400">
              <Icon name="plus" class="h-3 w-3" />{{ file.additions }}
            </span>
            <span class="flex items-center text-red-600 dark:text-red-400">
              <Icon name="status-skipped" class="h-3 w-3" />{{ file.deletions }}
            </span>
          </span>
        </summary>

        <div v-if="file.lines.length" class="overflow-x-auto border-t border-border bg-muted/20">
          <pre class="min-w-max py-2 font-mono text-xs leading-5"><code><span
            v-for="(line, index) in file.lines"
            :key="index"
            class="block min-h-5 whitespace-pre px-3"
            :class="lineClass(line.type)"
          >{{ line.content || ' ' }}</span></code></pre>
        </div>
        <p v-else class="border-t border-border px-4 py-3 text-sm text-muted-foreground">
          {{ $t('repo.pipeline.diff_unavailable') }}
        </p>
      </details>
    </div>
  </Panel>
</template>

<script lang="ts" setup>
import Icon from '~/components/atomic/Icon.vue';
import Panel from '~/components/layout/Panel.vue';
import { Spinner } from '~/components/ui/spinner';
import useApiClient from '~/compositions/useApiClient';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import { requiredInject } from '~/compositions/useInjectProvide';
import { parseUnifiedDiff } from '~/lib/unified-diff';
import type { DiffLine } from '~/lib/unified-diff';
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';

const repo = requiredInject('repo');
const pipeline = requiredInject('pipeline');
const apiClient = useApiClient();
const diff = ref('');
const isLoading = ref(true);

const { t } = useI18n();
useCrowTitle(
  computed(() => [
    t('repo.pipeline.files'),
    t('repo.pipeline.pipeline', { pipelineId: pipeline.value.number }),
    repo.value.full_name,
  ]),
);

const files = computed(() => {
  const parsed = parseUnifiedDiff(diff.value);
  const byPath = new Map(parsed.map((file) => [file.path, file]));
  return (pipeline.value.changed_files ?? []).map((path) => {
    const file = byPath.get(path);
    const lines = file?.lines ?? [];
    return {
      path,
      lines,
      additions: lines.filter((line) => line.type === 'addition').length,
      deletions: lines.filter((line) => line.type === 'deletion').length,
    };
  });
});

onMounted(async () => {
  try {
    const response = await apiClient.getPipelineDiff(repo.value.id, pipeline.value.number);
    diff.value = response.diff;
  } catch {
    // Changed file names remain useful when a forge does not expose raw diffs.
  } finally {
    isLoading.value = false;
  }
});

function lineClass(type: DiffLine['type']): string {
  switch (type) {
    case 'addition':
      return 'bg-green-500/10 text-green-900 dark:text-green-200';
    case 'deletion':
      return 'bg-red-500/10 text-red-900 dark:text-red-200';
    case 'hunk':
      return 'bg-blue-500/10 text-blue-700 dark:text-blue-300';
    case 'metadata':
      return 'text-muted-foreground';
    default:
      return '';
  }
}
</script>

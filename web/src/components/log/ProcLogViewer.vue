<template>
  <div class="flex flex-col h-full">
    <div class="flex items-center gap-2 px-3 py-2 border-b">
      <Input v-model="search" type="search" :placeholder="$t('proclog.search')" class="flex-1 h-8 text-sm" />
      <Select v-model="levelFilter">
        <SelectTrigger class="w-32 h-8 text-sm">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{{ $t('proclog.level.all') }}</SelectItem>
          <SelectItem value="debug">{{ $t('proclog.level.debug') }}</SelectItem>
          <SelectItem value="info">{{ $t('proclog.level.info') }}</SelectItem>
          <SelectItem value="warn">{{ $t('proclog.level.warn') }}</SelectItem>
          <SelectItem value="error">{{ $t('proclog.level.error') }}</SelectItem>
        </SelectContent>
      </Select>
      <Button variant="outline" size="sm" @click="paused = !paused">
        {{ paused ? $t('proclog.resume') : $t('proclog.pause') }}
      </Button>
    </div>

    <div ref="scrollEl" class="flex-1 overflow-auto font-mono text-xs">
      <div v-for="entry in visible" :key="entry.id" :class="rowClass(entry)" class="px-3 py-0.5">
        <span class="text-muted-foreground">{{ formatTime(entry.time) }}</span>
        <span class="ml-2 uppercase font-semibold" :class="levelClass(entry.level)">{{ entry.level }}</span>
        <span class="ml-2 whitespace-pre-wrap">{{ entry.line }}</span>
      </div>
      <div v-if="visible.length === 0" class="px-3 py-4 text-muted-foreground">
        {{ $t('proclog.empty') }}
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';

import { Button } from '~/components/ui/button';
import { Input } from '~/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import useApiClient from '~/compositions/useApiClient';
import type { ProcLogEntry } from '~/lib/api/types/procLog';

const props = defineProps<{
  source: 'agent' | 'autoscaler';
  sourceId: number;
}>();

const RATE_LIMIT_MARKER_RE = /^\d+ lines dropped \(rate limit\)$/;
const MAX_ENTRIES = 5000;

const apiClient = useApiClient();
const entries = ref<ProcLogEntry[]>([]);
const search = ref('');
const levelFilter = ref<'all' | ProcLogEntry['level']>('all');
const paused = ref(false);
const scrollEl = ref<HTMLDivElement | null>(null);

const visible = computed(() =>
  entries.value.filter((e) => {
    if (levelFilter.value !== 'all' && e.level !== levelFilter.value) return false;
    if (search.value && !e.line.includes(search.value)) return false;
    return true;
  }),
);

let evtSource: EventSource | null = null;

function open() {
  evtSource?.close();
  evtSource =
    props.source === 'agent'
      ? apiClient.subscribeAgentLogs(props.sourceId, onEntry)
      : apiClient.subscribeAutoscalerLogs(props.sourceId, onEntry);
}

function onEntry(entry: ProcLogEntry) {
  if (paused.value) return;
  entries.value.push(entry);
  if (entries.value.length > MAX_ENTRIES) {
    entries.value.splice(0, entries.value.length - MAX_ENTRIES);
  }
  void nextTick(() => {
    if (scrollEl.value) {
      scrollEl.value.scrollTop = scrollEl.value.scrollHeight;
    }
  });
}

function formatTime(t: number): string {
  return new Date(t / 1_000_000).toISOString().slice(11, 23);
}

function levelClass(level: ProcLogEntry['level']) {
  return {
    'text-muted-foreground': level === 'debug' || level === 'trace',
    'text-blue-500 dark:text-blue-400': level === 'info',
    'text-yellow-600 dark:text-yellow-400': level === 'warn',
    'text-destructive': level === 'error' || level === 'fatal',
  };
}

function rowClass(entry: ProcLogEntry) {
  return {
    'bg-yellow-600/10 dark:bg-yellow-800/20': RATE_LIMIT_MARKER_RE.test(entry.line),
  };
}

watch(
  () => [props.source, props.sourceId] as const,
  () => {
    entries.value = [];
    open();
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  evtSource?.close();
});
</script>

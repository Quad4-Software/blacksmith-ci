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
      <Button
        variant="outline"
        size="icon"
        class="h-8 w-8"
        :disabled="visible.length === 0"
        :title="copied ? $t('proclog.copied') : $t('proclog.copy')"
        @click="copyLogs"
      >
        <Icon :name="copied ? 'ph:check' : 'ph:copy'" class="h-4 w-4" />
      </Button>
      <Button
        variant="outline"
        size="icon"
        class="h-8 w-8"
        :disabled="visible.length === 0"
        :title="$t('proclog.download')"
        @click="downloadLogs"
      >
        <Icon name="download" class="h-4 w-4" />
      </Button>
    </div>

    <div ref="scrollEl" class="flex-1 overflow-auto font-mono text-xs">
      <div v-for="entry in visible" :key="entry.id" :class="rowClass(entry)" class="proclog-row px-3 py-0.5">
        <span class="text-muted-foreground">{{ formatTime(entry.time) }}</span>
        <span class="ml-2 uppercase font-semibold" :class="levelClass(entry.level)">{{ entry.level }}</span>
        <span class="ml-2 whitespace-pre-wrap">{{ entry.line }}</span>
      </div>
      <div v-if="visible.length === 0" class="px-3 py-4 text-muted-foreground text-sm font-sans">
        <div v-if="connState === 'connecting'" class="flex items-center gap-2">
          <Icon name="loading" class="h-4 w-4 animate-spin" />
          <span>{{ $t('proclog.connecting') }}</span>
        </div>
        <div v-else-if="connState === 'error'" class="flex items-center gap-2 text-destructive">
          <Icon name="alert" class="h-4 w-4" />
          <span>{{ $t('proclog.connection_error') }}</span>
        </div>
        <div v-else class="flex items-start gap-2">
          <Icon name="info" class="h-4 w-4 mt-0.5 shrink-0" />
          <div>
            <p>{{ entries.length === 0 ? $t('proclog.waiting') : $t('proclog.no_match') }}</p>
            <p v-if="entries.length === 0" class="mt-1 text-xs">{{ $t('proclog.waiting_hint') }}</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import { Input } from '~/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import useApiClient from '~/compositions/useApiClient';
import type { ProcLogEntry } from '~/lib/api/types/procLog';
import { computed, nextTick, onBeforeUnmount, ref, shallowRef, watch } from 'vue';

const props = defineProps<{
  source: 'agent' | 'autoscaler';
  sourceId: number;
}>();

const RATE_LIMIT_MARKER_RE = /^\d+ lines dropped \(rate limit\)$/;
const MAX_ENTRIES = 5000;
// The server replays up to MAX_ENTRIES backlog rows on connect, one SSE message
// each, and a chatty agent keeps that rate up. Rendering per message means one
// full-list patch plus one forced layout per message, so incoming entries are
// collected here and flushed at most every FLUSH_INTERVAL ms instead.
const FLUSH_INTERVAL = 100;

const apiClient = useApiClient();
// shallowRef: entries are append-only and replaced wholesale on flush, so deep
// reactivity would only cost us a proxy per log line.
const entries = shallowRef<ProcLogEntry[]>([]);
const search = ref('');
const levelFilter = ref<'all' | ProcLogEntry['level']>('all');
const paused = ref(false);
const copied = ref(false);
const connState = ref<'connecting' | 'open' | 'error'>('connecting');
const scrollEl = ref<HTMLDivElement | null>(null);
let copiedTimer: ReturnType<typeof setTimeout> | null = null;
let pending: ProcLogEntry[] = [];
let flushTimer: ReturnType<typeof setTimeout> | null = null;
let lastEntryID = 0;

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
  connState.value = 'connecting';
  evtSource =
    props.source === 'agent'
      ? apiClient.subscribeAgentLogs(props.sourceId, onEntry)
      : apiClient.subscribeAutoscalerLogs(props.sourceId, onEntry);
  evtSource.addEventListener('open', () => {
    connState.value = 'open';
  });
  evtSource.addEventListener('error', () => {
    connState.value = evtSource?.readyState === EventSource.CLOSED ? 'error' : 'connecting';
  });
}

function flush() {
  flushTimer = null;
  if (pending.length === 0) return;

  const next = entries.value.concat(pending);
  pending = [];
  entries.value = next.length > MAX_ENTRIES ? next.slice(next.length - MAX_ENTRIES) : next;

  void nextTick(() => {
    if (scrollEl.value) {
      scrollEl.value.scrollTop = scrollEl.value.scrollHeight;
    }
  });
}

function onEntry(entry: ProcLogEntry) {
  if (paused.value) return;
  // EventSource reconnects transparently and the server replays its recent
  // window, so ids we already hold would come back as duplicate `:key`s.
  if (entry.id <= lastEntryID) return;
  lastEntryID = entry.id;
  pending.push(entry);
  if (pending.length > MAX_ENTRIES) {
    pending.splice(0, pending.length - MAX_ENTRIES);
  }
  // Fixed deadline rather than a debounce: a continuous stream must never
  // starve the flush.
  flushTimer ??= setTimeout(flush, FLUSH_INTERVAL);
}

function formatTime(t: number): string {
  return new Date(t / 1_000_000).toISOString().slice(11, 23);
}

function visibleAsText(): string {
  return visible.value
    .map((e) => `${new Date(e.time / 1_000_000).toISOString().slice(11, 19)} ${e.level.toUpperCase()} ${e.line}`)
    .join('\n');
}

async function copyLogs() {
  if (visible.value.length === 0) return;
  try {
    await navigator.clipboard.writeText(visibleAsText());
    copied.value = true;
    if (copiedTimer) clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => {
      copied.value = false;
    }, 1500);
  } catch {
    // clipboard write rejected (insecure context or denied) — silently no-op
  }
}

function downloadLogs() {
  if (visible.value.length === 0) return;
  const blob = new Blob([visibleAsText()], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${props.source}-${props.sourceId}-logs.log`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
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
    pending = [];
    lastEntryID = 0;
    if (flushTimer) {
      clearTimeout(flushTimer);
      flushTimer = null;
    }
    open();
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  evtSource?.close();
  if (copiedTimer) clearTimeout(copiedTimer);
  if (flushTimer) clearTimeout(flushTimer);
});
</script>

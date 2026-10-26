<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <button
        class="flex items-center justify-center gap-1.5 px-2 py-1.5 mr-1 rounded-md transition-colors hover:bg-accent"
        :class="attentionCount > 0 ? 'bg-destructive/10 hover:bg-destructive/20' : ''"
        :title="bellTooltip"
        :aria-label="bellTooltip"
        data-testid="watch-bell"
      >
        <Icon
          name="ph:bell"
          class="size-4"
          :class="attentionCount > 0 ? 'text-destructive' : 'text-muted-foreground'"
        />
        <span v-if="attentionCount > 0" class="text-xs font-semibold leading-none text-destructive tabular-nums">
          {{ attentionCount }}
        </span>
      </button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end" class="w-96">
      <template v-if="stuck.length > 0">
        <DropdownMenuLabel>{{ $t('navbar.watches.stuck_title') }}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        <div class="max-h-60 overflow-y-auto">
          <router-link
            v-for="item in stuck"
            :key="`${item.repo_id}-${item.pipeline_number}`"
            :to="{ name: 'repo-pipeline', params: { repoId: item.repo_id, pipelineId: item.pipeline_number } }"
            class="flex min-w-0 flex-col gap-0.5 rounded-sm px-2 py-2 hover:bg-accent"
            data-testid="stuck-bell-row"
          >
            <span class="truncate text-sm font-medium">{{ item.repo_full_name }}</span>
            <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
            <span class="truncate text-xs text-muted-foreground">#{{ item.pipeline_number }}</span>
            <span class="truncate text-xs text-destructive">{{ stuckWorkflowNames(item) }}</span>
            <span class="text-xs text-muted-foreground">{{ timeAgo(item.created * 1000) }}</span>
          </router-link>
        </div>
        <DropdownMenuSeparator />
      </template>
      <DropdownMenuLabel class="flex items-center justify-between gap-2">
        <span>{{ $t('navbar.watches.title') }}</span>
        <Button
          v-if="unreadCount > 0"
          variant="ghost"
          size="sm"
          class="h-6 px-2 text-xs"
          :disabled="isMarkingAll"
          @click.prevent="markAll()"
        >
          <Icon v-if="isMarkingAll" name="spinner" class="size-3 animate-spin mr-1" />
          {{ $t('navbar.watches.mark_all_read') }}
        </Button>
      </DropdownMenuLabel>
      <DropdownMenuSeparator />
      <div class="max-h-80 overflow-y-auto">
        <div v-if="failing.length === 0" class="px-2 py-6 text-center text-sm text-muted-foreground">
          {{ $t('navbar.watches.empty') }}
        </div>
        <div
          v-for="item in failing"
          :key="item.watch.id"
          class="flex items-start justify-between gap-2 px-2 py-2"
          :class="item.unread ? '' : 'opacity-60'"
          data-testid="watch-bell-row"
        >
          <router-link
            v-if="item.latest_pipeline"
            :to="{
              name: 'repo-pipeline',
              params: { repoId: item.watch.repo_id, pipelineId: item.latest_pipeline.number },
            }"
            class="flex min-w-0 flex-col gap-0.5"
          >
            <span class="truncate text-sm font-medium">{{ item.repo_full_name }}</span>
            <span class="truncate text-xs text-muted-foreground">
              {{ `${watchLabel(item.watch.event, item.watch.target)} · #${item.latest_pipeline.number}` }}
            </span>
            <span class="truncate text-xs text-destructive">{{
              failedNames(item, $t('user.watching.config_error'))
            }}</span>
            <span class="text-xs text-muted-foreground">
              {{ timeAgo((item.latest_pipeline.finished || item.latest_pipeline.created) * 1000) }}
            </span>
          </router-link>
          <Button
            v-if="item.unread"
            variant="ghost"
            size="icon"
            class="size-7 shrink-0"
            :title="$t('navbar.watches.mark_read')"
            :disabled="busy.has(item.watch.id)"
            @click.prevent="markRead(item.watch.id)"
          >
            <Icon v-if="busy.has(item.watch.id)" name="spinner" class="size-4 animate-spin" />
            <Icon v-else name="ph:check" class="size-4" />
          </Button>
        </div>
      </div>
      <DropdownMenuSeparator />
      <DropdownMenuItem as-child>
        <router-link
          :to="{ name: 'user-watching' }"
          class="flex w-full cursor-pointer items-center justify-center text-xs"
        >
          {{ $t('navbar.watches.view_all') }}
        </router-link>
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, reactive } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useDate } from '~/compositions/useDate';
import { useStuckPipelines } from '~/compositions/useStuckPipelines';
import { useWatches } from '~/compositions/useWatches';
import type { StuckPipeline } from '~/lib/api/types';
import { failedNames } from '~/lib/watchDisplay';
import { watchLabel } from '~/lib/watchKey';

const { timeAgo } = useDate();
const { failing, unreadCount, load, markRead: doMarkRead, markAllRead } = useWatches();
const { stuck, load: loadStuck } = useStuckPipelines();
const { t } = useI18n();

// Stuck pipelines have no read state: they count until they start or are canceled.
const attentionCount = computed(() => unreadCount.value + stuck.value.length);
const bellTooltip = computed(() =>
  stuck.value.length > 0
    ? t('navbar.watches.attention_tooltip', { count: attentionCount.value })
    : t('navbar.watches.tooltip', { count: unreadCount.value }),
);

function stuckWorkflowNames(item: StuckPipeline): string {
  return item.workflows
    .map(
      (w) =>
        `${w.name} (${w.reason === 'blocked' ? t('navbar.watches.stuck_blocked') : t('navbar.watches.stuck_waiting')})`,
    )
    .join(', ');
}

const busy = reactive(new Set<number>());

async function markRead(id: number) {
  busy.add(id);
  try {
    await doMarkRead(id);
  } catch (error) {
    console.error(error);
  } finally {
    busy.delete(id);
  }
}

const { doSubmit: markAll, isLoading: isMarkingAll } = useAsyncAction(markAllRead);

// Workflows become stuck by waiting, not through an event, so poll.
const stuckPollInterval = 60_000;
let stuckPoll: number | undefined;

onMounted(() => {
  void load().catch(console.error);
  void loadStuck().catch(console.error);
  stuckPoll = window.setInterval(() => {
    void loadStuck().catch(console.error);
  }, stuckPollInterval);
});

onBeforeUnmount(() => {
  window.clearInterval(stuckPoll);
});
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <button
        class="flex items-center justify-center gap-1.5 px-2 py-1.5 mr-1 rounded-md transition-colors hover:bg-accent"
        :class="unreadCount > 0 ? 'bg-destructive/10 hover:bg-destructive/20' : ''"
        :title="$t('navbar.watches.tooltip', { count: unreadCount })"
        :aria-label="$t('navbar.watches.tooltip', { count: unreadCount })"
        data-testid="watch-bell"
      >
        <Icon name="ph:bell" class="size-4" :class="unreadCount > 0 ? 'text-destructive' : 'text-muted-foreground'" />
        <span v-if="unreadCount > 0" class="text-xs font-semibold leading-none text-destructive tabular-nums">
          {{ unreadCount }}
        </span>
      </button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end" class="w-96">
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
import { onMounted, reactive } from 'vue';

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
import { useWatches } from '~/compositions/useWatches';
import { failedNames } from '~/lib/watchDisplay';
import { watchLabel } from '~/lib/watchKey';

const { timeAgo } = useDate();
const { failing, unreadCount, load, markRead: doMarkRead, markAllRead } = useWatches();

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

onMounted(() => {
  void load().catch(console.error);
});
</script>

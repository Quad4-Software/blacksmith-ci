<template>
  <Scaffold fill-width>
    <template #title>{{ $t('user.watching.title') }}</template>

    <div class="max-w-8xl mx-auto space-y-4">
      <p class="text-sm text-muted-foreground">{{ $t('user.watching.description') }}</p>

      <Tabs v-model="activeTab" class="w-full">
        <TabsList class="mb-4">
          <TabsTrigger value="failing" class="gap-2">
            {{ $t('user.watching.failing') }}
            <Badge v-if="failing.length > 0" variant="destructive" class="ml-1">{{ failing.length }}</Badge>
          </TabsTrigger>
          <TabsTrigger value="all" class="gap-2">
            {{ $t('user.watching.all') }}
            <Badge v-if="items.length > 0" variant="secondary" class="ml-1">{{ items.length }}</Badge>
          </TabsTrigger>
        </TabsList>

        <div v-if="loading && items.length === 0" class="flex justify-center py-8">
          <Icon name="spinner" class="size-6 animate-spin" />
        </div>

        <TabsContent value="failing" class="space-y-6">
          <div v-if="!loading && failing.length === 0" class="py-8 text-center text-sm text-muted-foreground">
            {{ $t('user.watching.none_failing') }}
          </div>
          <template v-for="group in groups" :key="group.key">
            <section v-if="group.items.length > 0" class="space-y-2">
              <div class="flex items-center justify-between">
                <h3 class="text-sm font-semibold">{{ group.title }}</h3>
                <Button
                  v-if="group.key === 'unread'"
                  variant="outline"
                  size="sm"
                  :disabled="isMarkingAll"
                  @click="markAll()"
                >
                  <Icon v-if="isMarkingAll" name="spinner" class="size-4 animate-spin mr-2" />
                  {{ $t('navbar.watches.mark_all_read') }}
                </Button>
              </div>
              <div
                v-for="item in group.items"
                :key="item.watch.id"
                class="flex items-center justify-between gap-4 rounded-md border p-3"
                :class="item.unread ? '' : 'opacity-70'"
                data-testid="watching-failing-row"
              >
                <div class="flex min-w-0 flex-col gap-1">
                  <span class="truncate text-sm font-medium">{{ item.repo_full_name }}</span>
                  <span class="flex flex-wrap gap-x-2 text-xs text-muted-foreground">
                    <span>{{ watchLabel(item.watch.event, item.watch.target) }}</span>
                    <template v-if="item.latest_pipeline">
                      <router-link
                        :to="{
                          name: 'repo-pipeline',
                          params: { repoId: item.watch.repo_id, pipelineId: item.latest_pipeline.number },
                        }"
                        class="underline"
                      >
                        {{ `#${item.latest_pipeline.number}` }}
                      </router-link>
                      <span>{{ timeAgo((item.latest_pipeline.finished || item.latest_pipeline.created) * 1000) }}</span>
                    </template>
                  </span>
                  <span class="truncate text-xs text-destructive">
                    {{ failedNames(item, $t('user.watching.config_error')) }}
                  </span>
                  <div class="flex gap-1">
                    <Badge v-for="c in channels(item)" :key="c" variant="outline">{{ c }}</Badge>
                  </div>
                </div>
                <div class="flex shrink-0 gap-2">
                  <Button
                    v-if="item.unread"
                    variant="outline"
                    size="sm"
                    :disabled="busy.has(item.watch.id)"
                    @click="markRead(item.watch.id)"
                  >
                    <Icon v-if="busy.has(item.watch.id)" name="spinner" class="size-4 animate-spin mr-2" />
                    {{ $t('user.watching.mark_read') }}
                  </Button>
                  <Button variant="ghost" size="sm" @click="confirmUnwatch = item">
                    {{ $t('user.watching.unwatch') }}
                  </Button>
                </div>
              </div>
            </section>
          </template>
        </TabsContent>

        <TabsContent value="all">
          <div v-if="!loading && items.length === 0" class="py-8 text-center text-sm text-muted-foreground">
            {{ $t('user.watching.none') }}
          </div>
          <Table v-else>
            <TableHeader>
              <TableRow>
                <TableHead>{{ $t('user.watching.col_watch') }}</TableHead>
                <TableHead>{{ $t('user.watching.col_repo') }}</TableHead>
                <TableHead>{{ $t('user.watching.col_channels') }}</TableHead>
                <TableHead>{{ $t('user.watching.col_last_run') }}</TableHead>
                <TableHead class="text-right">{{ $t('user.watching.col_actions') }}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="item in items" :key="item.watch.id" data-testid="watching-all-row">
                <TableCell>{{ watchLabel(item.watch.event, item.watch.target) }}</TableCell>
                <TableCell>{{ item.repo_full_name }}</TableCell>
                <TableCell>
                  <div class="flex gap-1">
                    <Badge v-for="c in channels(item)" :key="c" variant="outline">{{ c }}</Badge>
                  </div>
                </TableCell>
                <TableCell>
                  <router-link
                    v-if="item.latest_pipeline"
                    :to="{
                      name: 'repo-pipeline',
                      params: { repoId: item.watch.repo_id, pipelineId: item.latest_pipeline.number },
                    }"
                    class="flex items-center gap-2"
                  >
                    <PipelineStatusIcon :status="item.latest_pipeline.status" size="1.1rem" />
                    {{ `#${item.latest_pipeline.number}` }}
                  </router-link>
                  <span v-else class="text-muted-foreground">{{ $t('user.watching.no_runs') }}</span>
                </TableCell>
                <TableCell class="text-right">
                  <Button variant="ghost" size="sm" @click="startEdit(item)">{{ $t('edit') }}</Button>
                  <Button variant="ghost" size="sm" @click="confirmUnwatch = item">
                    {{ $t('user.watching.unwatch') }}
                  </Button>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </TabsContent>
      </Tabs>
    </div>

    <Dialog :open="editing !== null" @update:open="(o) => !o && (editing = null)">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{{ $t('user.watching.edit_title') }}</DialogTitle>
        </DialogHeader>
        <WatchSettingsFields v-if="editing" v-model="editSettings" @valid="editValid = $event" />
        <DialogFooter>
          <Button variant="outline" @click="editing = null">{{ $t('cancel') }}</Button>
          <Button :disabled="!editValid || isSaving" @click="save()">
            <Icon v-if="isSaving" name="spinner" class="size-4 animate-spin mr-2" />
            {{ $t('save') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <AlertDialog :open="confirmUnwatch !== null" @update:open="(o) => !o && (confirmUnwatch = null)">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('user.watching.unwatch_title') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{
              confirmUnwatch
                ? $t('user.watching.unwatch_confirm', {
                    label: watchLabel(confirmUnwatch.watch.event, confirmUnwatch.watch.target),
                    repo: confirmUnwatch.repo_full_name,
                  })
                : ''
            }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
          <Button variant="destructive" :disabled="isUnwatching" @click="doUnwatch()">
            <Icon v-if="isUnwatching" name="spinner" class="size-4 animate-spin mr-2" />
            {{ $t('user.watching.unwatch') }}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </Scaffold>
</template>

<script lang="ts" setup>
import { computed, onMounted, reactive, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '~/components/ui/alert-dialog';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '~/components/ui/tabs';
import WatchSettingsFields from '~/components/watch/WatchSettingsFields.vue';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import { useDate } from '~/compositions/useDate';
import useNotifications from '~/compositions/useNotifications';
import { useWatches } from '~/compositions/useWatches';
import type { WatchOverviewItem, WatchSettings } from '~/lib/api/types';
import { failedNames } from '~/lib/watchDisplay';
import { watchLabel } from '~/lib/watchKey';

const { t } = useI18n();
const { timeAgo } = useDate();
const { notify } = useNotifications();
const apiClient = useApiClient();
const { items, failing, loading, load, markRead: doMarkRead, markAllRead, unwatch } = useWatches();

const activeTab = ref('failing');
const busy = reactive(new Set<number>());
const confirmUnwatch = ref<WatchOverviewItem | null>(null);
const editing = ref<WatchOverviewItem | null>(null);
const editSettings = ref<WatchSettings>({ in_app: true, email: false, email_events: ['failure'] });
const editValid = ref(true);

const groups = computed(() => [
  { key: 'unread', title: t('user.watching.unread'), items: failing.value.filter((i) => i.unread) },
  { key: 'read', title: t('user.watching.read'), items: failing.value.filter((i) => !i.unread) },
]);

function channels(item: WatchOverviewItem): string[] {
  const out: string[] = [];
  if (item.watch.in_app) out.push(t('user.watching.channel_in_app'));
  if (item.watch.email) out.push(t('user.watching.channel_email'));
  return out;
}

async function markRead(id: number) {
  busy.add(id);
  try {
    await doMarkRead(id);
  } catch (error) {
    console.error(error);
    notify({ title: t('user.watching.action_failed'), type: 'error' });
  } finally {
    busy.delete(id);
  }
}

const { doSubmit: markAll, isLoading: isMarkingAll } = useAsyncAction(markAllRead, () =>
  notify({ title: t('user.watching.action_failed'), type: 'error' }),
);

const { doSubmit: doUnwatch, isLoading: isUnwatching } = useAsyncAction(
  async () => {
    if (!confirmUnwatch.value) return;
    await unwatch(confirmUnwatch.value.watch.id);
    confirmUnwatch.value = null;
  },
  () => notify({ title: t('user.watching.action_failed'), type: 'error' }),
);

function startEdit(item: WatchOverviewItem) {
  editSettings.value = {
    in_app: item.watch.in_app,
    email: item.watch.email,
    email_events: [...(item.watch.email_events ?? [])],
  };
  editing.value = item;
}

const { doSubmit: save, isLoading: isSaving } = useAsyncAction(
  async () => {
    if (!editing.value) return;
    const updated = await apiClient.updateWatch(editing.value.watch.id, editSettings.value);
    items.value = items.value.map((i) => (i.watch.id === updated.id ? { ...i, watch: updated } : i));
    editing.value = null;
    // Failing and unread depend on the in-app channel, so refetch the overview.
    await load();
  },
  () => notify({ title: t('user.watching.action_failed'), type: 'error' }),
);

onMounted(async () => {
  try {
    await load();
  } catch (error) {
    console.error(error);
  }
});

useCrowTitle(computed(() => [t('user.watching.title')]));
</script>

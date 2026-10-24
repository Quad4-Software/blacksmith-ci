<template>
  <Settings :title="$t('repo.settings.notifications.title', { repo: repo.full_name })">
    <div class="flex flex-col gap-4">
      <div class="flex items-center justify-between gap-4">
        <p class="text-sm text-muted-foreground">{{ $t('repo.settings.notifications.note') }}</p>
        <Button size="sm" class="shrink-0" @click="startAdd">
          <Icon name="plus" class="mr-2 size-4" />
          {{ $t('repo.settings.notifications.add') }}
        </Button>
      </div>

      <div v-if="loading" class="flex justify-center py-8">
        <Icon name="spinner" class="size-6 animate-spin" />
      </div>
      <div v-else-if="items.length === 0" class="py-8 text-center text-sm text-muted-foreground">
        {{ $t('repo.settings.notifications.none') }}
      </div>
      <Table v-else>
        <TableHeader>
          <TableRow>
            <TableHead>{{ $t('repo.settings.notifications.col_watch') }}</TableHead>
            <TableHead>{{ $t('repo.settings.notifications.col_channels') }}</TableHead>
            <TableHead>{{ $t('repo.settings.notifications.col_last_run') }}</TableHead>
            <TableHead class="text-right">{{ $t('repo.settings.notifications.col_actions') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="item in items" :key="item.watch.id" data-testid="repo-watch-row">
            <TableCell>{{ watchLabel(item.watch.event, item.watch.target) }}</TableCell>
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
              <span v-else class="text-muted-foreground">{{ $t('repo.settings.notifications.no_runs') }}</span>
            </TableCell>
            <TableCell class="text-right">
              <Button variant="ghost" size="sm" @click="startEdit(item)">{{ $t('edit') }}</Button>
              <Button variant="ghost" size="sm" @click="confirmUnwatch = item">
                {{ $t('repo.watch.unwatch') }}
              </Button>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <Dialog :open="dialog !== null" @update:open="(o) => !o && (dialog = null)">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {{
              dialog === 'add'
                ? $t('repo.settings.notifications.add_title')
                : $t('repo.settings.notifications.edit_title')
            }}
          </DialogTitle>
        </DialogHeader>
        <div v-if="dialog === 'add'" class="flex flex-col gap-4">
          <div class="flex flex-col gap-2">
            <Label for="watch-new-event">{{ $t('repo.settings.notifications.event') }}</Label>
            <Select v-model="newEvent" @update:model-value="newTarget = ''">
              <SelectTrigger id="watch-new-event" class="w-full" data-testid="watch-new-event">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="e in watchEvents" :key="e" :value="e">{{ e }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div v-if="needsTarget" class="flex flex-col gap-2">
            <Label for="watch-new-target">{{ $t('repo.settings.notifications.target') }}</Label>
            <Input
              id="watch-new-target"
              v-model="newTarget"
              list="watch-target-suggestions"
              :placeholder="$t('repo.settings.notifications.target_placeholder')"
              data-testid="watch-new-target"
            />
            <datalist id="watch-target-suggestions">
              <option v-for="s in targetSuggestions" :key="s" :value="s" />
            </datalist>
          </div>
        </div>
        <WatchSettingsFields v-model="settings" @valid="valid = $event" />
        <DialogFooter>
          <Button variant="outline" @click="dialog = null">{{ $t('cancel') }}</Button>
          <Button :disabled="!canSave || isSaving" data-testid="watch-dialog-save" @click="save()">
            <Icon v-if="isSaving" name="spinner" class="size-4 animate-spin mr-2" />
            {{ $t('save') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <AlertDialog :open="confirmUnwatch !== null" @update:open="(o) => !o && (confirmUnwatch = null)">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('repo.settings.notifications.unwatch_title') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{
              confirmUnwatch
                ? $t('repo.settings.notifications.unwatch_confirm', {
                    label: watchLabel(confirmUnwatch.watch.event, confirmUnwatch.watch.target),
                  })
                : ''
            }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
          <Button variant="destructive" :disabled="isUnwatching" @click="doUnwatch()">
            <Icon v-if="isUnwatching" name="spinner" class="size-4 animate-spin mr-2" />
            {{ $t('repo.watch.unwatch') }}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </Settings>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import Settings from '~/components/layout/Settings.vue';
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
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import WatchSettingsFields from '~/components/watch/WatchSettingsFields.vue';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import { useWatches } from '~/compositions/useWatches';
import type { WatchOverviewItem, WatchSettings, WatchSuggestion } from '~/lib/api/types';
import { watchLabel } from '~/lib/watchKey';

const watchEvents = ['push', 'manual', 'cron', 'deployment', 'tag', 'release'];

const { t } = useI18n();
const apiClient = useApiClient();
const { notify } = useNotifications();
const { load: reloadOverview } = useWatches();
const repo = requiredInject('repo');

const items = ref<WatchOverviewItem[]>([]);
const loading = ref(true);
const suggestions = ref<WatchSuggestion[]>([]);
const dialog = ref<'add' | 'edit' | null>(null);
const editing = ref<WatchOverviewItem | null>(null);
const confirmUnwatch = ref<WatchOverviewItem | null>(null);
const settings = ref<WatchSettings>(defaults());
const valid = ref(true);
const newEvent = ref('push');
const newTarget = ref('');

const needsTarget = computed(() => newEvent.value !== 'tag' && newEvent.value !== 'release');
const targetSuggestions = computed(() => [
  ...new Set(suggestions.value.filter((s) => s.event === newEvent.value).map((s) => s.target)),
]);
const canSave = computed(
  () => valid.value && (dialog.value !== 'add' || !needsTarget.value || newTarget.value.trim() !== ''),
);

function defaults(): WatchSettings {
  return { in_app: true, email: false, email_events: ['failure'] };
}

function channels(item: WatchOverviewItem): string[] {
  const out: string[] = [];
  if (item.watch.in_app) out.push(t('repo.settings.notifications.channel_in_app'));
  if (item.watch.email) out.push(t('repo.settings.notifications.channel_email'));
  return out;
}

function fail() {
  notify({ title: t('repo.watch.action_failed'), type: 'error' });
}

async function refresh() {
  items.value = await apiClient.getRepoWatches(repo.value.id);
  void reloadOverview().catch(console.error);
}

function startAdd() {
  settings.value = defaults();
  newEvent.value = 'push';
  newTarget.value = '';
  editing.value = null;
  dialog.value = 'add';
}

function startEdit(item: WatchOverviewItem) {
  settings.value = {
    in_app: item.watch.in_app,
    email: item.watch.email,
    email_events: [...(item.watch.email_events ?? [])],
  };
  editing.value = item;
  dialog.value = 'edit';
}

const { doSubmit: save, isLoading: isSaving } = useAsyncAction(async () => {
  if (dialog.value === 'edit' && editing.value) {
    await apiClient.updateWatch(editing.value.watch.id, settings.value);
  } else {
    await apiClient.createRepoWatch(repo.value.id, {
      event: newEvent.value,
      target: needsTarget.value ? newTarget.value.trim() : '',
      ...settings.value,
    });
  }
  dialog.value = null;
  await refresh();
}, fail);

const { doSubmit: doUnwatch, isLoading: isUnwatching } = useAsyncAction(async () => {
  if (!confirmUnwatch.value) return;
  await apiClient.deleteWatch(confirmUnwatch.value.watch.id);
  confirmUnwatch.value = null;
  await refresh();
}, fail);

onMounted(async () => {
  try {
    await refresh();
    suggestions.value = await apiClient.getRepoWatchSuggestions(repo.value.id);
  } catch (error) {
    console.error(error);
  } finally {
    loading.value = false;
  }
});
</script>

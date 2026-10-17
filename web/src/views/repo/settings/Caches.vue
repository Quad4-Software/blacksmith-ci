<template>
  <Settings
    :title="$t('repo.settings.caches.caches')"
    :description="$t('repo.settings.caches.desc')"
    docs-url="docs/usage/caching"
  >
    <template v-if="hasEntries" #headerActions>
      <AlertDialog>
        <AlertDialogTrigger as-child>
          <Button variant="destructive" :disabled="isDeletingAll || isDeleting" data-testid="delete-all-caches">
            <Icon v-if="isDeletingAll" name="loading" class="mr-2 h-4 w-4 animate-spin" />
            <Icon v-else name="trash" class="mr-2 h-4 w-4" />
            {{ $t('repo.settings.caches.delete_all') }}
          </Button>
        </AlertDialogTrigger>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{{ $t('repo.settings.caches.delete_all') }}</AlertDialogTitle>
            <AlertDialogDescription>
              {{ $t('repo.settings.caches.delete_all_confirm', { count: caches?.entries.length ?? 0 }) }}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
            <AlertDialogAction data-testid="confirm-delete-all-caches" @click="deleteAllCaches">
              {{ $t('delete') }}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </template>

    <!-- Loading State -->
    <div v-if="isLoading && !caches" class="text-center py-12 text-muted-foreground" data-testid="caches-loading">
      <Icon name="spinner" class="h-12 w-12 mx-auto mb-3 animate-spin" />
      <p class="text-base">{{ $t('loading') }}</p>
    </div>

    <!-- Caching disabled on the server -->
    <Alert v-else-if="caches && !caches.enabled" data-testid="caches-disabled">
      <Icon name="ph:info" class="h-4 w-4" />
      <AlertTitle>{{ $t('repo.settings.caches.disabled_title') }}</AlertTitle>
      <AlertDescription>
        <i18n-t keypath="repo.settings.caches.disabled_desc" tag="span">
          <code>{{ cacheBackendEnv }}</code>
        </i18n-t>
      </AlertDescription>
    </Alert>

    <!-- Empty State -->
    <div v-else-if="caches && !hasEntries" class="text-center py-12 text-muted-foreground" data-testid="caches-empty">
      <Icon name="cache" class="h-12 w-12 mx-auto mb-3 opacity-50" />
      <p class="text-base">{{ $t('repo.settings.caches.none') }}</p>
    </div>

    <div v-else-if="caches" class="space-y-3 text-foreground">
      <p class="text-sm text-muted-foreground" data-testid="caches-total-size">
        {{
          $t('repo.settings.caches.total_size', { size: formatCacheSize(caches.size), count: caches.entries.length })
        }}
      </p>

      <div class="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{{ $t('repo.settings.caches.key') }}</TableHead>
              <TableHead>{{ $t('repo.settings.caches.scope') }}</TableHead>
              <TableHead class="text-right">{{ $t('repo.settings.caches.size') }}</TableHead>
              <TableHead>{{ $t('repo.settings.caches.last_used') }}</TableHead>
              <TableHead class="w-12" />
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="entry in caches.entries" :key="entry.id" data-testid="cache-entry">
              <TableCell>
                <div class="flex items-center gap-2 min-w-0">
                  <span class="font-mono text-sm truncate" :title="entry.key">{{ entry.key }}</span>
                  <Badge
                    v-if="!entry.committed"
                    variant="secondary"
                    class="shrink-0"
                    :title="$t('repo.settings.caches.pending_desc')"
                  >
                    {{ $t('repo.settings.caches.pending') }}
                  </Badge>
                </div>
              </TableCell>
              <TableCell>
                <span class="flex items-center gap-1 min-w-0" :title="entry.scope">
                  <Icon :name="scopeIcon(entry.scope)" class="h-4 w-4 shrink-0 text-muted-foreground" />
                  <span class="truncate">{{ parseCacheScope(entry.scope).name }}</span>
                </span>
              </TableCell>
              <TableCell class="text-right whitespace-nowrap">{{ formatCacheSize(entry.size) }}</TableCell>
              <TableCell class="whitespace-nowrap" :title="toLocaleString(new Date(entry.last_used * 1000))">
                {{ timeAgo(entry.last_used * 1000) }}
              </TableCell>
              <TableCell class="text-right">
                <AlertDialog>
                  <AlertDialogTrigger as-child>
                    <Button
                      variant="ghost"
                      size="icon"
                      :title="$t('repo.settings.caches.delete')"
                      :disabled="isDeleting || isDeletingAll"
                    >
                      <Icon v-if="isDeleting && deletingId === entry.id" name="loading" class="h-4 w-4 animate-spin" />
                      <Icon v-else name="trash" class="h-4 w-4 text-destructive" />
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>{{ $t('repo.settings.caches.delete') }}</AlertDialogTitle>
                      <AlertDialogDescription>
                        {{ $t('repo.settings.caches.delete_confirm', { key: entry.key }) }}
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
                      <AlertDialogAction @click="deleteCache(entry)">
                        {{ $t('delete') }}
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </div>
  </Settings>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import type { IconNames } from '~/components/atomic/Icon.types';
import Settings from '~/components/layout/Settings.vue';
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '~/components/ui/alert-dialog';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import { useDate } from '~/compositions/useDate';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import type { BuildCache, RepoCaches } from '~/lib/api/types';
import { formatCacheSize, parseCacheScope } from '~/lib/build-cache';
import type { CacheScopeKind } from '~/lib/build-cache';

const cacheBackendEnv = 'CROW_CACHE_BACKEND';

const scopeIcons: Record<CacheScopeKind, IconNames> = {
  branch: 'branch',
  tag: 'tag',
  'pull-request': 'pull-request',
  other: 'commit',
};

const apiClient = useApiClient();
const notifications = useNotifications();
const i18n = useI18n();
const { timeAgo, toLocaleString } = useDate();

const repo = requiredInject('repo');
const caches = ref<RepoCaches>();
const deletingId = ref<number>();

const hasEntries = computed(() => !!caches.value?.enabled && caches.value.entries.length > 0);

function scopeIcon(scope: string): IconNames {
  return scopeIcons[parseCacheScope(scope).kind];
}

const { doSubmit: loadCaches, isLoading } = useAsyncAction(async () => {
  const result = await apiClient.getRepoCaches(repo.value.id);
  caches.value = { ...result, entries: result.entries ?? [] };
});

const { doSubmit: deleteCache, isLoading: isDeleting } = useAsyncAction(async (entry: BuildCache) => {
  deletingId.value = entry.id;
  try {
    await apiClient.deleteRepoCache(repo.value.id, entry.id);
    notifications.notify({
      title: i18n.t('notify_title_success'),
      text: i18n.t('repo.settings.caches.notify_text_deleted'),
      type: 'success',
      duration: 6000,
    });
    await loadCaches();
  } finally {
    deletingId.value = undefined;
  }
});

const { doSubmit: deleteAllCaches, isLoading: isDeletingAll } = useAsyncAction(async () => {
  await apiClient.deleteRepoCaches(repo.value.id);
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('repo.settings.caches.notify_text_all_deleted'),
    type: 'success',
    duration: 6000,
  });
  await loadCaches();
});

onMounted(loadCaches);

useCrowTitle(computed(() => [i18n.t('repo.settings.caches.caches'), repo.value.full_name]));
</script>

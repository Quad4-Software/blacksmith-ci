<template>
  <Settings :title="$t('caches.admin.title')" :description="$t('caches.admin.desc')" docs-url="docs/usage/caching">
    <template v-if="overview && canWrite" #headerActions>
      <Button variant="outline" :disabled="isCleaning" data-testid="run-cache-cleanup" @click="runCleanup">
        <Icon v-if="isCleaning" name="loading" class="mr-2 h-4 w-4 animate-spin" />
        <Icon v-else name="trash" class="mr-2 h-4 w-4" />
        {{ $t('caches.admin.cleanup') }}
      </Button>
    </template>

    <div v-if="isLoading && !overview && !disabled" class="text-center py-12 text-muted-foreground">
      <Icon name="spinner" class="h-12 w-12 mx-auto mb-3 animate-spin" />
      <p class="text-base">{{ $t('loading') }}</p>
    </div>

    <Alert v-else-if="disabled" data-testid="admin-caches-disabled">
      <Icon name="ph:info" class="h-4 w-4" />
      <AlertTitle>{{ $t('repo.settings.caches.disabled_title') }}</AlertTitle>
      <AlertDescription>
        <i18n-t keypath="repo.settings.caches.disabled_desc" tag="span">
          <code>{{ cacheBackendEnv }}</code>
        </i18n-t>
      </AlertDescription>
    </Alert>

    <div v-else-if="overview" class="space-y-6 text-foreground">
      <CacheUsageTiles
        :usage="overview.usage"
        :size="overview.size"
        :size-limit="overview.max_total_size"
        :egress-month="overview.egress_month"
        :egress-limit="0"
      />

      <div class="flex flex-wrap gap-x-6 gap-y-1 text-sm text-muted-foreground" data-testid="cache-defaults">
        <span>{{ $t('caches.admin.default_size', { value: limitLabel(overview.defaults.max_size, 'bytes') }) }}</span>
        <span>{{
          $t('caches.admin.default_egress', { value: limitLabel(overview.defaults.max_egress, 'bytes') })
        }}</span>
        <span>{{ $t('caches.admin.default_ttl', { value: limitLabel(overview.defaults.ttl, 'seconds') }) }}</span>
        <span>{{ $t('caches.admin.repo_size', { value: limitLabel(overview.max_repo_size, 'bytes') }) }}</span>
        <span v-if="overview.bucket_entries > 0">
          {{
            $t('caches.admin.bucket_total', {
              size: formatCacheSize(overview.bucket_size),
              count: overview.bucket_entries,
            })
          }}
        </span>
      </div>

      <div v-if="orgs.length === 0" class="text-center py-12 text-muted-foreground" data-testid="admin-caches-empty">
        <Icon name="cache" class="h-12 w-12 mx-auto mb-3 opacity-50" />
        <p class="text-base">{{ $t('caches.admin.none') }}</p>
      </div>

      <div v-else class="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{{ $t('caches.admin.owner') }}</TableHead>
              <TableHead class="text-right">{{ $t('caches.size') }}</TableHead>
              <TableHead class="text-right">{{ $t('caches.entries') }}</TableHead>
              <TableHead class="text-right">{{ $t('caches.hit_rate') }}</TableHead>
              <TableHead class="text-right">{{ $t('caches.egress_month') }}</TableHead>
              <TableHead>{{ $t('caches.retention') }}</TableHead>
              <TableHead class="w-28" />
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="org in orgs" :key="org.org_id" data-testid="cache-org-row">
              <TableCell>
                <div class="flex items-center gap-2 min-w-0">
                  <Icon :name="org.is_user ? 'user' : 'org'" class="h-4 w-4 shrink-0 text-muted-foreground" />
                  <span class="truncate font-medium">{{
                    org.name || $t('caches.admin.deleted_org', { id: org.org_id })
                  }}</span>
                  <Badge v-if="org.bucket" variant="secondary" class="shrink-0" :title="bucketTitle(org)">
                    <Icon name="ph:plug" class="mr-1 h-3 w-3" />
                    {{ $t('caches.own_bucket') }}
                  </Badge>
                  <Badge v-if="hasCustomSettings(org)" variant="outline" class="shrink-0">
                    {{ $t('caches.admin.custom_limits') }}
                  </Badge>
                </div>
              </TableCell>
              <TableCell class="text-right whitespace-nowrap">
                <div :class="{ 'text-destructive font-medium': overLimit(org.size, org.limits.max_size) }">
                  {{ formatCacheSize(org.size) }}
                </div>
                <div class="text-xs text-muted-foreground">
                  {{ org.limits.max_size > 0 ? $t('caches.of', { limit: formatCacheSize(org.limits.max_size) }) : '' }}
                  <span v-if="org.bucket_size > 0" class="block">
                    {{ $t('caches.admin.in_bucket', { size: formatCacheSize(org.bucket_size) }) }}
                  </span>
                </div>
              </TableCell>
              <TableCell class="text-right">{{ org.entries + org.bucket_entries }}</TableCell>
              <TableCell class="text-right whitespace-nowrap" :title="restoreTitle(org)">
                {{ formatHitRate(cacheHitRate(org.usage)) }}
              </TableCell>
              <TableCell class="text-right whitespace-nowrap">
                <div :class="{ 'text-destructive font-medium': overLimit(org.egress_month, org.limits.max_egress) }">
                  <Icon
                    v-if="overLimit(org.egress_month, org.limits.max_egress)"
                    name="alert"
                    class="inline h-3 w-3"
                    :title="$t('caches.over_limit')"
                  />
                  {{ formatCacheSize(org.egress_month) }}
                </div>
                <div v-if="org.limits.max_egress > 0" class="text-xs text-muted-foreground">
                  {{ $t('caches.of', { limit: formatCacheSize(org.limits.max_egress) }) }}
                </div>
                <div v-if="org.bucket_egress_month > 0" class="text-xs text-muted-foreground">
                  {{ $t('caches.admin.bucket_egress', { size: formatCacheSize(org.bucket_egress_month) }) }}
                </div>
              </TableCell>
              <TableCell class="whitespace-nowrap">{{ limitLabel(org.limits.ttl, 'seconds') }}</TableCell>
              <TableCell class="text-right whitespace-nowrap">
                <Button variant="ghost" size="icon" :title="$t('caches.admin.details')" @click="openDetails(org)">
                  <Icon name="graph" class="h-4 w-4" />
                </Button>
                <Button
                  v-if="canWrite && org.org_id > 0"
                  variant="ghost"
                  size="icon"
                  :title="$t('caches.admin.edit_limits')"
                  data-testid="edit-cache-limits"
                  @click="openLimits(org)"
                >
                  <Icon name="edit" class="h-4 w-4" />
                </Button>
                <Button
                  v-if="canWrite && org.org_id > 0"
                  variant="ghost"
                  size="icon"
                  :title="$t('caches.purge')"
                  :disabled="org.entries + org.bucket_entries === 0"
                  @click="purgeTarget = org"
                >
                  <Icon name="trash" class="h-4 w-4 text-destructive" />
                </Button>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </div>

    <!-- Limits of one org -->
    <Dialog :open="!!limitsTarget" @update:open="(open) => !open && (limitsTarget = undefined)">
      <DialogContent v-if="limitsTarget && overview">
        <DialogHeader>
          <DialogTitle>{{ $t('caches.admin.limits_of', { name: limitsTarget.name }) }}</DialogTitle>
          <DialogDescription>{{ $t('caches.admin.limits_desc') }}</DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <CacheLimitField
            id="cache-max-size"
            v-model="limitsForm.max_size"
            :label="$t('caches.limit.max_size')"
            kind="bytes"
            :default-value="overview.defaults.max_size"
            :hint="$t('caches.limit.max_size_hint')"
          />
          <CacheLimitField
            id="cache-max-egress"
            v-model="limitsForm.max_egress"
            :label="$t('caches.limit.max_egress')"
            kind="bytes"
            :default-value="overview.defaults.max_egress"
            :hint="$t('caches.limit.max_egress_hint')"
          />
          <CacheLimitField
            id="cache-ttl"
            v-model="limitsForm.ttl"
            :label="$t('caches.retention')"
            kind="seconds"
            :default-value="overview.defaults.ttl"
            :hint="$t('caches.limit.ttl_hint')"
          />
        </div>
        <DialogFooter>
          <Button variant="outline" @click="limitsTarget = undefined">{{ $t('cancel') }}</Button>
          <Button :disabled="isSaving" data-testid="save-cache-limits" @click="saveLimits">
            <Icon v-if="isSaving" name="loading" class="mr-2 h-4 w-4 animate-spin" />
            {{ $t('save') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Details of one org -->
    <Sheet :open="!!detailsTarget" @update:open="(open) => !open && closeDetails()">
      <SheetContent class="w-full sm:max-w-2xl overflow-y-auto">
        <SheetHeader>
          <SheetTitle>{{ detailsTarget?.name }}</SheetTitle>
          <SheetDescription>{{ $t('caches.last_30_days') }}</SheetDescription>
        </SheetHeader>
        <div v-if="isLoadingDetails && !details" class="py-12 text-center text-muted-foreground">
          <Icon name="spinner" class="h-8 w-8 mx-auto animate-spin" />
        </div>
        <div v-else-if="details" class="space-y-6 px-4 pb-6">
          <CacheDailyChart :daily="details.daily ?? []" :days="30" />
          <CacheRepoTable :repos="details.repo_list" />
        </div>
      </SheetContent>
    </Sheet>

    <!-- Purge one org -->
    <AlertDialog :open="!!purgeTarget" @update:open="(open) => !open && (purgeTarget = undefined)">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('caches.purge') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{
              $t('caches.purge_confirm', {
                name: purgeTarget?.name,
                count: (purgeTarget?.entries ?? 0) + (purgeTarget?.bucket_entries ?? 0),
              })
            }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
          <AlertDialogAction :disabled="isPurging" data-testid="confirm-purge-org-caches" @click="purge">
            <Icon v-if="isPurging" name="loading" class="mr-2 h-4 w-4 animate-spin" />
            {{ $t('delete') }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </Settings>
</template>

<script lang="ts" setup>
import { computed, onMounted, reactive, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import CacheDailyChart from '~/components/caches/CacheDailyChart.vue';
import CacheLimitField from '~/components/caches/CacheLimitField.vue';
import CacheRepoTable from '~/components/caches/CacheRepoTable.vue';
import CacheUsageTiles from '~/components/caches/CacheUsageTiles.vue';
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
} from '~/components/ui/alert-dialog';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '~/components/ui/sheet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import useNotifications from '~/compositions/useNotifications';
import usePermissions from '~/compositions/usePermissions';
import type {
  BuildCacheCleanupResult,
  BuildCacheOrgDetail,
  BuildCacheOrgSummary,
  BuildCacheOverview,
} from '~/lib/api/types';
import { cacheHitRate, formatCacheSize, formatCacheTTL, formatHitRate } from '~/lib/build-cache';

const cacheBackendEnv = 'CROW_CACHE_BACKEND';

const apiClient = useApiClient();
const notifications = useNotifications();
const i18n = useI18n();
const { can } = usePermissions();

const canWrite = computed(() => can('maintenance:write'));
const overview = ref<BuildCacheOverview>();
const disabled = ref(false);
const orgs = computed(() => overview.value?.orgs ?? []);

function limitLabel(value: number, kind: 'bytes' | 'seconds'): string {
  if (value <= 0) return kind === 'bytes' ? i18n.t('caches.unlimited') : i18n.t('caches.limit.keep');
  return kind === 'bytes' ? formatCacheSize(value) : formatCacheTTL(value);
}

// overLimit reports whether used reached a limit, 0 being unlimited.
function overLimit(used: number, limit: number): boolean {
  return limit > 0 && used >= limit;
}

function hasCustomSettings(org: BuildCacheOrgSummary): boolean {
  return org.settings.max_size !== 0 || org.settings.max_egress !== 0 || org.settings.ttl !== 0;
}

function bucketTitle(org: BuildCacheOrgSummary): string {
  return org.bucket ? `${org.bucket.endpoint}/${org.bucket.bucket}` : '';
}

function restoreTitle(org: BuildCacheOrgSummary): string {
  return i18n.t('caches.restore_breakdown', {
    hits: org.usage.hits,
    partial: org.usage.partial_hits,
    misses: org.usage.misses,
  });
}

function notifyCleanup(result: BuildCacheCleanupResult) {
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('caches.notify_removed', { count: result.removed, size: formatCacheSize(result.freed) }),
    type: 'success',
    duration: 6000,
  });
}

const { doSubmit: loadOverview, isLoading } = useAsyncAction(async () => {
  const result = await apiClient.getAdminCaches();
  disabled.value = !result.enabled;
  overview.value = result.overview;
});

const { doSubmit: runCleanup, isLoading: isCleaning } = useAsyncAction(async () => {
  notifyCleanup(await apiClient.runAdminCacheCleanup());
  await loadOverview();
});

const limitsTarget = ref<BuildCacheOrgSummary>();
const limitsForm = reactive({ max_size: 0, max_egress: 0, ttl: 0 });

function openLimits(org: BuildCacheOrgSummary) {
  limitsForm.max_size = org.settings.max_size;
  limitsForm.max_egress = org.settings.max_egress;
  limitsForm.ttl = org.settings.ttl;
  limitsTarget.value = org;
}

const { doSubmit: saveLimits, isLoading: isSaving } = useAsyncAction(async () => {
  if (!limitsTarget.value) return;
  await apiClient.putAdminCacheOrg(limitsTarget.value.org_id, { ...limitsForm });
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('caches.admin.notify_limits_saved'),
    type: 'success',
    duration: 6000,
  });
  limitsTarget.value = undefined;
  await loadOverview();
});

const purgeTarget = ref<BuildCacheOrgSummary>();
const { doSubmit: purge, isLoading: isPurging } = useAsyncAction(async () => {
  if (!purgeTarget.value) return;
  notifyCleanup(await apiClient.deleteAdminCacheOrg(purgeTarget.value.org_id));
  purgeTarget.value = undefined;
  await loadOverview();
});

const detailsTarget = ref<BuildCacheOrgSummary>();
const details = ref<BuildCacheOrgDetail>();
const { doSubmit: loadDetails, isLoading: isLoadingDetails } = useAsyncAction(async (orgId: number) => {
  details.value = await apiClient.getAdminCacheOrg(orgId);
});

async function openDetails(org: BuildCacheOrgSummary) {
  details.value = undefined;
  detailsTarget.value = org;
  await loadDetails(org.org_id);
}

function closeDetails() {
  detailsTarget.value = undefined;
  details.value = undefined;
}

onMounted(loadOverview);

useCrowTitle(computed(() => [i18n.t('caches.admin.title')]));
</script>

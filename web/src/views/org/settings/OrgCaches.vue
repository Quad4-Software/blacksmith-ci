<template>
  <Settings :title="$t('caches.org.title')" :description="$t('caches.org.desc')" docs-url="docs/usage/caching">
    <template v-if="detail && detail.entries + detail.bucket_entries > 0" #headerActions>
      <AlertDialog>
        <AlertDialogTrigger as-child>
          <Button variant="destructive" :disabled="isPurging" data-testid="purge-org-caches">
            <Icon v-if="isPurging" name="loading" class="mr-2 h-4 w-4 animate-spin" />
            <Icon v-else name="trash" class="mr-2 h-4 w-4" />
            {{ $t('caches.purge') }}
          </Button>
        </AlertDialogTrigger>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{{ $t('caches.purge') }}</AlertDialogTitle>
            <AlertDialogDescription>
              {{ $t('caches.purge_confirm', { name: org.name, count: detail.entries + detail.bucket_entries }) }}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
            <AlertDialogAction @click="purge">{{ $t('delete') }}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </template>

    <div v-if="isLoading && !caches" class="text-center py-12 text-muted-foreground">
      <Icon name="spinner" class="h-12 w-12 mx-auto mb-3 animate-spin" />
      <p class="text-base">{{ $t('loading') }}</p>
    </div>

    <Alert v-else-if="caches && !caches.enabled" data-testid="org-caches-disabled">
      <Icon name="ph:info" class="h-4 w-4" />
      <AlertTitle>{{ $t('repo.settings.caches.disabled_title') }}</AlertTitle>
      <AlertDescription>
        <i18n-t keypath="repo.settings.caches.disabled_desc" tag="span">
          <code>{{ cacheBackendEnv }}</code>
        </i18n-t>
      </AlertDescription>
    </Alert>

    <div v-else-if="detail" class="space-y-6 text-foreground">
      <CacheUsageTiles
        :usage="detail.usage"
        :size="detail.bucket ? detail.bucket_size : detail.size"
        :size-limit="detail.bucket ? detail.bucket.max_size : detail.limits.max_size"
        :egress-month="detail.bucket ? detail.bucket_egress_month : detail.egress_month"
        :egress-limit="detail.bucket ? detail.bucket.max_egress : detail.limits.max_egress"
      />

      <p class="text-sm text-muted-foreground" data-testid="org-cache-retention">
        {{ $t('caches.org.retention', { value: retentionLabel }) }}
        <span v-if="detail.bucket">{{ $t('caches.org.limits_by_owner') }}</span>
        <span v-else>{{ $t('caches.org.limits_by_admin') }}</span>
      </p>

      <section class="space-y-3">
        <h3 class="text-base font-medium">{{ $t('caches.last_30_days') }}</h3>
        <CacheDailyChart :daily="detail.daily ?? []" :days="30" />
      </section>

      <section v-if="detail.repo_list.length > 0" class="space-y-3">
        <h3 class="text-base font-medium">{{ $t('caches.org.by_repo') }}</h3>
        <CacheRepoTable :repos="detail.repo_list" />
      </section>

      <!-- Own bucket -->
      <section v-if="caches?.allow_buckets || detail.bucket" class="space-y-3" data-testid="org-cache-bucket">
        <div class="flex items-center justify-between gap-2">
          <h3 class="text-base font-medium">{{ $t('caches.bucket.title') }}</h3>
          <Badge v-if="detail.bucket" variant="secondary">
            <Icon name="ph:plug" class="mr-1 h-3 w-3" />
            {{ $t('caches.bucket.connected') }}
          </Badge>
        </div>
        <p class="text-sm text-muted-foreground">{{ $t('caches.bucket.desc') }}</p>
        <Alert>
          <Icon name="alert" class="h-4 w-4" />
          <AlertDescription>{{ $t('caches.bucket.move_warning') }}</AlertDescription>
        </Alert>

        <form class="grid grid-cols-1 md:grid-cols-2 gap-4" @submit.prevent="saveBucket">
          <div class="space-y-2">
            <Label for="bucket-endpoint">{{ $t('caches.bucket.endpoint') }}</Label>
            <Input id="bucket-endpoint" v-model="bucketForm.endpoint" required placeholder="s3.amazonaws.com" />
          </div>
          <div class="space-y-2">
            <Label for="bucket-name">{{ $t('caches.bucket.bucket') }}</Label>
            <Input id="bucket-name" v-model="bucketForm.bucket" required />
          </div>
          <div class="space-y-2">
            <Label for="bucket-region">{{ $t('caches.bucket.region') }}</Label>
            <Input id="bucket-region" v-model="bucketForm.region" />
          </div>
          <div class="space-y-2">
            <Label for="bucket-prefix">{{ $t('caches.bucket.prefix') }}</Label>
            <Input id="bucket-prefix" v-model="bucketForm.prefix" placeholder="crow-cache" />
          </div>
          <div class="space-y-2">
            <Label for="bucket-access-key">{{ $t('caches.bucket.access_key') }}</Label>
            <Input id="bucket-access-key" v-model="bucketForm.access_key" autocomplete="off" />
          </div>
          <div class="space-y-2">
            <Label for="bucket-secret-key">{{ $t('caches.bucket.secret_key') }}</Label>
            <Input
              id="bucket-secret-key"
              v-model="bucketForm.secret_key"
              type="password"
              autocomplete="new-password"
              :placeholder="detail.bucket ? $t('caches.bucket.secret_keep') : ''"
            />
          </div>
          <div class="flex items-center gap-2">
            <Checkbox
              id="bucket-ssl"
              :checked="bucketForm.ssl"
              @update:checked="(v) => (bucketForm.ssl = v === true)"
            />
            <Label for="bucket-ssl">{{ $t('caches.bucket.ssl') }}</Label>
          </div>
          <div class="flex items-center gap-2">
            <Checkbox
              id="bucket-path-style"
              :checked="bucketForm.path_style"
              @update:checked="(v) => (bucketForm.path_style = v === true)"
            />
            <Label for="bucket-path-style">{{ $t('caches.bucket.path_style') }}</Label>
          </div>
          <CacheLimitField
            id="bucket-max-size"
            v-model="bucketForm.max_size"
            :label="$t('caches.limit.max_size')"
            kind="bytes"
            :default-value="0"
            no-default
            :hint="$t('caches.limit.max_size_hint')"
          />
          <CacheLimitField
            id="bucket-max-egress"
            v-model="bucketForm.max_egress"
            :label="$t('caches.limit.max_egress')"
            kind="bytes"
            :default-value="0"
            no-default
            :hint="$t('caches.limit.max_egress_hint')"
          />
          <div class="md:col-span-2">
            <CacheLimitField
              id="bucket-ttl"
              v-model="bucketForm.ttl"
              :label="$t('caches.retention')"
              kind="seconds"
              :default-value="serverTTL"
              :hint="$t('caches.bucket.ttl_hint')"
            />
          </div>
          <div class="md:col-span-2 flex flex-wrap gap-2">
            <Button type="submit" :disabled="isSavingBucket || !caches?.allow_buckets" data-testid="save-cache-bucket">
              <Icon v-if="isSavingBucket" name="loading" class="mr-2 h-4 w-4 animate-spin" />
              {{ detail.bucket ? $t('caches.bucket.update') : $t('caches.bucket.connect') }}
            </Button>
            <AlertDialog v-if="detail.bucket">
              <AlertDialogTrigger as-child>
                <Button type="button" variant="outline" :disabled="isDisconnecting">
                  <Icon v-if="isDisconnecting" name="loading" class="mr-2 h-4 w-4 animate-spin" />
                  {{ $t('caches.bucket.disconnect') }}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{{ $t('caches.bucket.disconnect') }}</AlertDialogTitle>
                  <AlertDialogDescription>{{ $t('caches.bucket.disconnect_confirm') }}</AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
                  <AlertDialogAction data-testid="confirm-disconnect-bucket" @click="disconnectBucket">
                    {{ $t('caches.bucket.disconnect') }}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
          <p v-if="bucketError" class="md:col-span-2 text-sm text-destructive" data-testid="bucket-error">
            {{ bucketError }}
          </p>
        </form>
      </section>
    </div>
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
  AlertDialogTrigger,
} from '~/components/ui/alert-dialog';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import type { BuildCacheBucketInput, OrgCaches } from '~/lib/api/types';
import { formatCacheSize, formatCacheTTL } from '~/lib/build-cache';

const cacheBackendEnv = 'CROW_CACHE_BACKEND';

const apiClient = useApiClient();
const notifications = useNotifications();
const i18n = useI18n();

const org = requiredInject('org');
const caches = ref<OrgCaches>();
const detail = computed(() => caches.value?.detail);

// The server default retention, which a bucket without a retention of its own uses.
const serverTTL = computed(() => caches.value?.defaults.ttl ?? 0);

const retentionLabel = computed(() => {
  const d = detail.value;
  if (!d) return '';
  const ttl = d.bucket ? (d.bucket.ttl === 0 ? serverTTL.value : Math.max(d.bucket.ttl, 0)) : d.limits.ttl;
  return ttl > 0 ? formatCacheTTL(ttl) : i18n.t('caches.limit.keep');
});

function emptyBucket(): BuildCacheBucketInput {
  return {
    endpoint: '',
    bucket: '',
    region: '',
    prefix: '',
    access_key: '',
    secret_key: '',
    ssl: true,
    path_style: false,
    ttl: 0,
    max_size: 0,
    max_egress: 0,
  };
}

const bucketForm = reactive<BuildCacheBucketInput>(emptyBucket());
const bucketError = ref('');

function fillBucketForm() {
  const bucket = detail.value?.bucket;
  Object.assign(
    bucketForm,
    bucket
      ? {
          endpoint: bucket.endpoint,
          bucket: bucket.bucket,
          region: bucket.region,
          prefix: bucket.prefix,
          access_key: bucket.access_key,
          secret_key: '',
          ssl: bucket.ssl,
          path_style: bucket.path_style,
          ttl: bucket.ttl,
          max_size: bucket.max_size,
          max_egress: bucket.max_egress,
        }
      : emptyBucket(),
  );
}

const { doSubmit: loadCaches, isLoading } = useAsyncAction(async () => {
  caches.value = await apiClient.getOrgCaches(org.value.id);
  fillBucketForm();
});

const { doSubmit: purge, isLoading: isPurging } = useAsyncAction(async () => {
  const result = await apiClient.deleteOrgCaches(org.value.id);
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('caches.notify_removed', { count: result.removed, size: formatCacheSize(result.freed) }),
    type: 'success',
    duration: 6000,
  });
  await loadCaches();
});

const { doSubmit: saveBucket, isLoading: isSavingBucket } = useAsyncAction(async () => {
  bucketError.value = '';
  try {
    await apiClient.putOrgCacheBucket(org.value.id, { ...bucketForm });
  } catch (error) {
    // Keep the reason next to the form, e.g. that the bucket could not be reached.
    bucketError.value = error instanceof Error ? error.message : String(error);
    return;
  }
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('caches.bucket.notify_saved'),
    type: 'success',
    duration: 6000,
  });
  await loadCaches();
});

const { doSubmit: disconnectBucket, isLoading: isDisconnecting } = useAsyncAction(async () => {
  await apiClient.deleteOrgCacheBucket(org.value.id);
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('caches.bucket.notify_disconnected'),
    type: 'success',
    duration: 6000,
  });
  await loadCaches();
});

onMounted(loadCaches);

useCrowTitle(computed(() => [i18n.t('caches.org.title'), org.value.name]));
</script>

<template>
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3" data-testid="cache-usage-tiles">
    <div class="rounded-md border p-4 space-y-1">
      <p class="text-sm text-muted-foreground">{{ $t('caches.hit_rate') }}</p>
      <p class="text-2xl font-semibold" data-testid="cache-hit-rate">{{ formatHitRate(hitRate) }}</p>
      <p class="text-xs text-muted-foreground">
        {{
          $t('caches.restore_breakdown', {
            hits: usage.hits,
            partial: usage.partial_hits,
            misses: usage.misses,
          })
        }}
        <span v-if="usage.denied > 0" class="block">{{ $t('caches.denied_count', { count: usage.denied }) }}</span>
      </p>
    </div>

    <div class="rounded-md border p-4 space-y-1">
      <p class="text-sm text-muted-foreground">{{ $t('caches.size') }}</p>
      <p class="text-2xl font-semibold">{{ formatCacheSize(size) }}</p>
      <LimitMeter :used="size" :limit="sizeLimit" />
    </div>

    <div class="rounded-md border p-4 space-y-1">
      <p class="text-sm text-muted-foreground">{{ $t('caches.egress_month') }}</p>
      <p class="text-2xl font-semibold" data-testid="cache-egress-month">{{ formatCacheSize(egressMonth) }}</p>
      <LimitMeter :used="egressMonth" :limit="egressLimit" />
    </div>

    <div class="rounded-md border p-4 space-y-1">
      <p class="text-sm text-muted-foreground">{{ $t('caches.traffic_30d') }}</p>
      <p class="text-sm">
        <Icon name="ph:arrow-down" class="inline h-4 w-4 text-muted-foreground" />
        {{ $t('caches.egress_value', { size: formatCacheSize(usage.egress) }) }}
      </p>
      <p class="text-sm">
        <Icon name="ph:arrow-up" class="inline h-4 w-4 text-muted-foreground" />
        {{ $t('caches.ingress_value', { size: formatCacheSize(usage.ingress) }) }}
      </p>
      <p class="text-xs text-muted-foreground">{{ $t('caches.saves_count', { count: usage.saves }) }}</p>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import LimitMeter from '~/components/caches/LimitMeter.vue';
import type { BuildCacheUsageCounts } from '~/lib/api/types';
import { cacheHitRate, formatCacheSize, formatHitRate } from '~/lib/build-cache';

const props = defineProps<{
  usage: BuildCacheUsageCounts;
  size: number;
  // 0 is unlimited.
  sizeLimit: number;
  egressMonth: number;
  egressLimit: number;
}>();

const hitRate = computed(() => cacheHitRate(props.usage));
</script>

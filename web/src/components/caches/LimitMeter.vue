<template>
  <div v-if="share !== undefined" class="space-y-1">
    <div class="h-1.5 w-full rounded-full bg-muted overflow-hidden">
      <div
        class="h-full rounded-full"
        :class="share >= 0.9 ? 'bg-destructive' : 'bg-primary'"
        :style="{ width: `${Math.min(share, 1) * 100}%` }"
      />
    </div>
    <p class="text-xs text-muted-foreground">
      <Icon v-if="share >= 0.9" name="alert" class="inline h-3 w-3" />
      {{ $t('caches.of_limit', { percent: Math.round(share * 100), limit: formatCacheSize(limit) }) }}
    </p>
  </div>
  <p v-else class="text-xs text-muted-foreground">{{ $t('caches.unlimited') }}</p>
</template>

<script lang="ts" setup>
import { computed } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import { formatCacheSize, limitUsage } from '~/lib/build-cache';

const props = defineProps<{
  used: number;
  // 0 is unlimited.
  limit: number;
}>();

const share = computed(() => limitUsage(props.used, props.limit));
</script>

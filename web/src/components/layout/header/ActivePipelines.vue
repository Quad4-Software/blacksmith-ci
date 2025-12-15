<template>
  <Button
    variant="ghost"
    size="icon"
    class="relative"
    :title="pipelineCount > 0 ? `${$t('pipeline_feed')} (${pipelineCount})` : $t('pipeline_feed')"
    @click="toggle"
  >
    <Icon name="lucide:list-todo" class="w-4 h-4" />
    <Badge
      v-if="pipelineCount > 0"
      variant="default"
      class="absolute -right-1 -top-1 flex h-4 min-w-4 items-center justify-center rounded-full px-0.5 text-[10px] font-semibold tabular-nums"
    >
      {{ displayCount }}
    </Badge>
  </Button>
</template>

<script lang="ts" setup>
import { computed, onMounted, toRef } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import usePipelineFeed from '~/compositions/usePipelineFeed';

const pipelineFeed = usePipelineFeed();
const activePipelines = toRef(pipelineFeed, 'activePipelines');
const { toggle } = pipelineFeed;
const pipelineCount = computed(() => activePipelines.value.length || 0);
const displayCount = computed(() => (pipelineCount.value > 9 ? `${9}+` : pipelineCount.value));

onMounted(async () => {
  await pipelineFeed.load();
});
</script>

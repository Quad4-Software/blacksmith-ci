<template>
  <IconButton
    :title="pipelineCount > 0 ? `${$t('pipeline_feed')} (${pipelineCount})` : $t('pipeline_feed')"
    class="active-pipelines-toggle relative p-1.5! text-current"
    @click="toggle"
  >
    <div v-if="pipelineCount > 0" class="spinner" />
    <div
      class="z-0 flex h-full w-full items-center justify-center border border-wp-custom-highlight-100 rounded-md bg-white/10 font-bold"
    >
      <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
      {{ pipelineCount > 9 ? '9+' : pipelineCount }}
    </div>
  </IconButton>
</template>

<script lang="ts" setup>
import { computed, onMounted, toRef } from 'vue';

import IconButton from '~/components/atomic/IconButton.vue';
import usePipelineFeed from '~/compositions/usePipelineFeed';

const pipelineFeed = usePipelineFeed();
const activePipelines = toRef(pipelineFeed, 'activePipelines');
const { toggle } = pipelineFeed;
const pipelineCount = computed(() => activePipelines.value.length || 0);

onMounted(async () => {
  await pipelineFeed.load();
});
</script>

<style scoped>
@keyframes rotate {
  100% {
    transform: rotate(1turn);
  }
}
.spinner {
  position: absolute;
  inset: 0.375rem;
  border-radius: 0.375rem;
  overflow: hidden;
}
.spinner::before {
  background-color: var(--wp-custom-highlight-100);
  position: absolute;
  content: '';
  left: -50%;
  top: -50%;
  width: 200%;
  height: 200%;
  background-repeat: no-repeat;
  background-size:
    50% 50%,
    50% 50%;
  background-image: linear-gradient(#fff, #fff);
  animation: rotate 1.5s linear infinite;
}
.spinner::after {
  background-color: var(--wp-custom-highlight-100);
  position: absolute;
  inset: 0.125rem;
  /*
  The nested border radius needs to be calculated correctly to look right:
  https://www.30secondsofcode.org/css/s/nested-border-radius/
  */
  border-radius: calc(0.375rem - 0.125rem);
  content: '';
}
</style>

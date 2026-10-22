<template>
  <div class="flex flex-col items-center gap-3 text-center">
    <template v-if="workflow.autoscaler_provisioning_stale">
      <Icon name="ph:warning" class="w-10 h-10 text-destructive" />
      <span class="text-lg font-medium text-destructive">{{ $t('repo.pipeline.agent_provisioning_stale') }}</span>
      <span class="text-sm text-muted-foreground">
        {{ $t('repo.pipeline.agent_provisioning_stale_desc', { expected: expectedMinutes }) }}
      </span>
    </template>
    <template v-else>
      <Icon name="ph:hard-drives" class="w-10 h-10 text-primary animate-pulse" />
      <span class="text-lg font-medium text-foreground">{{ $t('repo.pipeline.agent_provisioning') }}</span>
      <span class="text-sm text-muted-foreground">{{ $t('repo.pipeline.agent_provisioning_desc') }}</span>
    </template>
    <span v-if="elapsedMinutes !== undefined" class="text-sm text-muted-foreground" data-testid="provisioning-elapsed">
      {{ $t('repo.pipeline.agent_provisioning_elapsed', { elapsed: elapsedMinutes, expected: expectedMinutes }) }}
    </span>

    <div
      v-for="agent in agentsWithPhases"
      :key="agent.id"
      class="mt-2 flex flex-col gap-1.5 text-left text-sm"
      data-testid="provisioning-agent"
    >
      <span class="font-mono text-xs text-muted-foreground">{{ agent.name }}</span>
      <div
        v-for="(phase, index) in agent.phases"
        :key="`${phase.at}-${index}`"
        class="flex items-center gap-2"
        :class="agent.states[index] === 'done' ? 'text-muted-foreground' : 'text-foreground'"
      >
        <Icon v-if="agent.states[index] === 'done'" name="ph:check" class="h-4 w-4 shrink-0 text-primary" />
        <Icon v-else-if="agent.states[index] === 'failed'" name="ph:x" class="h-4 w-4 shrink-0 text-destructive" />
        <Icon v-else name="spinner" class="h-4 w-4 shrink-0 animate-spin text-primary" />
        <span :class="{ 'text-destructive': agent.states[index] === 'failed' }">{{ phase.phase }}</span>
        <span class="ml-auto pl-4 text-xs tabular-nums text-muted-foreground">{{ clock(phase.at) }}</span>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import Icon from '~/components/atomic/Icon.vue';
import type { PipelineWorkflow } from '~/lib/api/types';
import { provisioningPhaseStates, wholeMinutes } from '~/lib/provisioning';
import { usePipelineStore } from '~/store/pipelines';
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';

const props = defineProps<{
  workflow: PipelineWorkflow;
  repoId: number;
  pipelineNumber: number;
}>();

// Hosts report phases without touching the pipeline, so nothing pushes them
// to the browser; poll while the card is shown.
const REFRESH_INTERVAL_MS = 15_000;

const pipelineStore = usePipelineStore();
const now = ref(Date.now());
let ticker: ReturnType<typeof setInterval> | undefined;
let refresher: ReturnType<typeof setInterval> | undefined;

onMounted(() => {
  ticker = setInterval(() => {
    now.value = Date.now();
  }, 1000);
  refresher = setInterval(() => {
    pipelineStore.loadPipeline(props.repoId, props.pipelineNumber).catch(() => {});
  }, REFRESH_INTERVAL_MS);
});

onBeforeUnmount(() => {
  clearInterval(ticker);
  clearInterval(refresher);
});

const expectedMinutes = computed(() => wholeMinutes(props.workflow.autoscaler_provision_timeout ?? 300));
const elapsedMinutes = computed(() => {
  const since = props.workflow.autoscaler_provisioning_since;
  if (!since) return undefined;
  return wholeMinutes(Math.max(0, now.value / 1000 - since));
});

const agentsWithPhases = computed(() =>
  (props.workflow.provisioning_agents ?? [])
    .filter((agent) => agent.phases.length > 0)
    .map((agent) => ({ ...agent, states: provisioningPhaseStates(agent.phases) })),
);

function clock(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}
</script>

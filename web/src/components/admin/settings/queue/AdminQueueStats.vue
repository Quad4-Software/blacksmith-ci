<template>
  <div v-if="stats || agentStats.length > 0" class="space-y-4">
    <!-- Overall Queue Stats -->
    <Card v-if="stats">
      <div class="p-4">
        <h3 class="text-lg font-semibold mb-4">{{ $t('admin.settings.queue.stats.overview') }}</h3>
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
          <div class="flex flex-col">
            <span class="text-xs font-medium text-muted-foreground mb-1">{{
              $t('admin.settings.queue.stats.worker_count')
            }}</span>
            <span class="text-2xl font-bold text-green-600 dark:text-green-400">{{ stats.worker_count }}</span>
          </div>
          <div class="flex flex-col">
            <span class="text-xs font-medium text-muted-foreground mb-1">{{
              $t('admin.settings.queue.stats.running_count')
            }}</span>
            <span class="text-2xl font-bold text-blue-600 dark:text-blue-400">{{ stats.running_count }}</span>
          </div>
          <div class="flex flex-col">
            <span class="text-xs font-medium text-muted-foreground mb-1">{{
              $t('admin.settings.queue.stats.pending_count')
            }}</span>
            <span class="text-2xl font-bold text-gray-600 dark:text-gray-400">{{ stats.pending_count }}</span>
          </div>
          <div class="flex flex-col">
            <span class="text-xs font-medium text-muted-foreground mb-1">{{
              $t('admin.settings.queue.stats.waiting_on_deps_count')
            }}</span>
            <span class="text-2xl font-bold text-orange-600 dark:text-orange-400">{{
              stats.waiting_on_deps_count
            }}</span>
          </div>
        </div>

        <!-- Progress bar -->
        <div v-if="total > 0" class="mt-4">
          <div class="flex h-2 overflow-hidden rounded-full bg-muted">
            <div
              v-for="item in progressData"
              :key="item.key"
              class="h-full transition-all duration-300"
              :class="item.color"
              :style="{ width: `${item.percentage}%` }"
              :title="`${item.label}: ${item.value}`"
            />
          </div>
        </div>
      </div>
    </Card>

    <!-- Per-Agent Breakdown -->
    <div v-if="agentStats.length > 0">
      <h3 class="text-lg font-semibold mb-3">{{ $t('admin.settings.queue.stats.by_agent') }}</h3>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
        <router-link
          v-for="agent in agentStats"
          :key="agent.name"
          :to="{ name: 'agents', query: { search: agent.name } }"
          class="block"
        >
          <Card class="hover:shadow-md hover:border-primary/50 transition-all cursor-pointer h-full">
            <div class="p-4">
              <div class="flex items-center gap-2 mb-3">
                <div
                  class="w-2 h-2 rounded-full"
                  :class="agent.running > 0 ? 'bg-green-500 animate-pulse' : 'bg-gray-400'"
                />
                <span class="font-medium truncate">{{ agent.name }}</span>
                <Icon name="chevron-right" class="h-4 w-4 text-muted-foreground ml-auto shrink-0" />
              </div>
              <div class="grid grid-cols-3 gap-2 text-center">
                <div>
                  <div class="text-lg font-bold text-blue-600 dark:text-blue-400">{{ agent.running }}</div>
                  <div class="text-xs text-muted-foreground">{{ $t('admin.settings.queue.running') }}</div>
                </div>
                <div>
                  <div class="text-lg font-bold text-gray-600 dark:text-gray-400">{{ agent.pending }}</div>
                  <div class="text-xs text-muted-foreground">{{ $t('admin.settings.queue.pending') }}</div>
                </div>
                <div>
                  <div class="text-lg font-bold text-orange-600 dark:text-orange-400">{{ agent.waiting }}</div>
                  <div class="text-xs text-muted-foreground">{{ $t('admin.settings.queue.waiting') }}</div>
                </div>
              </div>
            </div>
          </Card>
        </router-link>
      </div>
    </div>

    <!-- Unassigned tasks card -->
    <Card v-if="unassignedStats.total > 0" class="border-dashed">
      <div class="p-4">
        <div class="flex items-center gap-2 mb-3">
          <div class="w-2 h-2 rounded-full bg-gray-400" />
          <span class="font-medium text-muted-foreground">{{ $t('admin.settings.queue.unassigned') }}</span>
        </div>
        <div class="grid grid-cols-3 gap-2 text-center">
          <div>
            <div class="text-lg font-bold text-blue-600 dark:text-blue-400">{{ unassignedStats.running }}</div>
            <div class="text-xs text-muted-foreground">{{ $t('admin.settings.queue.running') }}</div>
          </div>
          <div>
            <div class="text-lg font-bold text-gray-600 dark:text-gray-400">{{ unassignedStats.pending }}</div>
            <div class="text-xs text-muted-foreground">{{ $t('admin.settings.queue.pending') }}</div>
          </div>
          <div>
            <div class="text-lg font-bold text-orange-600 dark:text-orange-400">{{ unassignedStats.waiting }}</div>
            <div class="text-xs text-muted-foreground">{{ $t('admin.settings.queue.waiting') }}</div>
          </div>
        </div>
      </div>
    </Card>
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import Card from '~/components/ui/card/Card.vue';
import type { QueueStats, Task } from '~/lib/api/types/queue';

const props = defineProps<{
  stats?: QueueStats;
  running?: Task[];
  pending?: Task[];
  waitingOnDeps?: Task[];
}>();

const { t } = useI18n();

interface AgentStat {
  name: string;
  running: number;
  pending: number;
  waiting: number;
}

const agentStats = computed<AgentStat[]>(() => {
  const agentMap = new Map<string, AgentStat>();

  // Count running tasks per agent
  props.running?.forEach((task) => {
    if (task.agent_name) {
      const existing = agentMap.get(task.agent_name) || { name: task.agent_name, running: 0, pending: 0, waiting: 0 };
      existing.running++;
      agentMap.set(task.agent_name, existing);
    }
  });

  // Count pending tasks per agent (if assigned)
  props.pending?.forEach((task) => {
    if (task.agent_name) {
      const existing = agentMap.get(task.agent_name) || { name: task.agent_name, running: 0, pending: 0, waiting: 0 };
      existing.pending++;
      agentMap.set(task.agent_name, existing);
    }
  });

  // Count waiting tasks per agent (if assigned)
  props.waitingOnDeps?.forEach((task) => {
    if (task.agent_name) {
      const existing = agentMap.get(task.agent_name) || { name: task.agent_name, running: 0, pending: 0, waiting: 0 };
      existing.waiting++;
      agentMap.set(task.agent_name, existing);
    }
  });

  return [...agentMap.values()].toSorted((a, b) => {
    // Sort by running count desc, then by name
    if (b.running !== a.running) return b.running - a.running;
    return a.name.localeCompare(b.name);
  });
});

const unassignedStats = computed(() => {
  let running = 0;
  let pending = 0;
  let waiting = 0;

  props.running?.forEach((task) => {
    if (!task.agent_name) running++;
  });
  props.pending?.forEach((task) => {
    if (!task.agent_name) pending++;
  });
  props.waitingOnDeps?.forEach((task) => {
    if (!task.agent_name) waiting++;
  });

  return { running, pending, waiting, total: running + pending + waiting };
});

const total = computed(() => {
  if (!props.stats) return 0;
  return (
    props.stats.worker_count + props.stats.running_count + props.stats.pending_count + props.stats.waiting_on_deps_count
  );
});

const progressData = computed(() => {
  if (!props.stats) return [];

  return [
    {
      key: 'worker_count',
      label: t('admin.settings.queue.stats.worker_count'),
      value: props.stats.worker_count,
      percentage: total.value > 0 ? (props.stats.worker_count / total.value) * 100 : 0,
      color: 'bg-green-500',
    },
    {
      key: 'running_count',
      label: t('admin.settings.queue.stats.running_count'),
      value: props.stats.running_count,
      percentage: total.value > 0 ? (props.stats.running_count / total.value) * 100 : 0,
      color: 'bg-blue-500',
    },
    {
      key: 'pending_count',
      label: t('admin.settings.queue.stats.pending_count'),
      value: props.stats.pending_count,
      percentage: total.value > 0 ? (props.stats.pending_count / total.value) * 100 : 0,
      color: 'bg-gray-400',
    },
    {
      key: 'waiting_on_deps_count',
      label: t('admin.settings.queue.stats.waiting_on_deps_count'),
      value: props.stats.waiting_on_deps_count,
      percentage: total.value > 0 ? (props.stats.waiting_on_deps_count / total.value) * 100 : 0,
      color: 'bg-orange-500',
    },
  ];
});
</script>

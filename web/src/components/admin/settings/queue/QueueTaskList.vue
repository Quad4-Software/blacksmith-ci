<template>
  <div class="space-y-3">
    <div v-for="task in tasks" :key="task.id" class="relative">
      <router-link
        :to="{
          name: 'repo-pipeline',
          params: { repoId: task.repo_id, pipelineId: task.pipeline_number, stepId: task.pid },
        }"
        class="block rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        :aria-label="$t('admin.settings.queue.view_task', { name: task.name })"
      >
        <Card class="hover:shadow-md hover:border-primary/50 transition-all cursor-pointer">
          <div class="p-4">
            <!-- Header: Status, Name, Agent -->
            <div
              class="mb-2 flex items-center gap-3"
              :class="{ 'pr-10 sm:pr-32': cancelable && task.status === 'running' }"
            >
              <!-- Status indicator -->
              <div
                class="w-3 h-3 rounded-full shrink-0"
                :class="getStatusColor(task.status)"
                :title="getStatusLabel(task.status)"
              />

              <!-- Task name -->
              <span class="font-medium truncate flex-1">{{ task.name }}</span>

              <!-- Agent badge -->
              <UiBadge v-if="task.agent_name" variant="secondary" class="shrink-0">
                {{ task.agent_name }}
              </UiBadge>

              <!-- Status badge -->
              <UiBadge :variant="getStatusBadgeVariant(task.status)" class="shrink-0">
                {{ getStatusLabel(task.status) }}
              </UiBadge>

              <!-- Arrow icon -->
              <Icon name="chevron-right" class="h-4 w-4 text-muted-foreground shrink-0" />
            </div>

            <!-- Time info -->
            <div class="flex items-center gap-4 mb-2 text-xs text-muted-foreground">
              <!-- Created time -->
              <div v-if="task.pipeline_created" class="flex items-center gap-1">
                <Icon name="ph:clock" class="h-3 w-3" />
                <span>{{ $t('admin.settings.queue.created') }} {{ timeAgo(task.pipeline_created * 1000) }}</span>
              </div>
              <!-- Running duration (for running tasks) -->
              <div v-if="task.status === 'running' && task.pipeline_started" class="flex items-center gap-1">
                <Icon name="ph:timer" class="h-3 w-3" />
                <span
                  >{{ $t('admin.settings.queue.running_for') }}
                  {{ prettyDuration(Date.now() - task.pipeline_started * 1000) }}</span
                >
              </div>
              <!-- Waiting duration (for pending/waiting tasks) -->
              <div v-else-if="task.pipeline_created" class="flex items-center gap-1">
                <Icon name="ph:hourglass" class="h-3 w-3" />
                <span
                  >{{ $t('admin.settings.queue.waiting_since') }}
                  {{ prettyDuration(Date.now() - task.pipeline_created * 1000) }}</span
                >
              </div>
            </div>

            <!-- Dependencies -->
            <div v-if="task.dependencies && task.dependencies.length > 0" class="mb-2">
              <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
              <span class="text-xs text-muted-foreground">{{ $t('admin.settings.queue.waiting_for') }}:</span>
              <span class="text-xs ml-1">{{ task.dependencies.join(', ') }}</span>
            </div>

            <!-- Labels -->
            <div v-if="Object.keys(task.labels).length > 0" class="flex flex-wrap gap-1.5">
              <template v-for="(value, label) in task.labels" :key="label">
                <UiBadge v-if="value" variant="outline" class="text-xs">
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  {{ label }}: {{ value }}
                </UiBadge>
              </template>
            </div>
          </div>
        </Card>
      </router-link>
      <Button
        v-if="cancelable && task.status === 'running'"
        variant="destructive"
        size="sm"
        class="absolute right-12 top-3.5"
        :disabled="cancelingTaskId === task.id"
        @click="emit('cancel', task)"
      >
        <Icon v-if="cancelingTaskId === task.id" name="spinner" class="h-4 w-4 animate-spin sm:mr-1.5" />
        <Icon v-else name="close" class="h-4 w-4 sm:mr-1.5" />
        <span class="hidden sm:inline">{{ $t('admin.settings.queue.cancel_task') }}</span>
        <span class="sr-only sm:hidden">{{ $t('admin.settings.queue.cancel_task') }}</span>
      </Button>
    </div>
  </div>
</template>

<script lang="ts" setup>
import Icon from '~/components/atomic/Icon.vue';
import { Badge as UiBadge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import Card from '~/components/ui/card/Card.vue';
import type { Task } from '~/lib/api/types';

interface TaskWithStatus extends Task {
  status: 'running' | 'pending' | 'waiting_on_deps';
}

withDefaults(
  defineProps<{
    tasks: TaskWithStatus[];
    getStatusColor: (status: string) => string;
    getStatusLabel: (status: string) => string;
    getStatusBadgeVariant: (status: string) => 'default' | 'secondary' | 'destructive' | 'outline';
    timeAgo: (timestamp: number) => string;
    prettyDuration: (ms: number) => string;
    cancelable?: boolean;
    cancelingTaskId?: number;
  }>(),
  {
    cancelable: false,
    cancelingTaskId: undefined,
  },
);

const emit = defineEmits<{
  cancel: [task: TaskWithStatus];
}>();
</script>

<template>
  <Settings :title="$t('admin.settings.queue.queue')" :description="$t('admin.settings.queue.desc')">
    <template #headerActions>
      <div v-if="queueInfo" class="flex items-center gap-2">
        <Button v-if="queueInfo.paused" variant="default" size="sm" @click="resumeQueue">
          <Icon name="play" class="mr-2 h-4 w-4" />
          {{ $t('admin.settings.queue.resume') }}
        </Button>
        <Button v-else variant="outline" size="sm" @click="pauseQueue">
          <Icon name="pause" class="mr-2 h-4 w-4" />
          {{ $t('admin.settings.queue.pause') }}
        </Button>
        <Badge v-if="queueInfo.paused" variant="destructive">{{ $t('admin.settings.queue.paused') }}</Badge>
      </div>
    </template>

    <div class="flex flex-col gap-6">
      <!-- Stats Dashboard -->
      <AdminQueueStats
        :stats="queueInfo?.stats"
        :running="queueInfo?.running"
        :pending="queueInfo?.pending"
        :waiting-on-deps="queueInfo?.waiting_on_deps"
      />

      <!-- Task List -->
      <div v-if="tasks.length > 0">
        <div class="flex flex-wrap items-center gap-2 mb-4">
          <h3 class="text-lg font-semibold">{{ $t('admin.settings.queue.tasks') }}</h3>

          <!-- Spacer -->
          <div class="flex-1" />

          <!-- Search -->
          <div class="w-[200px]">
            <SearchInput v-model="searchQuery" :placeholder="$t('admin.settings.queue.search')" class="[&_input]:h-8" />
          </div>

          <!-- Status filter -->
          <DropdownMenu :modal="false">
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" class="h-8">
                <Icon name="filter" class="mr-2 h-4 w-4" />
                <span v-if="statusFilter === ''">{{ $t('admin.settings.queue.filter_status') }}</span>
                <span v-else>{{ getStatusLabel(statusFilter) }}</span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent class="w-[200px]" align="end">
              <DropdownMenuRadioGroup :model-value="statusFilter" @update:model-value="statusFilter = $event">
                <DropdownMenuRadioItem value="">{{ $t('admin.settings.queue.all_statuses') }}</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="running">{{ $t('admin.settings.queue.running') }}</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="pending">{{ $t('admin.settings.queue.pending') }}</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="waiting_on_deps">{{
                  $t('admin.settings.queue.waiting')
                }}</DropdownMenuRadioItem>
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>

          <!-- Sort -->
          <DropdownMenu :modal="false">
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" class="h-8">
                <Icon name="lucide:arrow-up-down" class="mr-2 h-4 w-4" />
                {{ $t('sort') }}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent class="w-[200px]" align="end">
              <DropdownMenuRadioGroup :model-value="sortBy" @update:model-value="sortBy = $event">
                <DropdownMenuRadioItem value="name">{{ $t('admin.settings.queue.sort.name') }}</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="status">{{
                  $t('admin.settings.queue.sort.status')
                }}</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="agent">{{ $t('admin.settings.queue.sort.agent') }}</DropdownMenuRadioItem>
              </DropdownMenuRadioGroup>
              <DropdownMenuSeparator />
              <DropdownMenuCheckboxItem :checked="sortDesc" @update:checked="sortDesc = $event">
                {{ $t('admin.settings.queue.sort_descending') }}
              </DropdownMenuCheckboxItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <!-- Task Cards -->
        <div class="space-y-3">
          <router-link
            v-for="task in filteredTasks"
            :key="task.id"
            :to="{
              name: 'repo-pipeline',
              params: { repoId: task.repo_id, pipelineId: task.pipeline_number, stepId: task.pid },
            }"
            class="block"
          >
            <Card class="hover:shadow-md hover:border-primary/50 transition-all cursor-pointer">
              <div class="p-4">
                <!-- Header: Status, Name, Agent -->
                <div class="flex items-center gap-3 mb-2">
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
                    <Icon name="lucide:clock" class="h-3 w-3" />
                    <span>{{ $t('admin.settings.queue.created') }} {{ timeAgo(task.pipeline_created * 1000) }}</span>
                  </div>
                  <!-- Running duration (for running tasks) -->
                  <div v-if="task.status === 'running' && task.pipeline_started" class="flex items-center gap-1">
                    <Icon name="lucide:timer" class="h-3 w-3" />
                    <span
                      >{{ $t('admin.settings.queue.running_for') }}
                      {{ prettyDuration(Date.now() - task.pipeline_started * 1000) }}</span
                    >
                  </div>
                  <!-- Waiting duration (for pending/waiting tasks) -->
                  <div v-else-if="task.pipeline_created" class="flex items-center gap-1">
                    <Icon name="lucide:hourglass" class="h-3 w-3" />
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
        </div>

        <!-- Empty filtered state -->
        <div v-if="filteredTasks.length === 0 && tasks.length > 0" class="text-center py-8 text-muted-foreground">
          <Icon name="question" class="h-12 w-12 mx-auto mb-3 opacity-50" />
          <p>{{ $t('admin.settings.queue.no_match') }}</p>
        </div>
      </div>

      <!-- Empty state -->
      <div v-else-if="queueInfo" class="text-center py-12 text-muted-foreground">
        <Icon name="list" class="h-12 w-12 mx-auto mb-3 opacity-50" />
        <p>{{ $t('admin.settings.queue.no_tasks') }}</p>
      </div>
    </div>
  </Settings>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import AdminQueueStats from '~/components/admin/settings/queue/AdminQueueStats.vue';
import Icon from '~/components/atomic/Icon.vue';
import Settings from '~/components/layout/Settings.vue';
import { Badge as UiBadge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import Card from '~/components/ui/card/Card.vue';
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import SearchInput from '~/components/ui/input/SearchInput.vue';
import useApiClient from '~/compositions/useApiClient';
import { useDate } from '~/compositions/useDate';
import useNotifications from '~/compositions/useNotifications';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { QueueInfo, Task } from '~/lib/api/types';

const apiClient = useApiClient();
const notifications = useNotifications();
const { timeAgo, prettyDuration } = useDate();
const { t } = useI18n();

const queueInfo = ref<QueueInfo>();
const searchQuery = ref('');
const statusFilter = ref('');
const sortBy = ref('status');
const sortDesc = ref(false);

interface TaskWithStatus extends Task {
  status: 'running' | 'pending' | 'waiting_on_deps';
}

const tasks = computed<TaskWithStatus[]>(() => {
  const _tasks: TaskWithStatus[] = [];

  if (queueInfo.value?.running) {
    _tasks.push(...queueInfo.value.running.map((task) => ({ ...task, status: 'running' as const })));
  }

  if (queueInfo.value?.pending) {
    _tasks.push(...queueInfo.value.pending.map((task) => ({ ...task, status: 'pending' as const })));
  }

  if (queueInfo.value?.waiting_on_deps) {
    _tasks.push(...queueInfo.value.waiting_on_deps.map((task) => ({ ...task, status: 'waiting_on_deps' as const })));
  }

  return _tasks.map((task) => ({
    ...task,
    labels: Object.fromEntries(Object.entries(task.labels).filter(([key]) => key !== 'org-id')),
  }));
});

const filteredTasks = computed(() => {
  let result = [...tasks.value];

  // Filter by search
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase();
    result = result.filter(
      (task) =>
        task.name.toLowerCase().includes(query) ||
        task.agent_name?.toLowerCase().includes(query) ||
        Object.values(task.labels).some((v) => v.toLowerCase().includes(query)),
    );
  }

  // Filter by status
  if (statusFilter.value) {
    result = result.filter((task) => task.status === statusFilter.value);
  }

  // Sort
  result.sort((a, b) => {
    let cmp = 0;
    switch (sortBy.value) {
      case 'name':
        cmp = a.name.localeCompare(b.name);
        break;
      case 'status':
        cmp = getStatusOrder(a.status) - getStatusOrder(b.status);
        break;
      case 'agent':
        cmp = (a.agent_name || '').localeCompare(b.agent_name || '');
        break;
      default:
        cmp = a.id - b.id;
    }
    return sortDesc.value ? -cmp : cmp;
  });

  return result;
});

function getStatusOrder(status: string): number {
  switch (status) {
    case 'running':
      return 0;
    case 'pending':
      return 1;
    case 'waiting_on_deps':
      return 2;
    default:
      return 3;
  }
}

function getStatusLabel(status: string): string {
  switch (status) {
    case 'running':
      return t('admin.settings.queue.running');
    case 'pending':
      return t('admin.settings.queue.pending');
    case 'waiting_on_deps':
      return t('admin.settings.queue.waiting');
    default:
      return status;
  }
}

function getStatusColor(status: string): string {
  switch (status) {
    case 'running':
      return 'bg-blue-500 animate-pulse';
    case 'pending':
      return 'bg-gray-400';
    case 'waiting_on_deps':
      return 'bg-orange-500';
    default:
      return 'bg-gray-400';
  }
}

function getStatusBadgeVariant(status: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case 'running':
      return 'default';
    case 'pending':
      return 'secondary';
    case 'waiting_on_deps':
      return 'outline';
    default:
      return 'secondary';
  }
}

async function loadQueueInfo() {
  queueInfo.value = await apiClient.getQueueInfo();
}

async function pauseQueue() {
  await apiClient.pauseQueue();
  await loadQueueInfo();
  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.queue.notify_text_paused'),
    type: 'success',
    duration: 6000,
  });
}

async function resumeQueue() {
  await apiClient.resumeQueue();
  await loadQueueInfo();
  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.queue.notify_text_resumed'),
    type: 'success',
    duration: 6000,
  });
}

const reloadInterval = ref<number>();
onMounted(async () => {
  await loadQueueInfo();
  reloadInterval.value = window.setInterval(async () => {
    await loadQueueInfo();
  }, 5000);
});

onBeforeUnmount(() => {
  if (reloadInterval.value) {
    window.clearInterval(reloadInterval.value);
  }
});

useCrowTitle(computed(() => [t('admin.settings.queue.queue'), t('admin.settings.settings')]));
</script>

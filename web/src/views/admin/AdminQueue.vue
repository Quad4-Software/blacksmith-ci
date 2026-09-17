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

          <!-- Sort -->
          <DropdownMenu :modal="false">
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" class="h-8">
                <Icon name="ph:arrows-down-up" class="mr-2 h-4 w-4" />
                {{ $t('sort') }}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent class="w-[200px]" align="end">
              <DropdownMenuRadioGroup :model-value="sortBy" @update:model-value="sortBy = String($event)">
                <DropdownMenuRadioItem value="name">{{ $t('admin.settings.queue.sort.name') }}</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="agent">{{ $t('admin.settings.queue.sort.agent') }}</DropdownMenuRadioItem>
              </DropdownMenuRadioGroup>
              <DropdownMenuSeparator />
              <DropdownMenuCheckboxItem :checked="sortDesc" @update:checked="sortDesc = $event">
                {{ $t('admin.settings.queue.sort_descending') }}
              </DropdownMenuCheckboxItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <!-- Status Tabs -->
        <Tabs v-model="activeTab" class="w-full">
          <TabsList class="mb-4">
            <TabsTrigger value="running" class="gap-2">
              {{ $t('admin.settings.queue.running') }}
              <UiBadge v-if="runningTasks.length > 0" variant="default" class="ml-1">
                {{ runningTasks.length }}
              </UiBadge>
            </TabsTrigger>
            <TabsTrigger value="pending" class="gap-2">
              {{ $t('admin.settings.queue.pending') }}
              <UiBadge v-if="pendingTasks.length > 0" variant="secondary" class="ml-1">
                {{ pendingTasks.length }}
              </UiBadge>
            </TabsTrigger>
            <TabsTrigger value="waiting_on_deps" class="gap-2">
              {{ $t('admin.settings.queue.waiting') }}
              <UiBadge v-if="waitingTasks.length > 0" variant="outline" class="ml-1">
                {{ waitingTasks.length }}
              </UiBadge>
            </TabsTrigger>
          </TabsList>

          <TabsContent value="running">
            <TaskList
              :tasks="filteredRunningTasks"
              :get-status-color="getStatusColor"
              :get-status-label="getStatusLabel"
              :get-status-badge-variant="getStatusBadgeVariant"
              :time-ago="timeAgo"
              :pretty-duration="prettyDuration"
              cancelable
              :canceling-task-id="cancelingTaskId"
              @cancel="taskToCancel = $event"
            />
            <div v-if="filteredRunningTasks.length === 0" class="text-center py-8 text-muted-foreground">
              <Icon name="list" class="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p>{{ searchQuery ? $t('admin.settings.queue.no_match') : $t('admin.settings.queue.no_running') }}</p>
            </div>
          </TabsContent>

          <TabsContent value="pending">
            <TaskList
              :tasks="filteredPendingTasks"
              :get-status-color="getStatusColor"
              :get-status-label="getStatusLabel"
              :get-status-badge-variant="getStatusBadgeVariant"
              :time-ago="timeAgo"
              :pretty-duration="prettyDuration"
            />
            <div v-if="filteredPendingTasks.length === 0" class="text-center py-8 text-muted-foreground">
              <Icon name="list" class="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p>{{ searchQuery ? $t('admin.settings.queue.no_match') : $t('admin.settings.queue.no_pending') }}</p>
            </div>
          </TabsContent>

          <TabsContent value="waiting_on_deps">
            <TaskList
              :tasks="filteredWaitingTasks"
              :get-status-color="getStatusColor"
              :get-status-label="getStatusLabel"
              :get-status-badge-variant="getStatusBadgeVariant"
              :time-ago="timeAgo"
              :pretty-duration="prettyDuration"
            />
            <div v-if="filteredWaitingTasks.length === 0" class="text-center py-8 text-muted-foreground">
              <Icon name="list" class="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p>{{ searchQuery ? $t('admin.settings.queue.no_match') : $t('admin.settings.queue.no_waiting') }}</p>
            </div>
          </TabsContent>
        </Tabs>
      </div>

      <!-- Empty state -->
      <div v-else-if="queueInfo" class="text-center py-12 text-muted-foreground">
        <Icon name="list" class="h-12 w-12 mx-auto mb-3 opacity-50" />
        <p>{{ $t('admin.settings.queue.no_tasks') }}</p>
      </div>
    </div>
  </Settings>

  <AlertDialog :open="taskToCancel !== undefined" @update:open="onCancelDialogOpenChange">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('admin.settings.queue.cancel_confirm_title') }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{ $t('admin.settings.queue.cancel_confirm_desc', { name: taskToCancel?.name }) }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel :disabled="cancelingTaskId !== undefined">{{ $t('cancel') }}</AlertDialogCancel>
        <AlertDialogAction
          class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          :disabled="cancelingTaskId !== undefined"
          @click.prevent="cancelTask"
        >
          <Icon v-if="cancelingTaskId !== undefined" name="spinner" class="mr-1.5 h-4 w-4 animate-spin" />
          {{ $t('admin.settings.queue.cancel_task') }}
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>

<script lang="ts" setup>
import AdminQueueStats from '~/components/admin/settings/queue/AdminQueueStats.vue';
import TaskList from '~/components/admin/settings/queue/QueueTaskList.vue';
import Icon from '~/components/atomic/Icon.vue';
import Settings from '~/components/layout/Settings.vue';
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
import { Badge as UiBadge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '~/components/ui/tabs';
import useApiClient from '~/compositions/useApiClient';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import { useDate } from '~/compositions/useDate';
import useNotifications from '~/compositions/useNotifications';
import type { QueueInfo, Task } from '~/lib/api/types';
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';

const apiClient = useApiClient();
const notifications = useNotifications();
const { timeAgo, prettyDuration } = useDate();
const { t } = useI18n();

const queueInfo = ref<QueueInfo>();
const searchQuery = ref('');
const activeTab = ref('running');
const sortBy = ref('name');
const sortDesc = ref(false);
const taskToCancel = ref<TaskWithStatus>();
const cancelingTaskId = ref<number>();

interface TaskWithStatus extends Task {
  status: 'running' | 'pending' | 'waiting_on_deps';
}

// Helper to clean up task labels
function cleanTaskLabels(task: Task): Task {
  return {
    ...task,
    labels: Object.fromEntries(Object.entries(task.labels).filter(([key]) => key !== 'org-id')),
  };
}

// Helper to filter and sort tasks
function filterAndSortTasks(taskList: TaskWithStatus[]): TaskWithStatus[] {
  let result = [...taskList];

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

  // Sort
  result.sort((a, b) => {
    let cmp = 0;
    switch (sortBy.value) {
      case 'name':
        cmp = a.name.localeCompare(b.name);
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
}

// All tasks combined (for checking if any exist)
const tasks = computed<TaskWithStatus[]>(() => {
  const _tasks: TaskWithStatus[] = [];

  if (queueInfo.value?.running) {
    _tasks.push(...queueInfo.value.running.map((task) => ({ ...cleanTaskLabels(task), status: 'running' as const })));
  }

  if (queueInfo.value?.pending) {
    _tasks.push(...queueInfo.value.pending.map((task) => ({ ...cleanTaskLabels(task), status: 'pending' as const })));
  }

  if (queueInfo.value?.waiting_on_deps) {
    _tasks.push(
      ...queueInfo.value.waiting_on_deps.map((task) => ({
        ...cleanTaskLabels(task),
        status: 'waiting_on_deps' as const,
      })),
    );
  }

  return _tasks;
});

// Tasks by status
const runningTasks = computed<TaskWithStatus[]>(() => {
  if (!queueInfo.value?.running) return [];
  return queueInfo.value.running.map((task) => ({ ...cleanTaskLabels(task), status: 'running' as const }));
});

const pendingTasks = computed<TaskWithStatus[]>(() => {
  if (!queueInfo.value?.pending) return [];
  return queueInfo.value.pending.map((task) => ({ ...cleanTaskLabels(task), status: 'pending' as const }));
});

const waitingTasks = computed<TaskWithStatus[]>(() => {
  if (!queueInfo.value?.waiting_on_deps) return [];
  return queueInfo.value.waiting_on_deps.map((task) => ({
    ...cleanTaskLabels(task),
    status: 'waiting_on_deps' as const,
  }));
});

// Filtered tasks by status
const filteredRunningTasks = computed(() => filterAndSortTasks(runningTasks.value));
const filteredPendingTasks = computed(() => filterAndSortTasks(pendingTasks.value));
const filteredWaitingTasks = computed(() => filterAndSortTasks(waitingTasks.value));

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

function onCancelDialogOpenChange(open: boolean) {
  if (!open && cancelingTaskId.value === undefined) {
    taskToCancel.value = undefined;
  }
}

async function cancelTask() {
  const task = taskToCancel.value;
  if (!task || cancelingTaskId.value !== undefined) return;

  cancelingTaskId.value = task.id;
  try {
    await apiClient.cancelQueueTask(task.id);
    await loadQueueInfo();
    taskToCancel.value = undefined;
    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.queue.notify_text_cancel_success', { name: task.name }),
      type: 'success',
      duration: 6000,
    });
  } catch {
    notifications.notify({
      title: t('notify_title_error'),
      text: t('unknown_error'),
      type: 'error',
      duration: 9000,
    });
  } finally {
    cancelingTaskId.value = undefined;
  }
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

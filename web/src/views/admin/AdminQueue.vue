<template>
  <Settings :title="$t('admin.settings.queue.queue')" :description="$t('admin.settings.queue.desc')">
    <template #headerActions>
      <div v-if="queueInfo" class="flex items-center gap-2">
        <Button v-if="queueInfo.paused && can('queue:write')" variant="default" size="sm" @click="resumeQueue">
          <Icon name="play" class="mr-2 h-4 w-4" />
          {{ $t('admin.settings.queue.resume') }}
        </Button>
        <Button v-else-if="can('queue:write')" variant="outline" size="sm" @click="pauseQueue">
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
            <TabsTrigger v-for="tab in tabs" :key="tab.value" :value="tab.value" class="gap-2">
              {{ tab.label }}
              <UiBadge v-if="tab.tasks.length > 0" :variant="tab.badgeVariant" class="ml-1">
                {{ tab.tasks.length }}
              </UiBadge>
            </TabsTrigger>
          </TabsList>

          <p v-if="activeTab === 'stuck'" class="mb-4 text-sm text-muted-foreground">
            {{
              queueInfo?.stuck_threshold
                ? $t('admin.settings.queue.stuck_desc', { threshold: stuckThresholdLabel })
                : $t('admin.settings.queue.stuck_desc_no_threshold')
            }}
          </p>

          <!-- Batch actions -->
          <div v-if="can('queue:write') && activeTabTasks.length > 0" class="mb-3 flex items-center gap-3 px-4">
            <Checkbox
              id="queue-select-all"
              :checked="allSelected ? true : selectedTaskIds.size > 0 ? 'indeterminate' : false"
              :disabled="batchCanceling"
              @update:checked="(checked: boolean | 'indeterminate') => selectAll(checked === true)"
            />
            <label for="queue-select-all" class="text-sm text-muted-foreground">
              {{ $t('admin.settings.queue.select_all') }}
            </label>
            <div class="flex-1" />
            <Button
              variant="destructive"
              size="sm"
              class="h-8"
              :disabled="selectedTaskIds.size === 0 || batchCanceling"
              @click="batchCancelOpen = true"
            >
              <Icon v-if="batchCanceling" name="spinner" class="mr-1.5 h-4 w-4 animate-spin" />
              <Icon v-else name="close" class="mr-1.5 h-4 w-4" />
              {{ $t('admin.settings.queue.cancel_selected') }}
              <UiBadge v-if="selectedTaskIds.size > 0" variant="outline" class="ml-1">
                {{ selectedTaskIds.size }}
              </UiBadge>
            </Button>
          </div>

          <TabsContent v-for="tab in tabs" :key="tab.value" :value="tab.value">
            <TaskList
              :tasks="tab.filtered"
              :get-status-color="getStatusColor"
              :get-status-label="getStatusLabel"
              :get-status-badge-variant="getStatusBadgeVariant"
              :time-ago="timeAgo"
              :pretty-duration="prettyDuration"
              :cancelable="can('queue:write')"
              :canceling-task-id="cancelingTaskId"
              :selectable="can('queue:write')"
              :selected-ids="selectedTaskIds"
              :stuck-threshold-label="stuckThresholdLabel"
              @cancel="taskToCancel = $event"
              @select="toggleSelected"
            />
            <div v-if="tab.filtered.length === 0" class="text-center py-8 text-muted-foreground">
              <Icon name="list" class="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p>{{ searchQuery ? $t('admin.settings.queue.no_match') : tab.empty }}</p>
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

  <AlertDialog :open="batchCancelOpen" @update:open="onBatchCancelDialogOpenChange">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>
          {{ $t('admin.settings.queue.cancel_selected_title', { count: selectedTaskIds.size }) }}
        </AlertDialogTitle>
        <AlertDialogDescription>{{ $t('admin.settings.queue.cancel_selected_desc') }}</AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel :disabled="batchCanceling">{{ $t('cancel') }}</AlertDialogCancel>
        <AlertDialogAction
          class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          :disabled="batchCanceling"
          @click.prevent="cancelSelectedTasks"
        >
          <Icon v-if="batchCanceling" name="spinner" class="mr-1.5 h-4 w-4 animate-spin" />
          {{ $t('admin.settings.queue.cancel_selected') }}
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

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
import { Checkbox } from '~/components/ui/checkbox';
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
import usePermissions from '~/compositions/usePermissions';
import type { QueueInfo, Task } from '~/lib/api/types';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

const apiClient = useApiClient();
const { can } = usePermissions();
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
const selectedTaskIds = ref(new Set<string>());
const batchCancelOpen = ref(false);
const batchCanceling = ref(false);

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

// Queued tasks the server flags as stuck; running tasks never are
const stuckTasks = computed<TaskWithStatus[]>(() =>
  [...pendingTasks.value, ...waitingTasks.value].filter((task) => task.stuck_reason),
);

const stuckThresholdLabel = computed(() =>
  queueInfo.value?.stuck_threshold ? prettyDuration(queueInfo.value.stuck_threshold * 1000) : '',
);

const tabs = computed(() => [
  {
    value: 'running',
    label: t('admin.settings.queue.running'),
    tasks: runningTasks.value,
    filtered: filterAndSortTasks(runningTasks.value),
    badgeVariant: 'default' as const,
    empty: t('admin.settings.queue.no_running'),
  },
  {
    value: 'pending',
    label: t('admin.settings.queue.pending'),
    tasks: pendingTasks.value,
    filtered: filterAndSortTasks(pendingTasks.value),
    badgeVariant: 'secondary' as const,
    empty: t('admin.settings.queue.no_pending'),
  },
  {
    value: 'waiting_on_deps',
    label: t('admin.settings.queue.waiting'),
    tasks: waitingTasks.value,
    filtered: filterAndSortTasks(waitingTasks.value),
    badgeVariant: 'outline' as const,
    empty: t('admin.settings.queue.no_waiting'),
  },
  {
    value: 'stuck',
    label: t('admin.settings.queue.stuck'),
    tasks: stuckTasks.value,
    filtered: filterAndSortTasks(stuckTasks.value),
    badgeVariant: 'destructive' as const,
    empty: t('admin.settings.queue.no_stuck'),
  },
]);

const activeTabTasks = computed(() => tabs.value.find((tab) => tab.value === activeTab.value)?.filtered ?? []);

const allSelected = computed(
  () =>
    activeTabTasks.value.length > 0 && activeTabTasks.value.every((task) => selectedTaskIds.value.has(String(task.id))),
);

function toggleSelected(task: TaskWithStatus, selected: boolean) {
  const next = new Set(selectedTaskIds.value);
  if (selected) {
    next.add(String(task.id));
  } else {
    next.delete(String(task.id));
  }
  selectedTaskIds.value = next;
}

function selectAll(selected: boolean) {
  selectedTaskIds.value = selected ? new Set(activeTabTasks.value.map((task) => String(task.id))) : new Set();
}

// Selection applies to the visible tab only, so switching tabs or narrowing
// the search drops tasks that are no longer shown.
watch([activeTab, searchQuery], () => {
  selectedTaskIds.value = new Set();
});

// Drop tasks that left the queue since the last refresh.
watch(activeTabTasks, (visible) => {
  const ids = new Set(visible.map((task) => String(task.id)));
  if ([...selectedTaskIds.value].some((id) => !ids.has(id))) {
    selectedTaskIds.value = new Set([...selectedTaskIds.value].filter((id) => ids.has(id)));
  }
});

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

function onBatchCancelDialogOpenChange(open: boolean) {
  if (!open && !batchCanceling.value) {
    batchCancelOpen.value = false;
  }
}

async function cancelSelectedTasks() {
  const ids = [...selectedTaskIds.value];
  if (ids.length === 0 || batchCanceling.value) return;

  batchCanceling.value = true;
  try {
    const result = await apiClient.cancelQueueTasks(ids);
    const failed = Object.keys(result.failed ?? {}).length;
    selectedTaskIds.value = new Set(Object.keys(result.failed ?? {}));
    batchCancelOpen.value = false;
    await loadQueueInfo();
    notifications.notify({
      title: failed > 0 ? t('notify_title_error') : t('notify_title_success'),
      text:
        failed > 0
          ? t('admin.settings.queue.notify_text_cancel_selected_partial', { canceled: result.canceled, failed })
          : t('admin.settings.queue.notify_text_cancel_selected_success', { count: result.canceled }),
      type: failed > 0 ? 'error' : 'success',
      duration: failed > 0 ? 9000 : 6000,
    });
  } catch {
    notifications.notify({
      title: t('notify_title_error'),
      text: t('unknown_error'),
      type: 'error',
      duration: 9000,
    });
  } finally {
    batchCanceling.value = false;
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

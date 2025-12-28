<template>
  <Scaffold fill-width>
    <template #title>
      {{ $t('repo.settings.crons.all_crons') }}
    </template>

    <template #headerActions>
      <Button size="sm" @click="cronDialog.open()">
        <Icon name="plus" class="mr-2 h-4 w-4" />
        {{ $t('repo.settings.crons.add') }}
      </Button>
    </template>

    <div class="max-w-7xl mx-auto space-y-4">
      <!-- Toolbar -->
      <div class="flex items-center gap-2 flex-wrap">
        <!-- Search -->
        <Input v-model="searchTerm" :placeholder="$t('search')" class="w-[300px] min-w-[80px]" />

        <!-- Owner Filter -->
        <DropdownMenu v-if="uniqueOwners.length > 1" :modal="false">
          <DropdownMenuTrigger as-child>
            <Button variant="outline" size="sm" class="h-9 border-dashed">
              <Icon name="user" class="mr-2 h-4 w-4" />
              {{ $t('repo.settings.crons.owner') }}
              <template v-if="selectedOwners.length > 0">
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span class="mx-2" aria-hidden="true">|</span>
                <Badge variant="secondary" class="rounded-sm px-1 font-normal">
                  {{ selectedOwners.length }}
                </Badge>
              </template>
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent class="w-[200px]" align="start">
            <div class="max-h-[300px] overflow-y-auto">
              <DropdownMenuCheckboxItem
                v-for="owner in uniqueOwners"
                :key="owner"
                :model-value="selectedOwners.includes(owner)"
                @update:model-value="(value) => toggleOwnerFilter(owner, !!value)"
              >
                {{ owner }}
              </DropdownMenuCheckboxItem>
            </div>
            <template v-if="selectedOwners.length > 0">
              <DropdownMenuSeparator />
              <DropdownMenuItem @click="clearOwnerFilters">
                {{ $t('clear_filters') }}
              </DropdownMenuItem>
            </template>
          </DropdownMenuContent>
        </DropdownMenu>

        <!-- Schedule Type Filter -->
        <DropdownMenu :modal="false">
          <DropdownMenuTrigger as-child>
            <Button variant="outline" size="sm" class="h-9 border-dashed">
              <Icon name="lucide:clock" class="mr-2 h-4 w-4" />
              {{ $t('repo.settings.crons.schedule.title') }}
              <template v-if="selectedScheduleTypes.length > 0">
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span class="mx-2" aria-hidden="true">|</span>
                <Badge variant="secondary" class="rounded-sm px-1 font-normal">
                  {{ selectedScheduleTypes.length }}
                </Badge>
              </template>
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent class="w-[180px]" align="start">
            <DropdownMenuCheckboxItem
              v-for="scheduleType in scheduleTypes"
              :key="scheduleType.value"
              :model-value="selectedScheduleTypes.includes(scheduleType.value)"
              @update:model-value="(value) => toggleScheduleTypeFilter(scheduleType.value, !!value)"
            >
              {{ scheduleType.label }}
            </DropdownMenuCheckboxItem>
            <template v-if="selectedScheduleTypes.length > 0">
              <DropdownMenuSeparator />
              <DropdownMenuItem @click="clearScheduleTypeFilters">
                {{ $t('clear_filters') }}
              </DropdownMenuItem>
            </template>
          </DropdownMenuContent>
        </DropdownMenu>

        <!-- Clear All Filters -->
        <Button v-if="hasActiveFilters" variant="ghost" size="sm" class="h-9" @click="clearAllFilters">
          {{ $t('clear_filters') }}
          <Icon name="close" class="ml-2 h-4 w-4" />
        </Button>

        <div class="flex-1" />

        <!-- Column Visibility -->
        <DropdownMenu :modal="false">
          <DropdownMenuTrigger as-child>
            <Button variant="outline" size="sm" class="h-9">
              <Icon name="columns" class="mr-2 h-4 w-4" />
              {{ $t('columns') }}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuCheckboxItem
              v-for="col in toggleableColumns"
              :key="col.key"
              :model-value="visibleColumns.includes(col.key)"
              @update:model-value="(v) => toggleColumn(col.key, !!v)"
            >
              {{ col.label }}
            </DropdownMenuCheckboxItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <!-- Data Table -->
      <div v-if="sortedCrons && sortedCrons.length > 0" class="rounded-md border overflow-x-auto">
        <Table ref="tableRef" class="table-fixed">
          <TableHeader>
            <TableRow>
              <TableHead class="w-48 min-w-32 cursor-pointer select-none" @click="toggleSort('name')">
                <div class="flex items-center gap-1">
                  {{ $t('repo.settings.crons.name.name') }}
                  <SortIndicator sort-key="name" :current-sort="sortConfig" />
                </div>
              </TableHead>
              <TableHead class="w-48 min-w-32 cursor-pointer select-none" @click="toggleSort('repo_name')">
                <div class="flex items-center gap-1">
                  {{ $t('repo.settings.crons.repo') }}
                  <SortIndicator sort-key="repo_name" :current-sort="sortConfig" />
                </div>
              </TableHead>
              <TableHead class="w-32 min-w-24 cursor-pointer select-none" @click="toggleSort('schedule')">
                <div class="flex items-center gap-1">
                  {{ $t('repo.settings.crons.schedule.title') }}
                  <SortIndicator sort-key="schedule" :current-sort="sortConfig" />
                </div>
              </TableHead>
              <TableHead v-if="visibleColumns.includes('branch')" class="w-28 min-w-20">{{
                $t('repo.settings.crons.branch.title')
              }}</TableHead>
              <TableHead
                v-if="visibleColumns.includes('next_exec')"
                class="w-40 min-w-32 cursor-pointer select-none"
                @click="toggleSort('next_exec')"
              >
                <div class="flex items-center gap-1">
                  {{ $t('repo.settings.crons.next_exec') }}
                  <SortIndicator sort-key="next_exec" :current-sort="sortConfig" />
                </div>
              </TableHead>
              <TableHead v-if="visibleColumns.includes('recent_runs')" class="w-28 min-w-24">{{
                $t('repo.settings.crons.recent_runs')
              }}</TableHead>
              <TableHead class="w-28 min-w-24 text-right">{{ $t('actions') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow
              v-for="cron in sortedCrons"
              :key="cron.id"
              class="cursor-pointer hover:bg-muted/50"
              @click="navigateToCron(cron)"
            >
              <TableCell class="font-medium" :title="cron.name">{{ cron.name }}</TableCell>
              <TableCell :title="cron.repo_name">{{ cron.repo_name }}</TableCell>
              <TableCell :title="cron.schedule">
                <code class="text-xs bg-muted px-1.5 py-0.5 rounded">{{ cron.schedule }}</code>
              </TableCell>
              <TableCell
                v-if="visibleColumns.includes('branch')"
                :title="cron.branch || cron.repo_default_branch || $t('repo.settings.crons.branch.default')"
              >
                <span class="text-sm">{{
                  cron.branch || cron.repo_default_branch || $t('repo.settings.crons.branch.default')
                }}</span>
              </TableCell>
              <TableCell
                v-if="visibleColumns.includes('next_exec')"
                :title="
                  cron.next_exec && cron.next_exec > 0
                    ? date.toISODateTimeString(new Date(cron.next_exec * 1000))
                    : undefined
                "
              >
                <span v-if="cron.next_exec && cron.next_exec > 0" class="text-sm font-mono">
                  {{ date.toISODateTimeString(new Date(cron.next_exec * 1000)) }}
                </span>
                <span v-else class="text-muted-foreground text-sm">
                  {{ $t('repo.settings.crons.not_executed_yet') }}
                </span>
              </TableCell>
              <TableCell v-if="visibleColumns.includes('recent_runs')" class="overflow-hidden" @click.stop>
                <div v-if="cronRecentRuns[cron.id]?.length" class="flex items-center gap-1 max-w-full overflow-hidden">
                  <router-link
                    v-for="pipeline in cronRecentRuns[cron.id].slice(0, 5)"
                    :key="pipeline.id"
                    :to="{ name: 'repo-pipeline', params: { repoId: cron.repo_id, pipelineId: pipeline.number } }"
                    class="hover:scale-125 transition-all hover:drop-shadow-md rounded-full p-0.5 hover:bg-accent"
                  >
                    <PipelineStatusIcon
                      :status="pipeline.status"
                      :title="`#${pipeline.number} - ${date.toISODateTimeString(new Date(pipeline.created * 1000))}`"
                      class="size-4"
                    />
                  </router-link>
                </div>
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span v-else class="text-muted-foreground text-sm" aria-hidden="true">-</span>
              </TableCell>
              <TableCell class="text-right overflow-visible" @click.stop>
                <div class="flex items-center justify-end gap-0.5 md:gap-1">
                  <Button variant="ghost" size="icon-sm" :title="$t('repo.settings.crons.run')" @click="runCron(cron)">
                    <Icon name="play-outline" class="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    :title="$t('repo.settings.crons.edit')"
                    @click="editCron(cron)"
                  >
                    <Icon name="edit" class="h-4 w-4" />
                  </Button>
                  <AlertDialog v-model:open="deleteDialogOpen[cron.id]">
                    <AlertDialogTrigger as-child>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        :title="$t('repo.settings.crons.delete')"
                        :disabled="isDeleting"
                      >
                        <Icon v-if="isDeleting" name="spinner" class="h-4 w-4 animate-spin" />
                        <Icon v-else name="trash" class="h-4 w-4 text-destructive" />
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent class="bg-background text-foreground">
                      <AlertDialogHeader>
                        <AlertDialogTitle class="text-foreground">
                          {{ $t('repo.settings.crons.delete') }}
                        </AlertDialogTitle>
                        <AlertDialogDescription class="text-muted-foreground">
                          {{ $t('repo.settings.crons.delete_confirm', { name: cron.name }) }}
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
                        <AlertDialogAction as-child>
                          <Button
                            variant="destructive"
                            class="text-destructive-foreground"
                            @click="confirmDelete(cron)"
                          >
                            {{ $t('repo.settings.crons.delete') }}
                          </Button>
                        </AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <!-- Empty State -->
      <div v-else class="text-sm text-muted-foreground py-8 text-center">
        {{ $t('repo.settings.crons.none') }}
      </div>
    </div>

    <!-- Edit Dialog -->
    <Dialog v-model:open="isEditDialogOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{{ $t('repo.settings.crons.edit') }}</DialogTitle>
        </DialogHeader>
        <form class="space-y-4" @submit.prevent="saveCron">
          <div class="space-y-2">
            <Label for="cron-name">{{ $t('repo.settings.crons.name.name') }}</Label>
            <Input
              id="cron-name"
              v-model="editingCron.name"
              :placeholder="$t('repo.settings.crons.name.placeholder')"
              required
            />
          </div>
          <div class="space-y-2">
            <Label for="cron-branch">{{ $t('repo.settings.crons.branch.title') }}</Label>
            <Input
              id="cron-branch"
              v-model="editingCron.branch"
              :placeholder="$t('repo.settings.crons.branch.placeholder')"
            />
          </div>
          <div class="space-y-2">
            <div class="flex items-center gap-2">
              <Label for="cron-schedule">{{ $t('repo.settings.crons.schedule.title') }}</Label>
              <a
                href="https://pkg.go.dev/github.com/gdgvda/cron#hdr-CRON_Expression_Format"
                target="_blank"
                rel="noopener noreferrer"
                class="text-xs text-muted-foreground hover:text-foreground"
              >
                <Icon name="lucide:external-link" class="h-3 w-3" />
              </a>
            </div>
            <Input
              id="cron-schedule"
              v-model="editingCron.schedule"
              :placeholder="$t('repo.settings.crons.schedule.placeholder')"
              required
            />
            <p class="text-xs text-muted-foreground">
              {{ $t('repo.settings.crons.timezone_note') }}
            </p>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" @click="isEditDialogOpen = false">
              {{ $t('cancel') }}
            </Button>
            <Button type="submit" :disabled="isSaving">
              <Icon v-if="isSaving" name="spinner" class="mr-2 h-4 w-4 animate-spin" />
              {{ $t('repo.settings.crons.save') }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  </Scaffold>
</template>

<script lang="ts" setup>
import { computed, defineComponent, h, nextTick, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '~/components/ui/alert-dialog';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import { useDate } from '~/compositions/useDate';
import { useGlobalDialogs } from '~/compositions/useGlobalDialogs';
import useNotifications from '~/compositions/useNotifications';
import { usePagination } from '~/compositions/usePaginate';
import { useResizableColumns } from '~/compositions/useResizableColumns';
import type { CronWithRepo, Pipeline } from '~/lib/api/types';

// Sort indicator component
interface SortConfig {
  key: string;
  direction: 'asc' | 'desc';
}

const SortIndicator = defineComponent({
  name: 'SortIndicator',
  props: {
    sortKey: { type: String, required: true },
    currentSort: { type: Object as () => SortConfig, required: true },
  },
  setup(props) {
    return () => {
      if (props.currentSort.key !== props.sortKey) {
        return h('span', { class: 'h-4 w-4 inline-flex items-center opacity-30' }, [
          h(Icon, { name: 'sort-ascending', class: 'h-3 w-3' }),
        ]);
      }
      return h(Icon, {
        name: props.currentSort.direction === 'asc' ? 'sort-ascending' : 'sort-descending',
        class: 'h-4 w-4',
      });
    };
  },
});

const apiClient = useApiClient();
const notifications = useNotifications();
const { t } = useI18n();
const router = useRouter();
const date = useDate();
const { cronDialog } = useGlobalDialogs();

const searchTerm = ref('');
const selectedOwners = ref<string[]>([]);
const selectedScheduleTypes = ref<string[]>([]);
const deleteDialogOpen = reactive<Record<number, boolean>>({});
const isEditDialogOpen = ref(false);
const editingCron = ref<Partial<CronWithRepo>>({});
const sortConfig = ref<SortConfig>({ key: 'name', direction: 'asc' });
const cronRecentRuns = ref<Record<number, Pipeline[]>>({});
const tableRef = ref<InstanceType<typeof Table> | null>(null);

// Column visibility (branch and recent_runs hidden by default to prevent mobile overflow)
const visibleColumns = ref<string[]>(['next_exec']);
const toggleableColumns = computed(() => [
  { key: 'branch', label: t('repo.settings.crons.branch.title') },
  { key: 'next_exec', label: t('repo.settings.crons.next_exec') },
  { key: 'recent_runs', label: t('repo.settings.crons.recent_runs') },
]);

function toggleColumn(key: string, visible: boolean) {
  if (visible) {
    visibleColumns.value = [...visibleColumns.value, key];
  } else {
    visibleColumns.value = visibleColumns.value.filter((k) => k !== key);
  }
}

// Resizable columns
const { init: initResizableColumns } = useResizableColumns(
  computed(() => (tableRef.value?.$el as HTMLElement) || null),
);

// Schedule type definitions
const scheduleTypes = [
  { value: '@yearly', label: '@yearly' },
  { value: '@monthly', label: '@monthly' },
  { value: '@weekly', label: '@weekly' },
  { value: '@daily', label: '@daily' },
  { value: '@hourly', label: '@hourly' },
  { value: 'custom', label: t('repo.settings.crons.schedule_custom') },
];

// Helper to categorize a schedule
function getScheduleType(schedule: string): string {
  const predefined = ['@yearly', '@monthly', '@weekly', '@daily', '@hourly'];
  if (predefined.includes(schedule)) {
    return schedule;
  }
  return 'custom';
}

async function loadCrons(page: number) {
  return apiClient.getUserCronList({
    page,
  });
}

const { resetPage, data: crons } = usePagination(loadCrons, () => !isEditDialogOpen.value);

// Refresh cron list when the global add dialog closes
watch(
  () => cronDialog.isOpen.value,
  (isOpen, wasOpen) => {
    if (wasOpen && !isOpen) {
      resetPage();
    }
  },
);

// Computed: unique owners for filter dropdown
const uniqueOwners = computed(() => {
  if (!crons.value) return [];
  // Filter out empty owners and get unique values
  const owners = new Set(crons.value.map((c) => c.repo_owner).filter((o) => o && o.trim()));
  return Array.from(owners).sort();
});

// Computed: filtered crons based on all filters
const filteredCrons = computed(() => {
  if (!crons.value) return [];

  const search = searchTerm.value.toLowerCase().trim();

  return crons.value.filter((c) => {
    // Search filter (name, repo_name, branch, schedule)
    if (search) {
      const searchableFields = [c.name, c.repo_name, c.repo_owner, c.branch, c.schedule]
        .filter(Boolean)
        .map((f) => f.toLowerCase());
      if (!searchableFields.some((f) => f.includes(search))) {
        return false;
      }
    }
    // Owner filter
    if (selectedOwners.value.length > 0 && !selectedOwners.value.includes(c.repo_owner)) {
      return false;
    }
    // Schedule type filter
    if (selectedScheduleTypes.value.length > 0) {
      const cronScheduleType = getScheduleType(c.schedule);
      if (!selectedScheduleTypes.value.includes(cronScheduleType)) {
        return false;
      }
    }
    return true;
  });
});

// Computed: sorted crons
const sortedCrons = computed(() => {
  const filtered = [...filteredCrons.value];
  const { key, direction } = sortConfig.value;
  const multiplier = direction === 'asc' ? 1 : -1;

  return filtered.sort((a, b) => {
    let aVal: string | number = '';
    let bVal: string | number = '';

    switch (key) {
      case 'name':
        aVal = a.name.toLowerCase();
        bVal = b.name.toLowerCase();
        break;
      case 'repo_name':
        aVal = a.repo_name.toLowerCase();
        bVal = b.repo_name.toLowerCase();
        break;
      case 'schedule':
        aVal = a.schedule.toLowerCase();
        bVal = b.schedule.toLowerCase();
        break;
      case 'next_exec':
        aVal = a.next_exec || 0;
        bVal = b.next_exec || 0;
        break;
      default:
        return 0;
    }

    if (aVal < bVal) return -1 * multiplier;
    if (aVal > bVal) return 1 * multiplier;
    return 0;
  });
});

// Check if any filters are active
const hasActiveFilters = computed(() => {
  return searchTerm.value.trim() !== '' || selectedOwners.value.length > 0 || selectedScheduleTypes.value.length > 0;
});

// Sort toggle
function toggleSort(key: string) {
  if (sortConfig.value.key === key) {
    sortConfig.value.direction = sortConfig.value.direction === 'asc' ? 'desc' : 'asc';
  } else {
    sortConfig.value = { key, direction: 'asc' };
  }
}

// Owner filter functions
function toggleOwnerFilter(owner: string, value: boolean) {
  if (value) {
    selectedOwners.value = [...selectedOwners.value, owner];
  } else {
    selectedOwners.value = selectedOwners.value.filter((o) => o !== owner);
  }
}

function clearOwnerFilters() {
  selectedOwners.value = [];
}

// Schedule type filter functions
function toggleScheduleTypeFilter(scheduleType: string, value: boolean) {
  if (value) {
    selectedScheduleTypes.value = [...selectedScheduleTypes.value, scheduleType];
  } else {
    selectedScheduleTypes.value = selectedScheduleTypes.value.filter((s) => s !== scheduleType);
  }
}

function clearScheduleTypeFilters() {
  selectedScheduleTypes.value = [];
}

// Clear all filters
function clearAllFilters() {
  searchTerm.value = '';
  selectedOwners.value = [];
  selectedScheduleTypes.value = [];
}

const { doSubmit: saveCronAction, isLoading: isSaving } = useAsyncAction(async () => {
  if (!editingCron.value.id || !editingCron.value.repo_id) {
    throw new Error("Unexpected: Can't get cron ID or repo ID");
  }

  await apiClient.updateCron(editingCron.value.repo_id, editingCron.value);

  notifications.notify({
    title: t('notify_title_success'),
    text: t('repo.settings.crons.notify_text_saved'),
    type: 'success',
  });

  isEditDialogOpen.value = false;
  editingCron.value = {};
  resetPage();
});

async function saveCron() {
  await saveCronAction();
}

const { doSubmit: deleteCronAction, isLoading: isDeleting } = useAsyncAction(async (cron: CronWithRepo) => {
  await apiClient.deleteCron(cron.repo_id, cron.id);

  notifications.notify({
    title: t('notify_title_success'),
    text: t('repo.settings.crons.notify_text_deleted'),
    type: 'success',
  });

  resetPage();
});

async function confirmDelete(cron: CronWithRepo) {
  deleteDialogOpen[cron.id] = false;
  await deleteCronAction(cron);
}

function editCron(cron: CronWithRepo) {
  editingCron.value = { ...cron };
  isEditDialogOpen.value = true;
}

const { doSubmit: runCronAction } = useAsyncAction(async (cron: CronWithRepo) => {
  const pipeline = await apiClient.runCron(cron.repo_id, cron.id);

  if (pipeline && typeof pipeline === 'object' && 'number' in pipeline) {
    await router.push({
      name: 'repo-pipeline',
      params: {
        repoId: cron.repo_id.toString(),
        pipelineId: pipeline.number.toString(),
      },
    });
  } else {
    notifications.notify({
      title: t('repo.settings.crons.run_success'),
      type: 'success',
    });
  }
});

async function runCron(cron: CronWithRepo) {
  await runCronAction(cron);
}

function navigateToCron(cron: CronWithRepo) {
  router.push({
    name: 'repo-settings-crons',
    params: { repoId: cron.repo_id.toString() },
  });
}

// Fetch recent runs for all crons (grouped by repo)
async function fetchAllRecentRuns() {
  if (!crons.value || crons.value.length === 0) {
    return;
  }

  // Group crons by repo_id
  const cronsByRepo = new Map<number, CronWithRepo[]>();
  for (const cron of crons.value) {
    const existing = cronsByRepo.get(cron.repo_id) || [];
    existing.push(cron);
    cronsByRepo.set(cron.repo_id, existing);
  }

  // Fetch pipelines for each repo in parallel
  const results = await Promise.all(
    Array.from(cronsByRepo.entries()).map(async ([repoId, repoCrons]) => {
      try {
        const pipelines = await apiClient.getPipelineList(repoId, { page: 1, perPage: 50 });
        if (!pipelines) return { repoId, repoCrons, pipelines: [] };

        // Filter to cron events only
        const cronPipelines = pipelines.filter((p) => p.event === 'cron');
        return { repoId, repoCrons, pipelines: cronPipelines };
      } catch {
        return { repoId, repoCrons, pipelines: [] };
      }
    }),
  );

  // Match pipelines to crons by name (message field contains cron name)
  const newRecentRuns: Record<number, Pipeline[]> = {};
  for (const { repoCrons, pipelines } of results) {
    // Group pipelines by cron name
    const pipelinesByCronName: Record<string, Pipeline[]> = {};
    for (const pipeline of pipelines) {
      const cronName = pipeline.message || '';
      if (!pipelinesByCronName[cronName]) {
        pipelinesByCronName[cronName] = [];
      }
      pipelinesByCronName[cronName].push(pipeline);
    }

    // Assign to each cron
    for (const cron of repoCrons) {
      const matching = pipelinesByCronName[cron.name] || [];
      if (matching.length > 0) {
        newRecentRuns[cron.id] = matching.slice(0, 5);
      }
    }
  }

  cronRecentRuns.value = newRecentRuns;
}

// Watch for crons changes and fetch recent runs
watch(
  crons,
  async (newCrons) => {
    if (newCrons && newCrons.length > 0) {
      await fetchAllRecentRuns();
      // Initialize resizable columns after table renders
      await nextTick();
      initResizableColumns();
    }
  },
  { immediate: true },
);

useCrowTitle(computed(() => [t('repo.settings.crons.all_crons')]));
</script>

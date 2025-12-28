<template>
  <Settings
    :title="$t('repo.settings.crons.crons')"
    :description="$t('repo.settings.crons.desc')"
    docs-url="docs/usage/cron"
  >
    <template #headerActions>
      <Button v-if="selectedCron" variant="outline" @click="selectedCron = undefined">
        <Icon name="back" class="mr-2 h-4 w-4" />
        {{ $t('repo.settings.crons.show') }}
      </Button>
      <Button v-else @click="selectedCron = {}">
        <Icon name="plus" class="mr-2 h-4 w-4" />
        {{ $t('repo.settings.crons.add') }}
      </Button>
    </template>

    <!-- Tabs for List/Calendar view (only show when not editing) -->
    <Tabs v-if="!selectedCron" v-model="activeTab" class="w-full max-w-full overflow-hidden">
      <TabsList class="hidden sm:grid w-full max-w-md grid-cols-2 mb-6">
        <TabsTrigger value="list">
          <Icon name="list" class="mr-2 h-4 w-4" />
          {{ $t('repo.settings.crons.list_view') }}
        </TabsTrigger>
        <TabsTrigger value="calendar">
          <Icon name="calendar" class="mr-2 h-4 w-4" />
          {{ $t('repo.settings.crons.calendar_view') }}
        </TabsTrigger>
      </TabsList>

      <!-- List View -->
      <TabsContent value="list" class="block -mx-4 sm:mx-0">
        <div v-if="crons && crons.length > 0" class="rounded-md border overflow-x-auto">
          <Table ref="tableRef" class="table-fixed">
            <TableHeader>
              <TableRow>
                <TableHead class="w-48 min-w-32">{{ $t('repo.settings.crons.name.name') }}</TableHead>
                <TableHead class="w-32 min-w-24">{{ $t('repo.settings.crons.schedule.title') }}</TableHead>
                <TableHead class="w-28 min-w-20">{{ $t('repo.settings.crons.branch.title') }}</TableHead>
                <TableHead class="w-40 min-w-32">{{ $t('repo.settings.crons.next_exec') }}</TableHead>
                <TableHead class="w-28 min-w-24">{{ $t('repo.settings.crons.recent_runs') }}</TableHead>
                <TableHead class="w-24 min-w-20 text-right">{{ $t('actions') }}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow
                v-for="cron in crons"
                :key="cron.id"
                class="cursor-pointer hover:bg-muted/50"
                @click="selectedCron = cron"
              >
                <TableCell class="font-medium" :title="cron.name">{{ cron.name }}</TableCell>
                <TableCell :title="cron.schedule">
                  <code class="text-xs bg-muted px-1.5 py-0.5 rounded">{{ cron.schedule }}</code>
                </TableCell>
                <TableCell :title="cron.branch || repo.default_branch || $t('repo.settings.crons.branch.default')">
                  <span class="text-sm">{{
                    cron.branch || repo.default_branch || $t('repo.settings.crons.branch.default')
                  }}</span>
                </TableCell>
                <TableCell
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
                <TableCell class="overflow-hidden" @click.stop>
                  <div
                    v-if="cronRecentRuns[cron.id]?.length"
                    class="flex items-center gap-1 max-w-full overflow-hidden"
                  >
                    <RouterLink
                      v-for="pipeline in cronRecentRuns[cron.id].slice(0, 5)"
                      :key="pipeline.id"
                      :to="{ name: 'repo-pipeline', params: { repoId: repo.id, pipelineId: pipeline.number } }"
                      class="hover:scale-125 transition-all hover:drop-shadow-md rounded-full p-0.5 hover:bg-accent"
                    >
                      <PipelineStatusIcon
                        :status="pipeline.status"
                        :title="`#${pipeline.number} - ${date.toISODateTimeString(new Date(pipeline.created * 1000))}`"
                        class="size-4"
                      />
                    </RouterLink>
                  </div>
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  <span v-else class="text-muted-foreground text-sm" aria-hidden="true">-</span>
                </TableCell>
                <TableCell class="text-right overflow-visible" @click.stop>
                  <div class="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="icon" :title="$t('repo.settings.crons.run')" @click="runCron(cron)">
                      <Icon name="play-outline" class="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      :title="$t('repo.settings.crons.edit')"
                      @click="selectedCron = cron"
                    >
                      <Icon name="edit" class="h-4 w-4" />
                    </Button>
                    <AlertDialog v-model:open="deleteDialogOpen[cron.id]">
                      <AlertDialogTrigger as-child>
                        <Button
                          variant="ghost"
                          size="icon"
                          :title="$t('repo.settings.crons.delete')"
                          :disabled="isDeleting"
                        >
                          <Icon v-if="isDeleting" name="spinner" class="h-4 w-4 animate-spin" />
                          <Icon v-else name="trash" class="h-4 w-4 text-destructive" />
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent class="bg-background text-foreground">
                        <AlertDialogHeader>
                          <AlertDialogTitle class="text-foreground">{{
                            $t('repo.settings.crons.delete')
                          }}</AlertDialogTitle>
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
        <div v-else class="text-sm text-muted-foreground py-8 text-center">
          {{ $t('repo.settings.crons.none') }}
        </div>
      </TabsContent>

      <!-- Calendar View -->
      <TabsContent value="calendar">
        <CronsCalendar
          :crons="crons || []"
          @cron-click="handleCronClick"
          @create-cron="handleCreateCronFromDate"
          @edit-cron="handleEditCron"
          @delete-cron="handleDeleteCron"
        />
      </TabsContent>
    </Tabs>

    <!-- Edit/Create Form (shown when selectedCron is set) -->
    <div v-else class="space-y-6">
      <form @submit.prevent="createCron">
        <div class="space-y-4">
          <div class="grid gap-2">
            <Label for="cron-name">{{ $t('repo.settings.crons.name.name') }}</Label>
            <Input
              id="cron-name"
              v-model="selectedCron.name"
              :placeholder="$t('repo.settings.crons.name.placeholder')"
              required
            />
          </div>

          <div class="grid gap-2">
            <Label for="cron-branch">{{ $t('repo.settings.crons.branch.title') }}</Label>
            <Input
              id="cron-branch"
              v-model="selectedCron.branch"
              :placeholder="$t('repo.settings.crons.branch.placeholder')"
            />
          </div>

          <div class="grid gap-2">
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
              v-model="selectedCron.schedule"
              :placeholder="$t('repo.settings.crons.schedule.placeholder')"
              required
            />
            <p class="text-xs text-muted-foreground">
              {{ $t('repo.settings.crons.timezone_note') }}
            </p>
          </div>

          <div v-if="isEditingCron" class="text-sm">
            <span v-if="selectedCron.next_exec && selectedCron.next_exec > 0">
              {{ $t('repo.settings.crons.next_exec') }}
              <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
              <span>: {{ date.toLocaleString(new Date(selectedCron.next_exec * 1000)) }}</span>
            </span>
            <span v-else>{{ $t('repo.settings.crons.not_executed_yet') }}</span>
          </div>

          <div class="flex gap-2">
            <Button type="button" variant="outline" @click="selectedCron = undefined">
              {{ $t('cancel') }}
            </Button>
            <Button type="submit" :disabled="isSaving">
              <Icon v-if="isSaving" name="spinner" class="mr-2 h-4 w-4 animate-spin" />
              {{ isEditingCron ? $t('repo.settings.crons.save') : $t('repo.settings.crons.add') }}
            </Button>
          </div>
        </div>
      </form>
    </div>

    <!-- Quick Create Dialog -->
    <Dialog v-model:open="quickCreateDialogOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{{ $t('repo.settings.crons.add') }}</DialogTitle>
          <DialogDescription>
            {{ $t('repo.settings.crons.desc') }}
          </DialogDescription>
        </DialogHeader>
        <form class="space-y-4" @submit.prevent="createQuickCron">
          <div class="space-y-2">
            <Label for="quick-cron-name">{{ $t('repo.settings.crons.name.name') }}</Label>
            <Input
              id="quick-cron-name"
              v-model="quickCreateForm.name"
              :placeholder="$t('repo.settings.crons.name.placeholder')"
              required
            />
          </div>
          <div class="space-y-2">
            <Label for="quick-cron-schedule">{{ $t('repo.settings.crons.schedule.title') }}</Label>
            <Input
              id="quick-cron-schedule"
              v-model="quickCreateForm.schedule"
              :placeholder="$t('repo.settings.crons.schedule.placeholder')"
              required
            />
            <p class="text-xs text-muted-foreground">{{ $t('repo.settings.crons.timezone_note') }}</p>
          </div>
          <div class="space-y-2">
            <Label for="quick-cron-branch">{{ $t('repo.settings.crons.branch.title') }}</Label>
            <Input
              id="quick-cron-branch"
              v-model="quickCreateForm.branch"
              :placeholder="$t('repo.settings.crons.branch.placeholder')"
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" @click="quickCreateDialogOpen = false">
              {{ $t('cancel') }}
            </Button>
            <Button type="submit">
              {{ $t('repo.settings.crons.save') }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>

    <!-- Shared Delete Confirmation Dialog (for context menu deletes) -->
    <AlertDialog v-if="cronToDelete" v-model:open="sharedDeleteDialogOpen">
      <AlertDialogContent class="bg-background text-foreground">
        <AlertDialogHeader>
          <AlertDialogTitle class="text-foreground">{{ $t('repo.settings.crons.delete') }}</AlertDialogTitle>
          <AlertDialogDescription class="text-muted-foreground">
            {{ $t('repo.settings.crons.delete_confirm', { name: cronToDelete.name }) }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
          <AlertDialogAction as-child>
            <Button variant="destructive" class="text-destructive-foreground" @click="confirmDelete(cronToDelete)">
              {{ $t('repo.settings.crons.delete') }}
            </Button>
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </Settings>
</template>

<script lang="ts" setup>
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { RouterLink, useRoute, useRouter } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
import Settings from '~/components/layout/Settings.vue';
import CronsCalendar from '~/components/repo/cron/CronsCalendar.vue';
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
import { Button } from '~/components/ui/button';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '~/components/ui/tabs';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import { useDate } from '~/compositions/useDate';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import { usePagination } from '~/compositions/usePaginate';
import { useResizableColumns } from '~/compositions/useResizableColumns';
import type { Cron, Pipeline } from '~/lib/api/types';
import router from '~/router';

const apiClient = useApiClient();
const notifications = useNotifications();
const i18n = useI18n();
const route = useRoute();
const routerInstance = useRouter();

const repo = requiredInject('repo');
const selectedCron = ref<Partial<Cron>>();
const isEditingCron = computed(() => !!selectedCron.value?.id);
const date = useDate();
const deleteDialogOpen = reactive<Record<number, boolean>>({});
const activeTab = ref('list');
const sharedDeleteDialogOpen = ref(false);
const cronToDelete = ref<Cron | null>(null);
const cronRecentRuns = ref<Record<number, Pipeline[]>>({});
const tableRef = ref<InstanceType<typeof Table> | null>(null);

// Resizable columns
const { init: initResizableColumns } = useResizableColumns(
  computed(() => (tableRef.value?.$el as HTMLElement) || null),
);

// Initialize activeTab from URL hash
onMounted(() => {
  const hash = route.hash.replace('#', '');
  if (hash === 'list' || hash === 'calendar') {
    activeTab.value = hash;
  }
});

// Update URL hash when activeTab changes
watch(activeTab, (newTab) => {
  routerInstance.replace({ hash: `#${newTab}` });
});

async function loadCrons(page: number): Promise<Cron[] | null> {
  return apiClient.getCronList(repo.value.id, { page });
}

const { resetPage, data: crons } = usePagination(loadCrons, () => !selectedCron.value);

const { doSubmit: createCron, isLoading: isSaving } = useAsyncAction(async () => {
  if (!selectedCron.value) {
    throw new Error("Unexpected: Can't get cron");
  }

  if (isEditingCron.value) {
    await apiClient.updateCron(repo.value.id, selectedCron.value);
  } else {
    await apiClient.createCron(repo.value.id, selectedCron.value);
  }
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: isEditingCron.value
      ? i18n.t('repo.settings.crons.notify_text_saved')
      : i18n.t('repo.settings.crons.notify_text_created'),
    type: 'success',
    duration: 6000,
  });
  selectedCron.value = undefined;
  resetPage();
});

const { doSubmit: deleteCronAction, isLoading: isDeleting } = useAsyncAction(async (_cron: Cron) => {
  await apiClient.deleteCron(repo.value.id, _cron.id);
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('repo.settings.crons.notify_text_deleted'),
    type: 'success',
    duration: 6000,
  });
  resetPage();
});

async function confirmDelete(cron: Cron) {
  deleteDialogOpen[cron.id] = false;
  sharedDeleteDialogOpen.value = false;
  cronToDelete.value = null;
  await deleteCronAction(cron);
}

const { doSubmit: runCron } = useAsyncAction(async (_cron: Cron) => {
  const pipeline = await apiClient.runCron(repo.value.id, _cron.id);

  // If we got a valid pipeline, navigate to it
  if (pipeline && typeof pipeline === 'object' && 'number' in pipeline) {
    await router.push({
      name: 'repo-pipeline',
      params: {
        repoId: repo.value.id.toString(),
        pipelineId: pipeline.number.toString(),
      },
    });
  } else {
    // Show success notification if navigation isn't possible
    notifications.notify({
      title: i18n.t('repo.settings.crons.run_success'),
      type: 'success',
    });
  }
});

function handleCronClick(cron: Cron) {
  selectedCron.value = cron;
}

function handleEditCron(cron: Cron) {
  selectedCron.value = cron;
  activeTab.value = 'list'; // Switch to list view to show the edit form
}

function handleDeleteCron(cron: Cron) {
  // Open the shared delete dialog for this cron
  cronToDelete.value = cron;
  sharedDeleteDialogOpen.value = true;
}

const quickCreateDialogOpen = ref(false);
const quickCreateForm = reactive({
  name: '',
  schedule: '@daily',
  branch: '',
});

function handleCreateCronFromDate() {
  // Reset form and open quick create dialog
  quickCreateForm.name = '';
  quickCreateForm.schedule = '@daily';
  quickCreateForm.branch = '';
  quickCreateDialogOpen.value = true;
}

async function createQuickCron() {
  const cron: Partial<Cron> = {
    name: quickCreateForm.name,
    schedule: quickCreateForm.schedule,
    branch: quickCreateForm.branch,
  };

  const newCron = await apiClient.createCron(repo.value.id, cron as Cron);
  crons.value = [...(crons.value || []), newCron];

  notifications.notify({
    title: i18n.t('repo.settings.crons.notify_text_created'),
    type: 'success',
  });

  quickCreateDialogOpen.value = false;
}

async function fetchAllRecentRuns() {
  try {
    if (!crons.value || crons.value.length === 0) {
      return;
    }

    // Fetch ALL pipelines (like calendar view does)
    const allPipelines = await apiClient.getPipelineList(repo.value.id, {
      page: 1,
      perPage: 100,
    });

    if (!allPipelines || allPipelines.length === 0) {
      return;
    }

    // Filter to only cron events
    const cronPipelines = allPipelines.filter((p) => p.event === 'cron');

    if (cronPipelines.length === 0) {
      return;
    }

    // Group pipelines by cron name (message field - same as calendar view)
    const pipelinesByCronName: Record<string, Pipeline[]> = {};
    for (const pipeline of cronPipelines) {
      const cronName = pipeline.message || ''; // Use message field like calendar view does
      if (!pipelinesByCronName[cronName]) {
        pipelinesByCronName[cronName] = [];
      }
      pipelinesByCronName[cronName].push(pipeline);
    }

    // Assign pipelines to each cron
    cronRecentRuns.value = {};
    for (const cron of crons.value) {
      const matchingPipelines = pipelinesByCronName[cron.name] || [];
      if (matchingPipelines.length > 0) {
        cronRecentRuns.value[cron.id] = matchingPipelines.slice(0, 5);
      }
    }
  } catch (error) {
    console.error('Failed to fetch recent runs for crons:', error);
  }
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
  { immediate: true, deep: true },
);

useCrowTitle(computed(() => [i18n.t('repo.settings.crons.crons'), repo.value.full_name]));
</script>

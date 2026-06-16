<template>
  <div class="space-y-4">
    <!-- Calendar Header -->
    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-1">
          <Button
            variant="outline"
            size="icon"
            class="size-7 bg-transparent p-0 opacity-50 hover:opacity-100"
            @click="previousMonth"
          >
            <Icon name="chevron-left" class="h-4 w-4" />
          </Button>
          <Button
            variant="outline"
            size="icon"
            class="size-7 bg-transparent p-0 opacity-50 hover:opacity-100"
            @click="nextMonth"
          >
            <Icon name="chevron-right" class="h-4 w-4" />
          </Button>
        </div>
        <div class="flex items-center gap-1">
          <Button variant="ghost" size="sm" class="h-6 px-2 text-xs" @click="goToToday">
            {{ $t('repo.settings.crons.today') }}
          </Button>
        </div>
      </div>
      <div class="flex items-center justify-center gap-1 pb-1">
        <Select v-model="selectedMonth">
          <SelectTrigger class="h-7 text-xs w-[110px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="(month, index) in monthNames" :key="index" :value="index.toString()">
              {{ month }}
            </SelectItem>
          </SelectContent>
        </Select>
        <Select v-model="selectedYear">
          <SelectTrigger class="h-7 text-xs w-[80px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="year in availableYears" :key="year" :value="year.toString()">
              {{ year }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
    </div>

    <!-- Calendar Grid -->
    <div class="rounded-lg border overflow-x-auto -mx-4 sm:mx-0">
      <div class="min-w-full sm:min-w-[640px]">
        <!-- Day headers -->
        <div class="grid grid-cols-7 border-b bg-muted/50">
          <div v-for="day in weekDays" :key="day" class="p-1 sm:p-2 text-center text-xs sm:text-sm font-medium">
            {{ day }}
          </div>
        </div>

        <!-- Calendar days -->
        <div class="grid grid-cols-7">
          <ContextMenu v-for="(day, index) in calendarDays" :key="index">
            <ContextMenuTrigger as-child>
              <div
                class="min-h-[100px] sm:min-h-[120px] border-b border-r p-1 sm:p-2 last:border-r-0"
                :class="{
                  'bg-muted/30': !day.isCurrentMonth,
                  'bg-accent/20': day.isToday,
                }"
              >
                <div class="flex flex-col h-full">
                  <div
                    class="text-xs sm:text-sm font-medium mb-1 sm:mb-2"
                    :class="{
                      'text-muted-foreground': !day.isCurrentMonth,
                      'text-accent-foreground': day.isToday,
                    }"
                  >
                    {{ day.day }}
                  </div>

                  <!-- Cron executions for this day -->
                  <div class="space-y-1 flex-1 overflow-y-auto">
                    <!-- Group view: show grouped crons if many occurrences -->
                    <template v-if="getGroupedCrons(day.cronOccurrences).grouped.length > 0">
                      <ContextMenu
                        v-for="group in getGroupedCrons(day.cronOccurrences).grouped"
                        :key="`${group.cron.id}-group`"
                      >
                        <ContextMenuTrigger as-child>
                          <div @contextmenu.stop>
                            <Popover>
                              <PopoverTrigger as-child>
                                <button
                                  class="w-full text-left text-xs px-2 py-1 rounded bg-primary/10 hover:bg-primary/20 text-foreground border transition-colors truncate"
                                  :class="getGroupBorderColor(day.cronOccurrences, group.cron.id)"
                                >
                                  <Icon name="stopwatch" class="h-3 w-3 inline mr-1 text-primary" />
                                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                                  <span class="font-medium text-primary">{{ group.count }}×</span>
                                  <span class="ml-1">{{ group.cron.name }}</span>
                                </button>
                              </PopoverTrigger>
                              <PopoverContent
                                class="w-[90vw] sm:w-80 max-w-sm p-0 bg-background opacity-100"
                                align="start"
                              >
                                <div class="p-3 border-b bg-background">
                                  <h4 class="font-semibold">{{ group.cron.name }}</h4>
                                  <p class="text-xs text-muted-foreground mt-1">
                                    {{ $t('repo.settings.crons.runs_today', { count: group.count }) }}
                                  </p>
                                </div>
                                <ScrollArea class="h-[300px]">
                                  <div class="p-2 space-y-1">
                                    <div
                                      v-for="(occurrence, idx) in getGroupOccurrences(
                                        day.cronOccurrences,
                                        group.cron.id,
                                      )"
                                      :key="`${occurrence.cron.id}-${idx}`"
                                    >
                                      <RouterLink
                                        v-if="occurrence.pipeline"
                                        :to="{
                                          name: 'repo-pipeline',
                                          params: {
                                            repoId: repoId.toString(),
                                            pipelineId: occurrence.pipeline.number.toString(),
                                          },
                                        }"
                                        class="flex items-center gap-2 px-2 py-1.5 rounded text-xs hover:bg-accent transition-colors"
                                      >
                                        <Icon
                                          :name="getStatusIcon(occurrence.pipeline.status)"
                                          class="h-3 w-3 shrink-0"
                                          :class="getStatusColor(occurrence.pipeline.status).text"
                                        />
                                        <span
                                          class="font-medium"
                                          :class="getStatusColor(occurrence.pipeline.status).text"
                                        >
                                          {{ formatTimeOnly(occurrence.time) }}
                                        </span>
                                        <span class="text-muted-foreground text-[10px] ml-auto">
                                          <!-- eslint-disable-next-line @intlify/vue-i18n/no-dynamic-keys -->
                                          {{ $t(`repo.pipeline.status.${occurrence.pipeline.status}`) }}
                                        </span>
                                      </RouterLink>
                                      <div v-else class="flex items-center gap-2 px-2 py-1.5 rounded text-xs">
                                        <Icon name="stopwatch" class="h-3 w-3 shrink-0 text-muted-foreground" />
                                        <span class="font-medium text-muted-foreground">
                                          {{ formatTimeOnly(occurrence.time) }}
                                        </span>
                                        <span class="text-muted-foreground text-[10px] ml-auto">
                                          {{ $t('repo.settings.crons.scheduled_run') }}
                                        </span>
                                      </div>
                                    </div>
                                  </div>
                                </ScrollArea>
                              </PopoverContent>
                            </Popover>
                          </div>
                        </ContextMenuTrigger>
                        <ContextMenuContent>
                          <ContextMenuItem @click="emit('editCron', group.cron)">
                            <Icon name="edit" class="mr-2 h-4 w-4" />
                            {{ $t('repo.settings.crons.edit') }}
                          </ContextMenuItem>
                          <ContextMenuSeparator />
                          <ContextMenuItem class="text-destructive" @click="emit('deleteCron', group.cron)">
                            <Icon name="trash" class="mr-2 h-4 w-4" />
                            {{ $t('repo.settings.crons.delete') }}
                          </ContextMenuItem>
                        </ContextMenuContent>
                      </ContextMenu>
                    </template>

                    <!-- Individual occurrences (limited to 5) -->
                    <ContextMenu
                      v-for="(occurrence, idx) in getGroupedCrons(day.cronOccurrences).individual.slice(0, 5)"
                      :key="`${occurrence.cron.id}-${idx}`"
                    >
                      <ContextMenuTrigger as-child>
                        <div @contextmenu.stop>
                          <TooltipProvider>
                            <Tooltip>
                              <TooltipTrigger as-child>
                                <RouterLink
                                  v-if="occurrence.pipeline"
                                  :to="getPipelineUrl(occurrence.pipeline)!"
                                  class="w-full text-left text-xs px-2 py-1 rounded hover:opacity-80 border transition-colors truncate block"
                                  :class="[
                                    getStatusColor(occurrence.pipeline.status).bg,
                                    getStatusColor(occurrence.pipeline.status).border,
                                  ]"
                                >
                                  <Icon
                                    :name="getStatusIcon(occurrence.pipeline.status)"
                                    class="h-3 w-3 inline mr-1"
                                    :class="getStatusColor(occurrence.pipeline.status).text"
                                  />
                                  <span class="font-medium" :class="getStatusColor(occurrence.pipeline.status).text">
                                    {{ formatTimeOnly(occurrence.time) }}
                                  </span>
                                  <Icon
                                    v-if="(occurrence.cron as any)._isManual"
                                    name="play-outline"
                                    class="h-3 w-3 inline mx-1 text-blue-500"
                                  />
                                  <span class="ml-1 text-foreground">{{ occurrence.cron.name }}</span>
                                </RouterLink>
                                <button
                                  v-else
                                  class="w-full text-left text-xs px-2 py-1 rounded hover:opacity-80 border transition-colors truncate"
                                  :class="[getStatusColor(undefined).bg, getStatusColor(undefined).border]"
                                  @click="$emit('cronClick', occurrence.cron)"
                                >
                                  <Icon
                                    name="stopwatch"
                                    class="h-3 w-3 inline mr-1"
                                    :class="getStatusColor(undefined).text"
                                  />
                                  <span class="font-medium" :class="getStatusColor(undefined).text">
                                    {{ formatTimeOnly(occurrence.time) }}
                                  </span>
                                  <span class="ml-1 text-foreground">{{ occurrence.cron.name }}</span>
                                </button>
                              </TooltipTrigger>
                              <TooltipContent side="top" class="max-w-xs">
                                <div class="space-y-1">
                                  <p class="font-semibold">{{ occurrence.cron.name }}</p>
                                  <p
                                    v-if="(occurrence.cron as any)._isManual"
                                    class="text-xs font-medium text-blue-500"
                                  >
                                    <Icon name="play-outline" class="h-3 w-3 inline mr-1" />
                                    {{ $t('repo.settings.crons.manually_triggered') }}
                                  </p>
                                  <p v-if="occurrence.pipeline" class="text-xs font-medium">
                                    {{ $t('repo.pipeline.metadata.fields.status') }}
                                    <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                                    <span>: </span>
                                    <span :class="getStatusColor(occurrence.pipeline.status).text">
                                      <!-- eslint-disable-next-line @intlify/vue-i18n/no-dynamic-keys -->
                                      {{ $t(`repo.pipeline.status.${occurrence.pipeline.status}`) }}
                                    </span>
                                  </p>
                                  <p v-else class="text-xs text-muted-foreground">
                                    {{ $t('repo.settings.crons.scheduled_run') }}
                                  </p>
                                  <p class="text-xs text-muted-foreground">
                                    {{ $t('repo.settings.crons.branch.title') }}
                                    <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                                    <span
                                      >:
                                      {{ occurrence.cron.branch || $t('repo.settings.crons.branch.placeholder') }}</span
                                    >
                                    <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                                  </p>
                                  <p class="text-xs text-muted-foreground">
                                    {{ $t('repo.settings.crons.schedule.title') }}
                                    <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                                    <span>: {{ occurrence.cron.schedule }}</span>
                                  </p>
                                  <p class="text-xs font-medium">
                                    {{ d(occurrence.time, 'short') }}
                                  </p>
                                </div>
                              </TooltipContent>
                            </Tooltip>
                          </TooltipProvider>
                        </div>
                      </ContextMenuTrigger>
                      <ContextMenuContent>
                        <ContextMenuItem @click="emit('editCron', occurrence.cron)">
                          <Icon name="edit" class="mr-2 h-4 w-4" />
                          {{ $t('repo.settings.crons.edit') }}
                        </ContextMenuItem>
                        <ContextMenuSeparator />
                        <ContextMenuItem class="text-destructive" @click="emit('deleteCron', occurrence.cron)">
                          <Icon name="trash" class="mr-2 h-4 w-4" />
                          {{ $t('repo.settings.crons.delete') }}
                        </ContextMenuItem>
                      </ContextMenuContent>
                    </ContextMenu>

                    <!-- Show "X more" indicator if there are more occurrences -->
                    <Popover v-if="getGroupedCrons(day.cronOccurrences).individual.length > 5">
                      <PopoverTrigger as-child>
                        <button
                          class="w-full text-xs text-muted-foreground text-center py-1 hover:text-foreground hover:bg-accent rounded transition-colors"
                        >
                          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                          <span>+{{ getGroupedCrons(day.cronOccurrences).individual.length - 5 }} </span>
                          {{ $t('repo.settings.crons.more') }}
                        </button>
                      </PopoverTrigger>
                      <PopoverContent class="w-[90vw] sm:w-80 max-w-sm p-0 bg-background opacity-100" align="start">
                        <div class="p-3 border-b bg-background">
                          <h4 class="font-semibold">{{ $t('repo.settings.crons.more') }}</h4>
                          <p class="text-xs text-muted-foreground mt-1">
                            {{ getGroupedCrons(day.cronOccurrences).individual.length }}
                            {{ $t('repo.settings.crons.scheduled_run') }}
                          </p>
                        </div>
                        <ScrollArea class="h-[300px]">
                          <div class="p-2 space-y-1">
                            <div
                              v-for="(occurrence, idx) in getGroupedCrons(day.cronOccurrences).individual.slice(5)"
                              :key="`${occurrence.cron.id}-more-${idx}`"
                            >
                              <RouterLink
                                v-if="occurrence.pipeline"
                                :to="{
                                  name: 'repo-pipeline',
                                  params: {
                                    repoId: repoId.toString(),
                                    pipelineId: occurrence.pipeline.number.toString(),
                                  },
                                }"
                                class="flex items-center gap-2 px-2 py-1.5 rounded text-xs hover:bg-accent transition-colors"
                              >
                                <Icon
                                  :name="getStatusIcon(occurrence.pipeline.status)"
                                  class="h-3 w-3 shrink-0"
                                  :class="getStatusColor(occurrence.pipeline.status).text"
                                />
                                <span class="font-medium" :class="getStatusColor(occurrence.pipeline.status).text">
                                  {{ formatTimeOnly(occurrence.time) }}
                                </span>
                                <span class="ml-1 flex-1 truncate">{{ occurrence.cron.name }}</span>
                                <span class="text-muted-foreground text-[10px]">
                                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-dynamic-keys -->
                                  {{ $t(`repo.pipeline.status.${occurrence.pipeline.status}`) }}
                                </span>
                              </RouterLink>
                              <div v-else class="flex items-center gap-2 px-2 py-1.5 rounded text-xs">
                                <Icon name="stopwatch" class="h-3 w-3 shrink-0 text-muted-foreground" />
                                <span class="font-medium text-muted-foreground">
                                  {{ formatTimeOnly(occurrence.time) }}
                                </span>
                                <span class="ml-1 flex-1 truncate text-muted-foreground">{{
                                  occurrence.cron.name
                                }}</span>
                                <span class="text-muted-foreground text-[10px]">
                                  {{ $t('repo.settings.crons.scheduled_run') }}
                                </span>
                              </div>
                            </div>
                          </div>
                        </ScrollArea>
                      </PopoverContent>
                    </Popover>
                  </div>
                </div>
              </div>
            </ContextMenuTrigger>
            <ContextMenuContent>
              <ContextMenuItem @click="emit('createCron', day.date)">
                <Icon name="plus" class="mr-2 h-4 w-4" />
                {{ $t('repo.settings.crons.add') }}
              </ContextMenuItem>
            </ContextMenuContent>
          </ContextMenu>
        </div>
      </div>
    </div>

    <!-- Legend -->
    <div class="flex flex-wrap items-center gap-3 sm:gap-4 text-xs sm:text-sm text-muted-foreground">
      <div class="flex items-center gap-2">
        <div class="w-4 h-4 rounded bg-green-500/20 border border-green-500/50"></div>
        <span>{{ $t('repo.pipeline.status.success') }}</span>
      </div>
      <div class="flex items-center gap-2">
        <div class="w-4 h-4 rounded bg-red-500/20 border border-red-500/50"></div>
        <span>{{ $t('repo.pipeline.status.failure') }}</span>
      </div>
      <div class="flex items-center gap-2">
        <Icon name="play-outline" class="h-4 w-4 text-blue-500" />
        <span>{{ $t('repo.settings.crons.manually_triggered') }}</span>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, RouterLink } from 'vue-router';
import { CronExpressionParser } from 'cron-parser';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import { ScrollArea } from '~/components/ui/scroll-area';
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '~/components/ui/context-menu';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import useApiClient from '~/compositions/useApiClient';
import type { Cron, Pipeline, PipelineStatus } from '~/lib/api/types';

interface CalendarDay {
  day: number;
  date: Date;
  isCurrentMonth: boolean;
  isToday: boolean;
  cronOccurrences: CronOccurrence[];
}

interface CronWithCount extends Cron {
  _actualCount?: number; // Metadata for actual occurrence count
}

interface CronOccurrence {
  cron: CronWithCount;
  time: Date;
  pipeline?: Pipeline; // Actual pipeline run if it exists
}

interface Props {
  crons: Cron[];
}

const props = defineProps<Props>();

const emit = defineEmits<{
  (event: 'cronClick', cron: Cron): void;
  (event: 'createCron', date: Date): void;
  (event: 'editCron', cron: Cron): void;
  (event: 'deleteCron', cron: Cron): void;
}>();

const { d } = useI18n();
const route = useRoute();
const apiClient = useApiClient();

const repoId = computed(() => Number.parseInt(route.params.repoId as string, 10));

const currentDate = ref(new Date());

// Month and year selectors
const selectedMonth = ref(currentDate.value.getMonth().toString());
const selectedYear = ref(currentDate.value.getFullYear().toString());

// Generate list of months
const monthNames = computed(() => {
  return Array.from({ length: 12 }, (_, i) => {
    const date = new Date(2024, i, 1);
    return d(date, { month: 'long' });
  });
});

// Generate available years (current year ± 5 years)
const availableYears = computed(() => {
  const currentYear = new Date().getFullYear();
  const years: number[] = [];
  for (let year = currentYear - 5; year <= currentYear + 5; year++) {
    years.push(year);
  }
  return years;
});

// Watch for changes in month/year selectors and update currentDate
watch([selectedMonth, selectedYear], ([newMonth, newYear]) => {
  const month = Number.parseInt(newMonth, 10);
  const year = Number.parseInt(newYear, 10);
  currentDate.value = new Date(year, month, 1);
});

// Watch currentDate changes and update selectors (for arrow buttons)
watch(currentDate, (newDate) => {
  selectedMonth.value = newDate.getMonth().toString();
  selectedYear.value = newDate.getFullYear().toString();
});

// Cache for cron occurrences to avoid recalculating
const occurrenceCache = ref(new Map<string, CronOccurrence[]>());

// Cache for pipeline runs
const pipelineCache = ref(new Map<string, Pipeline[]>());

// Clear cache when crons change or when pipelines are fetched
watch(
  () => props.crons,
  () => {
    occurrenceCache.value.clear();
    pipelineCache.value.clear();
  },
  { deep: true },
);

// Clear occurrence cache when pipeline cache changes (new pipelines fetched)
watch(
  pipelineCache,
  () => {
    occurrenceCache.value.clear();
  },
  { deep: true },
);

// Fetch pipelines for the current month
async function fetchPipelinesForMonth(): Promise<void> {
  const year = currentDate.value.getFullYear();
  const month = currentDate.value.getMonth();
  const monthStart = new Date(year, month, 1);
  const monthEnd = new Date(year, month + 1, 0, 23, 59, 59, 999);

  const cacheKey = `${monthStart.getTime()}-${monthEnd.getTime()}`;

  // Check cache first
  if (pipelineCache.value.has(cacheKey)) {
    return;
  }

  try {
    // Fetch all cron pipelines for this repository
    const pipelines = await apiClient.getPipelineList(
      repoId.value,
      {
        page: 1,
        perPage: 500, // Get a large number to cover the month
      },
      { silent: true },
    );

    // Filter to only cron events within the current month
    const cronPipelines = pipelines.filter((p) => {
      if (p.event !== 'cron') return false;
      const pipelineDate = new Date(p.created * 1000);
      return pipelineDate >= monthStart && pipelineDate <= monthEnd;
    });

    pipelineCache.value.set(cacheKey, cronPipelines);
  } catch (error) {
    console.error('Failed to fetch pipelines:', error);
    pipelineCache.value.set(cacheKey, []);
  }
}

// Fetch pipelines when month changes (only if month/year actually changed)
watch(
  () => ({ month: currentDate.value.getMonth(), year: currentDate.value.getFullYear() }),
  () => {
    fetchPipelinesForMonth();
  },
  { immediate: true },
);

const weekDays = computed(() => {
  // Get localized short weekday names starting from Sunday
  const baseDate = new Date(2024, 0, 7); // Sunday
  return Array.from({ length: 7 }, (_, i) => {
    const day = new Date(baseDate);
    day.setDate(baseDate.getDate() + i);
    return d(day, 'short').split(',')[0]; // Get just the day name
  });
});

const calendarDays = computed<CalendarDay[]>(() => {
  const year = currentDate.value.getFullYear();
  const month = currentDate.value.getMonth();

  // First day of the month
  const firstDay = new Date(year, month, 1);
  const firstDayOfWeek = firstDay.getDay(); // 0 = Sunday

  // Last day of the month
  const lastDay = new Date(year, month + 1, 0);
  const lastDate = lastDay.getDate();

  // Last day of previous month
  const prevMonthLastDay = new Date(year, month, 0);
  const prevMonthLastDate = prevMonthLastDay.getDate();

  const days: CalendarDay[] = [];
  const today = new Date();
  today.setHours(0, 0, 0, 0);

  // Previous month days
  for (let i = firstDayOfWeek - 1; i >= 0; i--) {
    const day = prevMonthLastDate - i;
    const date = new Date(year, month - 1, day);
    days.push({
      day,
      date,
      isCurrentMonth: false,
      isToday: date.getTime() === today.getTime(),
      cronOccurrences: getCronOccurrencesForDate(date),
    });
  }

  // Current month days
  for (let day = 1; day <= lastDate; day++) {
    const date = new Date(year, month, day);
    days.push({
      day,
      date,
      isCurrentMonth: true,
      isToday: date.getTime() === today.getTime(),
      cronOccurrences: getCronOccurrencesForDate(date),
    });
  }

  // Next month days to complete the grid
  const remainingDays = 42 - days.length; // 6 rows * 7 days
  for (let day = 1; day <= remainingDays; day++) {
    const date = new Date(year, month + 1, day);
    days.push({
      day,
      date,
      isCurrentMonth: false,
      isToday: date.getTime() === today.getTime(),
      cronOccurrences: getCronOccurrencesForDate(date),
    });
  }

  return days;
});

function getCronOccurrencesForDate(date: Date): CronOccurrence[] {
  const dayStart = new Date(date);
  dayStart.setHours(0, 0, 0, 0);
  const dayEnd = new Date(date);
  dayEnd.setHours(23, 59, 59, 999);

  // Create cache key from date and cron IDs
  const cacheKey = `${dayStart.getTime()}-${props.crons.map((c) => c.id).join(',')}`;

  // Check cache first
  if (occurrenceCache.value.has(cacheKey)) {
    return occurrenceCache.value.get(cacheKey)!;
  }

  // Get pipelines for this month
  const year = currentDate.value.getFullYear();
  const month = currentDate.value.getMonth();
  const monthStart = new Date(year, month, 1);
  const monthEnd = new Date(year, month + 1, 0, 23, 59, 59, 999);
  const monthCacheKey = `${monthStart.getTime()}-${monthEnd.getTime()}`;
  const pipelines = pipelineCache.value.get(monthCacheKey) || [];

  // Check if this is a past day
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const isPastDay = dayStart < today;

  // Strategy: Sample each cron to get a representative count
  // For frequent crons (>10 runs/day), we only need to know the count, not every occurrence
  const cronSamples = new Map<number, { cron: Cron; count: number; samples: Date[] }>();

  // Only parse cron schedules for future/today days
  if (!isPastDay) {
    for (const cron of props.crons) {
      if (!cron.schedule) continue;

      try {
        // Start parsing from 1 millisecond before the day to catch occurrences at exactly midnight
        const parseStart = new Date(dayStart);
        parseStart.setMilliseconds(parseStart.getMilliseconds() - 1);

        const interval = CronExpressionParser.parse(cron.schedule, {
          currentDate: parseStart,
          endDate: dayEnd,
          tz: 'UTC',
        });

        const samples: Date[] = [];
        let totalCount = 0;
        const maxSamples = 50; // Store more samples for popover display
        let iterations = 0;

        while (iterations < 100) {
          // Max 100 iterations per cron to prevent hangs
          iterations++;
          try {
            const next = interval.next();
            const nextDate = next.toDate();

            // Stop if past the day
            if (nextDate > dayEnd) {
              break;
            }

            // Count if within this day
            if (nextDate >= dayStart && nextDate <= dayEnd) {
              totalCount++;
              // Store up to maxSamples for display in popover
              if (samples.length < maxSamples) {
                samples.push(nextDate);
              }
            }
          } catch {
            break;
          }
        }

        if (totalCount > 0) {
          cronSamples.set(cron.id, { cron, count: totalCount, samples });
        }
      } catch (error) {
        console.error(`Failed to parse cron expression for "${cron.name}": ${cron.schedule}`, error);
      }
    }
  }

  // Build final occurrence list with metadata
  const occurrences: CronOccurrence[] = [];
  const matchedPipelines = new Set<number>(); // Track which pipelines we've matched

  if (!isPastDay) {
    // For future/today: show scheduled runs with matched pipelines
    for (const { cron, count, samples } of cronSamples.values()) {
      // Use all available samples (up to maxSamples)
      // They will be displayed in the popover for grouped items or individually for ungrouped
      const samplesToUse = samples;

      for (const time of samplesToUse) {
        // Try to find a matching pipeline run for this scheduled time
        // Match within ±5 minutes of the scheduled time
        const matchWindow = 5 * 60 * 1000; // 5 minutes in milliseconds
        const matchingPipeline = pipelines.find((p) => {
          // Check if pipeline matches this cron by name (stored in message field)
          const pipelineCronName = p.message;
          if (pipelineCronName !== cron.name) return false;

          // Check if pipeline was created within ±5 minutes of scheduled time
          const pipelineTime = p.created * 1000; // Convert to milliseconds
          const timeDiff = Math.abs(pipelineTime - time.getTime());
          return timeDiff <= matchWindow;
        });

        if (matchingPipeline) {
          matchedPipelines.add(matchingPipeline.id);
        }

        occurrences.push({
          cron: { ...cron, _actualCount: count }, // Store actual count for grouping
          time,
          pipeline: matchingPipeline,
        });
      }
    }
  } else {
    // For past days: only show actual executed pipelines (no scheduled runs)
    for (const cron of props.crons) {
      if (!cron.schedule) continue;

      // Get all pipelines for this cron
      const allCronPipelines = pipelines.filter((p) => {
        // Match by cron name
        if (p.message !== cron.name) return false;

        // Check if pipeline was executed on this day
        const pipelineDate = new Date(p.created * 1000);
        return pipelineDate >= dayStart && pipelineDate <= dayEnd;
      });

      // Check each pipeline to see if it matches the schedule
      for (const pipeline of allCronPipelines) {
        matchedPipelines.add(pipeline.id);

        // Check if this pipeline was executed on schedule or manually
        const pipelineTime = new Date(pipeline.created * 1000);
        let isScheduled = false;

        // Parse the cron schedule to see if this time matches
        try {
          const parseStart = new Date(dayStart);
          parseStart.setMilliseconds(parseStart.getMilliseconds() - 1);

          const interval = CronExpressionParser.parse(cron.schedule, {
            currentDate: parseStart,
            endDate: dayEnd,
            tz: 'UTC',
          });

          // Check if pipeline execution time matches any scheduled time (±5 minutes)
          const matchWindow = 5 * 60 * 1000;
          let iterations = 0;

          while (iterations < 100) {
            iterations++;
            try {
              const next = interval.next();
              const nextDate = next.toDate();

              if (nextDate > dayEnd) break;

              if (nextDate >= dayStart && nextDate <= dayEnd) {
                const timeDiff = Math.abs(pipelineTime.getTime() - nextDate.getTime());
                if (timeDiff <= matchWindow) {
                  isScheduled = true;
                  break;
                }
              }
            } catch {
              break;
            }
          }
        } catch {
          // If we can't parse the schedule, assume it's scheduled
          isScheduled = true;
        }

        occurrences.push({
          cron: { ...cron, _actualCount: allCronPipelines.length, _isManual: !isScheduled } as any,
          time: pipelineTime,
          pipeline,
        });
      }
    }
  }

  // Add unmatched pipelines (from deleted crons or manually triggered crons)
  // Only add them if they don't match any existing cron by name
  const activeCronNames = new Set(props.crons.map((c) => c.name));

  for (const pipeline of pipelines) {
    if (!matchedPipelines.has(pipeline.id)) {
      const pipelineCronName = pipeline.message;

      // If this pipeline's cron name still exists in active crons, skip it
      // (it's just a manual trigger or off-schedule run, not a deleted cron)
      if (activeCronNames.has(pipelineCronName)) {
        continue;
      }

      // This pipeline is from a deleted cron - show it
      const syntheticCron: CronWithCount = {
        id: -pipeline.id, // Negative ID to indicate it's synthetic
        repo_id: 0, // Synthetic cron from deleted job
        name: pipelineCronName || 'Unknown',
        branch: pipeline.branch,
        schedule: '', // No schedule for deleted crons
        next_exec: 0,
        fail_count: 0,
        fail_msg: '',
        disabled: false,
      };

      occurrences.push({
        cron: syntheticCron,
        time: new Date(pipeline.created * 1000),
        pipeline,
      });
    }
  }

  // Sort by time
  const sorted = occurrences.sort((a, b) => a.time.getTime() - b.time.getTime());

  // Cache the result
  occurrenceCache.value.set(cacheKey, sorted);

  return sorted;
}

interface GroupedCrons {
  grouped: Array<{ cron: Cron; count: number }>;
  individual: CronOccurrence[];
}

function getGroupedCrons(occurrences: CronOccurrence[]): GroupedCrons {
  // Group occurrences by cron
  const cronGroups = new Map<number, { occurrences: CronOccurrence[]; actualCount: number }>();

  for (const occurrence of occurrences) {
    const cronId = occurrence.cron.id;
    const actualCount = (occurrence.cron as any)._actualCount || 1;

    if (!cronGroups.has(cronId)) {
      cronGroups.set(cronId, { occurrences: [], actualCount });
    }
    cronGroups.get(cronId)!.occurrences.push(occurrence);
  }

  const grouped: Array<{ cron: Cron; count: number }> = [];
  const individual: CronOccurrence[] = [];

  // If a cron has more than 3 actual occurrences, group it; otherwise show individual times
  for (const { occurrences: cronOccurrences, actualCount } of cronGroups.values()) {
    if (actualCount > 3) {
      grouped.push({
        cron: cronOccurrences[0].cron,
        count: actualCount, // Use actual count, not sample count
      });
    } else {
      // Show individual occurrences for crons with 3 or fewer runs
      individual.push(...cronOccurrences);
    }
  }

  // Sort individual occurrences by time, but prioritize showing at least one from each cron
  // First, get one occurrence from each unique cron
  const cronsSeen = new Set<number>();
  const prioritized: CronOccurrence[] = [];
  const remaining: CronOccurrence[] = [];

  const sorted = individual.sort((a, b) => a.time.getTime() - b.time.getTime());

  for (const occurrence of sorted) {
    if (!cronsSeen.has(occurrence.cron.id)) {
      cronsSeen.add(occurrence.cron.id);
      prioritized.push(occurrence);
    } else {
      remaining.push(occurrence);
    }
  }

  // Return prioritized first, then remaining
  const reordered = [...prioritized, ...remaining];

  return { grouped, individual: reordered };
}

function getGroupOccurrences(occurrences: CronOccurrence[], cronId: number): CronOccurrence[] {
  // Get all occurrences for a specific cron, sorted by time
  return occurrences.filter((o) => o.cron.id === cronId).sort((a, b) => a.time.getTime() - b.time.getTime());
}

function getGroupBorderColor(occurrences: CronOccurrence[], cronId: number): string {
  // Get all occurrences for this group
  const groupOccurrences = occurrences.filter((o) => o.cron.id === cronId);

  // Check if any have a successful status
  const hasSuccess = groupOccurrences.some((o) => o.pipeline?.status === 'success');
  const hasFailure = groupOccurrences.some((o) => o.pipeline?.status === 'failure' || o.pipeline?.status === 'error');

  if (hasSuccess) {
    return 'border-green-500/50';
  } else if (hasFailure) {
    return 'border-red-500/50';
  }

  // Default border color
  return 'border-primary/20';
}

function formatTimeOnly(date: Date): string {
  return d(date, { hour: 'numeric', minute: '2-digit', hour12: false });
}

function getStatusColor(status?: PipelineStatus): { bg: string; border: string; text: string } {
  if (!status) {
    // Scheduled but not yet executed - use neutral colors
    return {
      bg: 'bg-primary/10',
      border: 'border-primary/20',
      text: 'text-primary',
    };
  }

  switch (status) {
    case 'success':
      return {
        bg: 'bg-green-500/10',
        border: 'border-green-500/30',
        text: 'text-green-600 dark:text-green-500',
      };
    case 'failure':
    case 'error':
      return {
        bg: 'bg-red-500/10',
        border: 'border-red-500/30',
        text: 'text-red-600 dark:text-red-500',
      };
    case 'running':
    case 'started':
      return {
        bg: 'bg-blue-500/10',
        border: 'border-blue-500/30',
        text: 'text-blue-600 dark:text-blue-500',
      };
    case 'pending':
      return {
        bg: 'bg-yellow-500/10',
        border: 'border-yellow-500/30',
        text: 'text-yellow-600 dark:text-yellow-500',
      };
    case 'killed':
    case 'blocked':
    case 'declined':
      return {
        bg: 'bg-gray-500/10',
        border: 'border-gray-500/30',
        text: 'text-gray-600 dark:text-gray-500',
      };
    case 'skipped':
      return {
        bg: 'bg-gray-400/10',
        border: 'border-gray-400/30',
        text: 'text-gray-500 dark:text-gray-400',
      };
    default:
      return {
        bg: 'bg-primary/10',
        border: 'border-primary/20',
        text: 'text-primary',
      };
  }
}

function getPipelineUrl(pipeline?: Pipeline): string | null {
  if (!pipeline) return null;
  return `/repos/${repoId.value}/pipelines/${pipeline.number}`;
}

function getStatusIcon(status?: PipelineStatus): 'status-success' | 'status-running' | 'close' | 'stopwatch' {
  if (!status) return 'stopwatch';

  switch (status) {
    case 'success':
      return 'status-success';
    case 'running':
    case 'started':
      return 'status-running';
    case 'failure':
    case 'error':
      return 'close';
    default:
      return 'stopwatch';
  }
}

function previousMonth() {
  currentDate.value = new Date(currentDate.value.getFullYear(), currentDate.value.getMonth() - 1, 1);
  // Clear cache when changing months
  occurrenceCache.value.clear();
}

function nextMonth() {
  currentDate.value = new Date(currentDate.value.getFullYear(), currentDate.value.getMonth() + 1, 1);
  // Clear cache when changing months
  occurrenceCache.value.clear();
}

function goToToday() {
  currentDate.value = new Date();
  // Clear cache when jumping to today
  occurrenceCache.value.clear();
}
</script>

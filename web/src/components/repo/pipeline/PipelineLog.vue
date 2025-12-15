<template>
  <component
    :is="isFullscreen ? 'div' : 'div'"
    v-if="pipeline"
    :class="isFullscreen ? 'fixed inset-0 z-50 bg-background' : 'flex flex-col h-full'"
  >
    <Card
      class="flex flex-col h-full overflow-hidden p-0"
      :class="[logThemeClass, isFullscreen ? 'rounded-none border-0' : '']"
    >
      <!-- Desktop header - hidden on mobile since we have a mobile header in the parent -->
      <CardHeader class="hidden md:flex flex-row items-center justify-between px-3 py-2 border-b space-y-0 gap-3">
        <!-- Left: Navigation buttons -->
        <div class="flex items-center gap-2">
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :disabled="!previousStepId"
                  @click="emit('update:step-id', previousStepId!)"
                >
                  <Icon name="chevron-up" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ $t('repo.pipeline.previous_step') }}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :disabled="!nextStepId"
                  @click="emit('update:step-id', nextStepId!)"
                >
                  <Icon name="chevron-down" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ $t('repo.pipeline.next_step') }}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>

        <!-- Center: Step name, status, and exit code -->
        <div class="flex items-center gap-2 flex-1 min-w-0">
          <PipelineStatusIcon v-if="step" :status="step.state" class="h-4 w-4 shrink-0" />
          <span class="text-sm font-semibold text-foreground truncate">
            {{ step?.name }}
          </span>
          <span
            v-if="step?.exit_code !== undefined"
            class="text-xs border rounded-full px-2 py-0.5 shrink-0"
            :class="step.exit_code === 0 ? 'border-green-500' : 'border-destructive'"
          >
            {{ $t('repo.pipeline.exit_code', { exitCode: step.exit_code }) }}
          </span>
        </div>

        <!-- Right: Action buttons -->
        <div class="flex flex-row items-center gap-x-2 shrink-0">
          <TooltipProvider v-if="step?.finished !== undefined && hasLogs">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" :disabled="downloadInProgress" @click="download">
                  <Icon v-if="downloadInProgress" name="spinner" class="animate-spin" />
                  <Icon v-else name="download" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{ $t('repo.pipeline.actions.log_download') }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <TooltipProvider v-if="step?.finished !== undefined && hasLogs && hasPushPermission">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" @click="deleteLogs">
                  <Icon name="trash" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{ $t('repo.pipeline.actions.log_delete') }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <TooltipProvider v-if="step?.finished === undefined">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" @click="autoScroll = !autoScroll">
                  <Icon :name="autoScroll ? 'auto-scroll' : 'auto-scroll-off'" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{
                  autoScroll
                    ? $t('repo.pipeline.actions.log_auto_scroll_off')
                    : $t('repo.pipeline.actions.log_auto_scroll')
                }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" @click="openSearch">
                  <Icon name="lucide:search" class="w-4 h-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ $t('repo.pipeline.search.title') }}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" @click="isFullscreen = !isFullscreen">
                  <Icon :name="isFullscreen ? 'lucide:minimize' : 'lucide:maximize'" class="w-4 h-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{
                  isFullscreen
                    ? $t('repo.pipeline.actions.log_exit_fullscreen')
                    : $t('repo.pipeline.actions.log_fullscreen')
                }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <Popover v-model:open="settingsPopoverOpen">
            <PopoverTrigger as-child>
              <Button variant="ghost" size="icon-sm">
                <Icon name="settings" class="w-4 h-4" />
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" class="w-80">
              <div class="space-y-5 p-1">
                <div>
                  <h4 class="text-sm font-semibold">{{ $t('repo.pipeline.actions.log_settings') }}</h4>
                </div>

                <div class="space-y-2.5">
                  <label class="text-sm font-medium block">{{ $t('user.settings.general.log_theme') }}</label>
                  <Select v-model="logTheme">
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem v-for="option in logThemeOptions" :key="option.value" :value="option.value">
                        {{ option.text }}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <div class="space-y-2.5">
                  <label class="text-sm font-medium block">{{
                    $t('user.settings.general.pipeline_steps_width')
                  }}</label>
                  <div class="flex items-center gap-3">
                    <NumberField v-model="pipelineSplitRatioTemp" :min="20" :max="50" class="max-w-[8rem]">
                      <NumberFieldContent>
                        <NumberFieldDecrement />
                        <NumberFieldInput />
                        <NumberFieldIncrement />
                      </NumberFieldContent>
                    </NumberField>
                    <span class="text-sm text-muted-foreground">
                      {{ $t('user.settings.general.split_ratio_range') }}
                    </span>
                  </div>
                </div>

                <div class="flex items-center justify-between">
                  <div class="space-y-0.5">
                    <label class="text-sm font-medium text-foreground">{{
                      $t('repo.pipeline.actions.auto_follow')
                    }}</label>
                    <p class="text-xs text-muted-foreground">
                      {{ $t('repo.pipeline.actions.auto_follow_desc') }}
                    </p>
                  </div>
                  <Switch
                    :model-value="autoFollowEnabled"
                    @update:model-value="(value) => emit('update:auto-follow-enabled', value)"
                  />
                </div>
              </div>
            </PopoverContent>
          </Popover>
        </div>
      </CardHeader>

      <!-- Mobile action buttons - foldable floating buttons in top-right corner -->
      <div class="md:hidden absolute top-2 right-2 z-10">
        <!-- Toggle button to show/hide action buttons -->
        <Button
          variant="outline"
          size="icon-sm"
          class="rounded-full shadow-lg mb-2"
          @click="mobileButtonsVisible = !mobileButtonsVisible"
        >
          <Icon name="settings" class="w-4 h-4" />
        </Button>

        <!-- Floating action buttons -->
        <div v-show="mobileButtonsVisible" class="flex flex-col gap-2">
          <Button variant="outline" size="icon-sm" class="rounded-full shadow-lg" @click="openSearch">
            <Icon name="lucide:search" />
          </Button>
          <Button variant="outline" size="icon-sm" class="rounded-full shadow-lg" @click="scrollToBottom">
            <Icon name="chevron-down" />
          </Button>
          <Button variant="outline" size="icon-sm" class="rounded-full shadow-lg" @click="scrollToTop">
            <Icon name="chevron-up" />
          </Button>
          <Button
            v-if="step?.finished !== undefined && hasLogs"
            variant="outline"
            size="icon-sm"
            class="rounded-full shadow-lg"
            :disabled="downloadInProgress"
            @click="download"
          >
            <Icon v-if="downloadInProgress" name="spinner" class="animate-spin" />
            <Icon v-else name="download" />
          </Button>
          <Button
            v-if="step?.finished !== undefined && hasLogs && hasPushPermission"
            variant="outline"
            size="icon-sm"
            class="rounded-full shadow-lg"
            @click="deleteLogs"
          >
            <Icon name="trash" />
          </Button>
          <Button
            v-if="step?.finished === undefined"
            variant="outline"
            size="icon-sm"
            class="rounded-full shadow-lg"
            @click="autoScroll = !autoScroll"
          >
            <Icon :name="autoScroll ? 'auto-scroll' : 'auto-scroll-off'" />
          </Button>
        </div>
      </div>

      <!-- Log area with floating scroll buttons wrapper -->
      <CardContent v-show="hasLogs && loadedLogs && (log?.length || 0) > 0" class="relative flex-1 p-0">
        <!-- Floating search bar -->
        <Transition
          enter-active-class="transition-all duration-200 ease-out"
          enter-from-class="opacity-0 -translate-y-2"
          enter-to-class="opacity-100 translate-y-0"
          leave-active-class="transition-all duration-150 ease-in"
          leave-from-class="opacity-100 translate-y-0"
          leave-to-class="opacity-0 -translate-y-2"
        >
          <div
            v-if="searchOpen"
            class="absolute top-2 left-2 right-2 z-30 flex items-center gap-2 rounded-lg border bg-background/95 backdrop-blur-sm px-3 py-2 shadow-lg"
          >
            <Icon name="lucide:search" class="h-4 w-4 text-muted-foreground shrink-0" />
            <Input
              ref="searchInputRef"
              v-model="searchTerm"
              type="text"
              :placeholder="$t('repo.pipeline.search.placeholder')"
              class="h-8 flex-1 border-0 bg-transparent focus-visible:ring-0 focus-visible:ring-offset-0"
              @keydown.stop
            />
            <span v-if="searchTerm" class="text-xs text-muted-foreground whitespace-nowrap">
              <template v-if="searchMatches.length > 0">
                {{
                  $t('repo.pipeline.search.match_count', {
                    current: currentMatchIndex + 1,
                    total: searchMatches.length,
                  })
                }}
              </template>
              <template v-else>
                {{ $t('repo.pipeline.search.no_results') }}
              </template>
            </span>
            <div v-if="searchTerm && searchMatches.length > 0" class="flex items-center gap-1">
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon-sm" @click="goToPreviousMatch">
                      <Icon name="chevron-up" class="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{{ $t('repo.pipeline.search.previous') }}</TooltipContent>
                </Tooltip>
              </TooltipProvider>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon-sm" @click="goToNextMatch">
                      <Icon name="chevron-down" class="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{{ $t('repo.pipeline.search.next') }}</TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button variant="ghost" size="icon-sm" @click="closeSearch">
                    <Icon name="close" class="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{{ $t('close') }} {{ $t('repo.pipeline.search.close_hint') }}</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
        </Transition>

        <!-- Floating scroll buttons -->
        <div v-if="isScrollable" class="hidden md:flex flex-col gap-2 absolute bottom-4 right-4 z-20">
          <TooltipProvider v-if="!atTop">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="outline" size="icon-sm" class="rounded-full shadow-lg" @click="scrollToTop">
                  <Icon name="chevron-up" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{ $t('repo.pipeline.actions.log_scroll_to_top') }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <TooltipProvider v-if="!atBottom">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="outline" size="icon-sm" class="rounded-full shadow-lg" @click="scrollToBottom">
                  <Icon name="chevron-down" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{ $t('repo.pipeline.actions.log_scroll_to_bottom') }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>

        <!-- ScrollArea wrapping the log grid -->
        <ScrollArea ref="scrollAreaRef" class="h-full w-full">
          <div
            class="grid max-h-[calc(100vh-210px)] w-full auto-rows-min grid-cols-[min-content_minmax(0,1fr)_min-content] px-3 py-2 text-xs"
          >
            <div v-for="line in log" :key="line.index" class="contents font-mono">
              <a
                :id="`L${line.number}`"
                :href="`#L${line.number}`"
                class="select-none whitespace-nowrap pl-2 pr-6 text-right text-muted-foreground"
                :class="{
                  'bg-red-600/40 dark:bg-red-800/50': line.type === 'error',
                  'bg-yellow-600/40 dark:bg-yellow-800/50': line.type === 'warning',
                  'bg-blue-600/30': isSelected(line),
                  underline: isSelected(line),
                }"
              >
                {{ line.number }}
              </a>
              <!-- eslint-disable vue/no-v-html -->
              <span
                class="whitespace-pre-wrap wrap-break-word align-top"
                :class="{
                  'bg-red-600/40 dark:bg-red-800/50': line.type === 'error',
                  'bg-yellow-600/40 dark:bg-yellow-800/50': line.type === 'warning',
                  'bg-blue-600/30': isSelected(line),
                  'ring-2 ring-yellow-500 ring-offset-1': isCurrentSearchMatch(line),
                }"
                v-html="highlightSearchTerm(line.text || '')"
              />
              <!-- eslint-enable vue/no-v-html -->
              <span
                class="select-none whitespace-nowrap pr-1 text-right text-muted-foreground"
                :class="{
                  'bg-red-600 bg-opacity-40 dark:bg-red-800/50': line.type === 'error',
                  'bg-yellow-600/40 dark:bg-yellow-800/50': line.type === 'warning',
                  'bg-blue-600/30': isSelected(line),
                }"
              >
                {{ formatTime(line.time) }}
              </span>
            </div>
          </div>
        </ScrollArea>
      </CardContent>

      <CardContent class="m-auto text-xl text-muted-foreground">
        <span v-if="step?.state === 'skipped'">{{ $t('repo.pipeline.actions.canceled') }}</span>
        <span v-else-if="!step?.started">{{ $t('repo.pipeline.step_not_started') }}</span>
        <div v-else-if="!loadedLogs">{{ $t('repo.pipeline.loading') }}</div>

        <div v-else-if="logsDeleted">{{ $t('repo.pipeline.logs_deleted') }}</div>
        <div v-else-if="log?.length === 0">{{ $t('repo.pipeline.no_logs') }}</div>
      </CardContent>

      <!-- Stderr Error Output Section -->
      <div
        v-if="stderrLines.length > 0"
        data-stderr-section
        class="border-t border-destructive bg-red-50 dark:bg-red-950/20"
      >
        <button
          class="w-full flex items-center justify-between px-3 py-2 hover:bg-red-100 dark:hover:bg-red-900/30 transition-colors"
          @click="stderrExpanded = !stderrExpanded"
        >
          <div class="flex items-center gap-2">
            <Icon name="alert" class="w-4 h-4 text-destructive dark:text-red-500" />
            <span class="text-sm font-semibold text-destructive dark:text-red-500">
              {{ $t('repo.pipeline.actions.stderr_output') }}
            </span>
            <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
            <span class="text-sm font-semibold text-destructive dark:text-red-500">({{ stderrLines.length }})</span>
          </div>
          <Icon
            :name="stderrExpanded ? 'chevron-up' : 'chevron-down'"
            class="w-4 h-4 text-destructive dark:text-red-500"
          />
        </button>
        <div v-show="stderrExpanded" class="relative px-3 pb-2">
          <Button
            variant="ghost"
            size="icon-sm"
            class="absolute top-2 right-5 text-destructive dark:text-red-500 hover:bg-red-100 dark:hover:bg-red-900/30 bg-white dark:bg-black/40"
            @click="copyStderr"
          >
            <Icon v-if="stderrCopied" name="lucide:check" class="w-4 h-4" />
            <Icon v-else name="lucide:copy" class="w-4 h-4" />
          </Button>
          <div class="max-h-96 overflow-y-auto">
            <!-- eslint-disable vue/no-v-html -->
            <pre
              class="text-xs font-mono bg-white dark:bg-black/20 rounded px-2 py-2 pr-10 border border-red-200 dark:border-red-800 text-destructive dark:text-red-500 whitespace-pre-wrap wrap-break-word"
              v-html="highlightSearchTerm(stderrLines.map((e) => e.text).join('\n'))"
            />
            <!-- eslint-enable vue/no-v-html -->
          </div>
        </div>
      </div>

      <div v-if="step?.error" class="text-xs flex w-full items-center gap-2 border-t px-3 py-1.5 text-foreground">
        <span class="border border-destructive rounded-full px-2 py-0.5">{{ step.error }}</span>
      </div>
    </Card>
  </component>
</template>

<script lang="ts" setup>
import '~/style/console.css';

import { useStorage } from '@vueuse/core';
import { AnsiUp } from 'ansi_up';
import { decode } from 'js-base64';
import { debounce } from 'lodash';
import { computed, nextTick, onBeforeUnmount, onMounted, ref, toRef, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import { Button } from '~/components/ui/button';
import { Card, CardContent, CardHeader } from '~/components/ui/card';
import {
  NumberField,
  NumberFieldContent,
  NumberFieldDecrement,
  NumberFieldIncrement,
  NumberFieldInput,
} from '~/components/ui/number-field';
import { Input } from '~/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import { ScrollArea } from '~/components/ui/scroll-area';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Switch } from '~/components/ui/switch';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import useApiClient from '~/compositions/useApiClient';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import type { Pipeline, PipelineStep, PipelineWorkflow } from '~/lib/api/types';

interface LogLine {
  index: number;
  number: number;
  text?: string;
  time?: number;
  type: 'error' | 'warning' | null;
}

const props = withDefaults(
  defineProps<{
    pipeline: Pipeline;
    stepId: number;
    previousStepId?: number | null;
    nextStepId?: number | null;
    autoFollowEnabled?: boolean;
    class?: string;
  }>(),
  {
    autoFollowEnabled: false,
    class: '',
  },
);

const emit = defineEmits<{
  (event: 'update:step-id', stepId: number | null): true;
  (event: 'update:auto-follow-enabled', value: boolean): true;
}>();

const notifications = useNotifications();
const i18n = useI18n();
const pipeline = toRef(props, 'pipeline');
const stepId = toRef(props, 'stepId');
const repo = requiredInject('repo');
const repoPermissions = requiredInject('repo-permissions');
const apiClient = useApiClient();
const route = useRoute();

const loadedStepSlug = ref<string>();
const stepSlug = computed(() => `${repo?.value.owner} - ${repo?.value.name} - ${pipeline.value.id} - ${stepId.value}`);
const step = computed(() => pipeline.value && findStep(pipeline.value.workflows || [], stepId.value));
const stream = ref<EventSource>();
const log = ref<LogLine[]>();
const scrollAreaRef = ref<any>();
const consoleElement = computed(() => {
  // Get the viewport element from ScrollArea component
  if (!scrollAreaRef.value) return undefined;

  // The ScrollArea component uses data-slot="scroll-area-viewport"
  const viewport = scrollAreaRef.value.$el?.querySelector('[data-slot="scroll-area-viewport"]');

  return viewport as HTMLElement | undefined;
});
const logsDeleted = computed(() => pipeline.value.logs_deleted);

const loadedLogs = computed(() => !!log.value);
const hasLogs = computed(
  () =>
    // we do not have logs for skipped steps
    repo?.value && pipeline.value && step.value && step.value.state !== 'skipped',
);
const autoScroll = useStorage('crow:log-auto-scroll', false);
const downloadInProgress = ref(false);
const ansiUp = ref(new AnsiUp());
ansiUp.value.use_classes = true;
const logBuffer = ref<LogLine[]>([]);

const maxLineCount = 5000; // TODO(2653): set back to 500 and implement lazy-loading support
const hasPushPermission = computed(() => repoPermissions?.value?.push);

const mobileButtonsVisible = ref(false);
const atTop = ref(true);
const atBottom = ref(false);
const isScrollable = ref(false);
const isFullscreen = ref(false);
const stderrExpanded = ref(true);
const stderrCopied = ref(false);

// Search state
const searchOpen = ref(false);
const searchTerm = ref('');
const currentMatchIndex = ref(0);
const searchInputRef = ref<HTMLInputElement | null>(null);

// Extract stderr lines (error output) from logs
const stderrLines = computed(() => {
  if (!log.value || log.value.length === 0 || step.value?.exit_code === 0) {
    return [];
  }

  // Filter lines that are marked as stderr (type === 'error')
  const errors = log.value
    .filter((line) => line.type === 'error')
    .map((line) => ({
      line: line.number,
      text: (line.text?.replace(/<[^>]*>/g, '') || '').trim(), // Strip HTML
    }))
    .filter((error) => error.text.length > 0); // Remove empty lines

  return errors;
});

// Search: find all matching line indices
const searchMatches = computed(() => {
  if (!searchTerm.value || searchTerm.value.length === 0) {
    return [];
  }

  const term = searchTerm.value.toLowerCase();
  const matches: { type: 'log' | 'stderr'; index: number; lineNumber: number }[] = [];

  // Search main logs
  if (log.value) {
    log.value.forEach((line, idx) => {
      // Strip HTML tags and check for match
      const plainText = (line.text?.replace(/<[^>]*>/g, '') || '').toLowerCase();
      if (plainText.includes(term)) {
        matches.push({ type: 'log', index: idx, lineNumber: line.number });
      }
    });
  }

  // Search stderr lines (only if stderr section is expanded)
  if (stderrExpanded.value && stderrLines.value.length > 0) {
    stderrLines.value.forEach((line, idx) => {
      if (line.text.toLowerCase().includes(term)) {
        matches.push({ type: 'stderr', index: idx, lineNumber: line.line });
      }
    });
  }

  return matches;
});

// Current match for display
const currentMatch = computed(() => {
  if (searchMatches.value.length === 0) return null;
  const idx = Math.max(0, Math.min(currentMatchIndex.value, searchMatches.value.length - 1));
  return searchMatches.value[idx];
});

// Highlight search term in text (returns HTML with <mark> tags)
function highlightSearchTerm(text: string): string {
  if (!searchTerm.value || searchTerm.value.length === 0) {
    return text;
  }

  // Simple approach: search in the text and wrap matches
  // This works because the text already has HTML from ANSI conversion
  const regex = new RegExp(`(${escapeRegExp(searchTerm.value)})`, 'gi');
  return text.replace(regex, '<mark class="bg-yellow-300 dark:bg-yellow-700 text-inherit">$1</mark>');
}

// Helper to escape special regex characters
function escapeRegExp(string: string): string {
  return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// Check if a log line is the current search match
function isCurrentSearchMatch(line: LogLine): boolean {
  if (!currentMatch.value || currentMatch.value.type !== 'log') return false;
  return currentMatch.value.lineNumber === line.number;
}

const logTheme = useStorage('crow:log-theme', 'auto');
const logThemeClass = computed(() => {
  const theme = logTheme.value;
  if (theme === 'light') {
    return 'log-theme-light';
  }
  if (theme === 'dark') {
    return 'log-theme-dark';
  }
  // 'auto' - no additional class, follows system theme
  return '';
});

const logThemeOptions = computed(() => [
  { value: 'auto', text: i18n.t('user.settings.general.log_theme_system') },
  { value: 'light', text: i18n.t('user.settings.general.log_theme_light') },
  { value: 'dark', text: i18n.t('user.settings.general.log_theme_dark') },
]);

const pipelineSplitRatio = useStorage('crow:pipeline-sidebar-size', 30);
const pipelineSplitRatioTemp = ref(pipelineSplitRatio.value);
const settingsPopoverOpen = ref(false);

// Watch for popover opening to sync temp value
watch(settingsPopoverOpen, (isOpen) => {
  if (isOpen) {
    // When opening, sync temp value from storage
    pipelineSplitRatioTemp.value = pipelineSplitRatio.value;
  } else {
    // When closing, save temp value to storage
    pipelineSplitRatio.value = pipelineSplitRatioTemp.value;
  }
});

function isSelected(line: LogLine): boolean {
  return route.hash === `#L${line.number}`;
}

function formatTime(time?: number): string {
  return time === undefined ? '' : `${time}s`;
}

function processText(text: string): string {
  const urlRegex = /https?:\/\/\S+/g;
  let txt = ansiUp.value.ansi_to_html(`${decode(text)}\n`);
  txt = txt.replace(
    urlRegex,
    (url) => `<a href="${url}" target="_blank" rel="noopener noreferrer" class="underline">${url}</a>`,
  );
  return txt;
}

function writeLog(line: Partial<LogLine> & { logType?: number }) {
  // Map backend LogEntryType to frontend type
  // 0 = stdout (null), 1 = stderr ('error'), 2 = exit code, 3 = metadata, 4 = progress
  let type: 'error' | 'warning' | null = null;
  if (line.logType === 1) {
    type = 'error'; // stderr
  }

  logBuffer.value.push({
    index: line.index ?? 0,
    number: (line.index ?? 0) + 1,
    text: processText(line.text ?? ''),
    time: line.time ?? 0,
    type,
  });
}

function scrollToBottom() {
  nextTick(() => {
    const element = consoleElement.value;
    if (!element) {
      return;
    }

    if (element.scrollHeight <= element.clientHeight) {
      return;
    }

    element.scrollTop = element.scrollHeight;
  });
}

function scrollToTop() {
  const element = consoleElement.value;
  if (!element) {
    return;
  }

  if (element.scrollHeight <= element.clientHeight) {
    return;
  }

  element.scrollTop = 0;
}

const flushLogs = debounce((scroll: boolean) => {
  let buffer = logBuffer.value.slice(-maxLineCount);
  logBuffer.value = [];

  if (buffer.length === 0) {
    if (!log.value) {
      log.value = [];
    }
    return;
  }

  // append old logs lines
  if (buffer.length < maxLineCount && log.value) {
    buffer = [...log.value.slice(-(maxLineCount - buffer.length)), ...buffer];
  }

  // deduplicate repeating times
  buffer = buffer.reduce(
    (acc, line) => ({
      lastTime: line.time ?? 0,
      lines: [
        ...acc.lines,
        {
          ...line,
          time: acc.lastTime === line.time ? undefined : line.time,
        },
      ],
    }),
    { lastTime: -1, lines: [] as LogLine[] },
  ).lines;

  log.value = buffer;

  if (route.hash.length > 0) {
    nextTick(() => document.getElementById(route.hash.substring(1))?.scrollIntoView());
  } else if (scroll && autoScroll.value) {
    scrollToBottom();
  }
}, 500);

async function download() {
  if (!repo?.value || !pipeline.value || !step.value) {
    throw new Error('The repository, pipeline or step was undefined');
  }
  let logs;
  try {
    downloadInProgress.value = true;
    logs = await apiClient.getLogs(repo.value.id, pipeline.value.number, step.value.id);
  } catch {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      text: i18n.t('repo.pipeline.log_download_error'),
      type: 'error',
      duration: 9000,
    });
    return;
  } finally {
    downloadInProgress.value = false;
  }
  const fileURL = window.URL.createObjectURL(
    new Blob([logs.map((line) => decode(line.data ?? '')).join('\n')], {
      type: 'text/plain',
    }),
  );
  const fileLink = document.createElement('a');

  fileLink.href = fileURL;
  fileLink.setAttribute(
    'download',
    `${repo.value.owner}-${repo.value.name}-${pipeline.value.number}-${step.value.name}.log`,
  );
  document.body.appendChild(fileLink);

  fileLink.click();
  document.body.removeChild(fileLink);
  window.URL.revokeObjectURL(fileURL);
}

async function loadLogs() {
  if (loadedStepSlug.value === stepSlug.value) {
    return;
  }

  log.value = undefined;
  logBuffer.value = [];
  ansiUp.value = new AnsiUp();
  ansiUp.value.use_classes = true;

  stream.value?.close();

  if (!hasLogs.value || !step.value) {
    return;
  }

  if (step.value.state !== 'running' && step.value.state !== 'pending') {
    loadedStepSlug.value = stepSlug.value;
    const logs = await apiClient.getLogs(repo.value.id, pipeline.value.number, step.value.id);
    logs?.forEach((line) => writeLog({ index: line.line, text: line.data, time: line.time, logType: line.type }));
    flushLogs(false);
  } else {
    loadedStepSlug.value = stepSlug.value;
    stream.value = apiClient.streamLogs(repo.value.id, pipeline.value.number, step.value.id, (line) => {
      writeLog({ index: line.line, text: line.data, time: line.time, logType: line.type });
      flushLogs(true);
    });
  }
}

async function deleteLogs() {
  if (!repo?.value || !pipeline.value || !step.value) {
    throw new Error('The repository, pipeline or step was undefined');
  }

  // TODO: use proper dialog (copy-pasted from web/src/components/secrets/SecretList.vue:deleteSecret)
  // eslint-disable-next-line no-alert
  if (!confirm(i18n.t('repo.pipeline.log_delete_confirm'))) {
    return;
  }

  try {
    await apiClient.deleteLogs(repo.value.id, pipeline.value.number, step.value.id);
    log.value = [];
  } catch {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      text: i18n.t('repo.pipeline.log_delete_error'),
      type: 'error',
      duration: 9000,
    });
  }
}

async function copyStderr() {
  const text = stderrLines.value.map((e) => e.text).join('\n');
  try {
    await navigator.clipboard.writeText(text);
    stderrCopied.value = true;
    setTimeout(() => {
      stderrCopied.value = false;
    }, 2000);
  } catch {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      text: 'Failed to copy to clipboard',
      type: 'error',
      duration: 3000,
    });
  }
}

function findStep(workflows: PipelineWorkflow[], pid: number): PipelineStep | undefined {
  return workflows.reduce(
    (prev, workflow) => {
      const result = workflow.children.reduce(
        (prevChild, step) => {
          if (step.pid === pid) {
            return step;
          }

          return prevChild;
        },
        undefined as PipelineStep | undefined,
      );
      if (result) {
        return result;
      }

      return prev;
    },
    undefined as PipelineStep | undefined,
  );
}

function updateScrollState() {
  const el = consoleElement.value;
  if (!el) {
    return;
  }
  isScrollable.value = el.scrollHeight > el.clientHeight;
  atTop.value = el.scrollTop === 0;
  atBottom.value = el.scrollTop + el.clientHeight >= el.scrollHeight - 5; // more forgiving for rounding
}

// Debounced scroll handler for better performance
const debouncedUpdateScrollState = debounce(updateScrollState, 50);

onMounted(() => {
  nextTick(() => {
    const el = consoleElement.value;
    if (el) {
      // Use passive listener to improve scroll performance
      el.addEventListener('scroll', debouncedUpdateScrollState, { passive: true });
      updateScrollState();
    }
  });
});

watch(log, () => {
  nextTick(() => {
    updateScrollState();
    setTimeout(updateScrollState, 50); // ensure update after rendering
  });
});

onBeforeUnmount(() => {
  const el = consoleElement.value;
  if (el) {
    el.removeEventListener('scroll', debouncedUpdateScrollState);
  }
});

// Handle keyboard shortcuts
function handleKeydown(event: KeyboardEvent) {
  // Ctrl/Cmd+F to open search
  if ((event.metaKey || event.ctrlKey) && event.key === 'f') {
    event.preventDefault();
    openSearch();
    return;
  }

  // ESC to close search or exit fullscreen
  if (event.key === 'Escape') {
    if (searchOpen.value) {
      closeSearch();
      return;
    }
    if (isFullscreen.value) {
      isFullscreen.value = false;
      return;
    }
  }

  // Enter/Shift+Enter to navigate matches (only when search input is focused)
  if (event.key === 'Enter' && searchOpen.value) {
    event.preventDefault();
    if (event.shiftKey) {
      goToPreviousMatch();
    } else {
      goToNextMatch();
    }
  }
}

// Search functions
function openSearch() {
  searchOpen.value = true;
  nextTick(() => {
    searchInputRef.value?.focus();
    searchInputRef.value?.select();
  });
}

function closeSearch() {
  searchOpen.value = false;
  searchTerm.value = '';
  currentMatchIndex.value = 0;
}

function goToNextMatch() {
  if (searchMatches.value.length === 0) return;
  currentMatchIndex.value = (currentMatchIndex.value + 1) % searchMatches.value.length;
  scrollToCurrentMatch();
}

function goToPreviousMatch() {
  if (searchMatches.value.length === 0) return;
  currentMatchIndex.value = (currentMatchIndex.value - 1 + searchMatches.value.length) % searchMatches.value.length;
  scrollToCurrentMatch();
}

function scrollToCurrentMatch() {
  const match = currentMatch.value;
  if (!match) return;

  if (match.type === 'log') {
    // Scroll to log line
    const lineElement = document.getElementById(`L${match.lineNumber}`);
    if (lineElement) {
      lineElement.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  } else if (match.type === 'stderr') {
    // Ensure stderr is expanded and scroll into view
    stderrExpanded.value = true;
    nextTick(() => {
      const stderrSection = document.querySelector('[data-stderr-section]');
      if (stderrSection) {
        stderrSection.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    });
  }
}

onMounted(async () => {
  await loadLogs();
  window.addEventListener('keydown', handleKeydown);
});

onBeforeUnmount(() => {
  stream.value?.close();
  window.removeEventListener('keydown', handleKeydown);
});

watch(stepSlug, async () => {
  await loadLogs();
});

watch(step, async (newStep, oldStep) => {
  if (oldStep?.name === newStep?.name) {
    if (oldStep?.finished !== newStep?.finished && autoScroll.value) {
      scrollToBottom();
    }

    if (oldStep?.state !== newStep?.state) {
      await loadLogs();
    }
  }
});
</script>

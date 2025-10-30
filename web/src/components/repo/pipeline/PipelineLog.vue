<template>
  <div v-if="pipeline" class="flex flex-col md:pt-0">
    <div
      class="code-box-log flex grow flex-col overflow-hidden p-0! shadow-md md:mt-0 md:rounded-lg!"
      @mouseover="showActions = true"
      @mouseleave="showActions = false"
    >
      <!-- Desktop header - hidden on mobile since we have a mobile header in the parent -->
      <div class="hidden md:flex flex-row items-center bg-wp-code-100 px-4 py-1">
        <span class="text-base font-bold text-wp-code-text-alt-100">
          <span class="md:display-unset hidden">{{ $t('repo.pipeline.log_title') }}</span>
          <span class="md:hidden">{{ step?.name }}</span>
        </span>
        <div class="ml-auto flex flex-row items-center gap-x-2">
          <span class="ml-2 px-2">
            <a
              v-if="repoPermissions?.admin"
              href="/admin/agents/"
              class="flex items-center gap-1 text-wp-link-100 hover:text-wp-link-200"
              :title="$t('repo.pipeline.metadata.agent_info')"
            >
              <Icon name="worker" class="w-4 h-4" />
              {{ agentDisplay }}
            </a>
            <a
              v-else
              href="/user/agents/"
              class="flex items-center gap-1 text-wp-link-100 hover:text-wp-link-200"
              :title="$t('repo.pipeline.metadata.agent_info')"
            >
              <Icon name="worker" class="w-4 h-4" />
              {{ agentDisplay }}
            </a>
          </span>
          <IconButton
            v-if="step?.finished !== undefined && hasLogs"
            :is-loading="downloadInProgress"
            :title="$t('repo.pipeline.actions.log_download')"
            class="text-wp-code-text-alt-100 hover:bg-white/10!"
            icon="download"
            @click="download"
          />
          <IconButton
            v-if="step?.finished !== undefined && hasLogs && hasPushPermission"
            :title="$t('repo.pipeline.actions.log_delete')"
            class="text-wp-code-text-alt-100 hover:bg-white/10"
            icon="trash"
            @click="deleteLogs"
          />
          <IconButton
            v-if="step?.finished === undefined"
            :title="
              autoScroll ? $t('repo.pipeline.actions.log_auto_scroll_off') : $t('repo.pipeline.actions.log_auto_scroll')
            "
            class="text-wp-code-text-alt-100 hover:bg-white/10!"
            :icon="autoScroll ? 'auto-scroll' : 'auto-scroll-off'"
            @click="autoScroll = !autoScroll"
          />
        </div>
      </div>

      <!-- Mobile action buttons - foldable floating buttons in top-right corner -->
      <div class="md:hidden absolute top-2 right-2 z-10">
        <!-- Toggle button to show/hide action buttons -->
        <button
          class="rounded-full border border-wp-custom-highlight-100 bg-white dark:bg-wp-code-100 p-2 shadow-lg transition-all duration-200 flex items-center justify-center mb-2"
          :class="{ 'bg-gray-100 dark:bg-wp-code-200': mobileButtonsVisible }"
          title="Toggle actions"
          @click="mobileButtonsVisible = !mobileButtonsVisible"
        >
          <Icon name="settings" class="w-4 h-4 text-gray-700 dark:text-wp-code-text-alt-100" />
        </button>

        <!-- Floating action buttons -->
        <div v-show="mobileButtonsVisible" class="flex flex-col gap-2">
          <div
            class="cursor-pointer rounded-full border border-wp-custom-highlight-100 bg-white dark:bg-wp-code-100 p-2 shadow-lg transition-opacity duration-300 flex items-center justify-center"
            :title="$t('repo.pipeline.actions.log_scroll_to_bottom')"
            @click="scrollToBottom"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              width="12"
              height="12"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              class="text-gray-700 dark:text-wp-code-text-alt-100"
            >
              <polyline points="6 9 12 15 18 9"></polyline>
            </svg>
          </div>
          <div
            class="cursor-pointer rounded-full border border-wp-custom-highlight-100 bg-white dark:bg-wp-code-100 p-2 shadow-lg transition-opacity duration-300 flex items-center justify-center"
            :title="$t('repo.pipeline.actions.log_scroll_to_top')"
            @click="scrollToTop"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              width="12"
              height="12"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              class="text-gray-700 dark:text-wp-code-text-alt-100"
            >
              <polyline points="18 15 12 9 6 15"></polyline>
            </svg>
          </div>
          <IconButton
            v-if="step?.finished !== undefined && hasLogs"
            :is-loading="downloadInProgress"
            :title="$t('repo.pipeline.actions.log_download')"
            class="text-gray-700 dark:text-wp-code-text-alt-100 hover:bg-gray-100 dark:hover:bg-white/10! bg-white dark:bg-wp-code-100 border border-wp-custom-highlight-100 shadow-lg"
            icon="download"
            @click="download"
          />
          <IconButton
            v-if="step?.finished !== undefined && hasLogs && hasPushPermission"
            :title="$t('repo.pipeline.actions.log_delete')"
            class="text-gray-700 dark:text-wp-code-text-alt-100 hover:bg-gray-100 dark:hover:bg-white/10 bg-white dark:bg-wp-code-100 border border-wp-custom-highlight-100 shadow-lg"
            icon="trash"
            @click="deleteLogs"
          />
          <IconButton
            v-if="step?.finished === undefined"
            :title="
              autoScroll ? $t('repo.pipeline.actions.log_auto_scroll_off') : $t('repo.pipeline.actions.log_auto_scroll')
            "
            class="text-gray-700 dark:text-wp-code-text-alt-100 hover:bg-gray-100 dark:hover:bg-white/10! bg-white dark:bg-wp-code-100 border border-wp-custom-highlight-100 shadow-lg"
            :icon="autoScroll ? 'auto-scroll' : 'auto-scroll-off'"
            @click="autoScroll = !autoScroll"
          />
        </div>
      </div>

      <!-- Log area with floating scroll buttons wrapper -->
      <div
        v-show="hasLogs && loadedLogs && (log?.length || 0) > 0"
        class="relative w-full max-w-full grow h-[calc(100vh-200px)]"
      >
        <!-- Floating scroll buttons -->
        <div v-if="isScrollable" class="hidden md:flex flex-col gap-2 absolute bottom-4 right-4 z-20">
          <button
            v-if="!atTop"
            class="rounded-full border border-wp-custom-highlight-100 bg-wp-code-100 p-1 shadow-lg flex items-center justify-center mb-1 hover:bg-wp-code-200 transition"
            :title="$t('repo.pipeline.actions.log_scroll_to_top')"
            @click="scrollToTop"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              class="text-wp-code-text-alt-100"
            >
              <polyline points="18 15 12 9 6 15"></polyline>
            </svg>
          </button>
          <button
            v-if="!atBottom"
            class="rounded-full border border-wp-custom-highlight-100 bg-wp-code-100 p-1 shadow-lg flex items-center justify-center hover:bg-wp-code-200 transition"
            :title="$t('repo.pipeline.actions.log_scroll_to_bottom')"
            @click="scrollToBottom"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              class="text-wp-code-text-alt-100"
            >
              <polyline points="6 9 12 15 18 9"></polyline>
            </svg>
          </button>
        </div>
        <!-- Log grid -->
        <div
          ref="consoleElement"
          class="grid w-full max-w-full h-full auto-rows-min grid-cols-[min-content_minmax(0,1fr)_min-content] overflow-y-auto overflow-x-hidden p-4 text-xs"
        >
          <div v-for="line in log" :key="line.index" class="contents font-mono">
            <a
              :id="`L${line.number}`"
              :href="`#L${line.number}`"
              class="select-none whitespace-nowrap pl-2 pr-6 text-right text-wp-code-text-line-numbers-100"
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
              class="whitespace-pre-wrap break-words align-top"
              :class="{
                'bg-red-600/40 dark:bg-red-800/50': line.type === 'error',
                'bg-yellow-600/40 dark:bg-yellow-800/50': line.type === 'warning',
                'bg-blue-600/30': isSelected(line),
              }"
              v-html="line.text"
            />
            <!-- eslint-enable vue/no-v-html -->
            <span
              class="select-none whitespace-nowrap pr-1 text-right text-wp-code-text-line-numbers-100"
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
      </div>

      <div class="m-auto text-xl text-wp-text-alt-100">
        <span v-if="step?.state === 'skipped'">{{ $t('repo.pipeline.actions.canceled') }}</span>
        <span v-else-if="!step?.started">{{ $t('repo.pipeline.step_not_started') }}</span>
        <div v-else-if="!loadedLogs">{{ $t('repo.pipeline.loading') }}</div>

        <div v-else-if="logsDeleted">{{ $t('repo.pipeline.logs_deleted') }}</div>
        <div v-else-if="log?.length === 0">{{ $t('repo.pipeline.no_logs') }}</div>
      </div>

      <div
        v-if="step?.finished !== undefined"
        class="text-md flex w-full items-center bg-wp-code-100 px-4 py-1 font-bold text-wp-code-text-alt-100 md:min-h-[27px]"
      >
        <PipelineStatusIcon :status="step.state" class="h-4! w-4!" />
        <span v-if="step?.error" class="ml-2 border border-wp-error-100 rounded-full px-2">{{ step.error }}</span>
        <span v-if="step.exit_code === 1" class="ml-2 border border-wp-error-100 rounded-full px-2">{{
          $t('repo.pipeline.exit_code', { exitCode: step.exit_code })
        }}</span>
        <span v-else class="ml-2 border border-wp-control-ok-300 rounded-full px-2">{{
          $t('repo.pipeline.exit_code', { exitCode: step.exit_code })
        }}</span>
      </div>
    </div>
  </div>
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

import IconButton from '~/components/atomic/IconButton.vue';
import Icon from '~/components/atomic/Icon.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
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

const props = defineProps<{
  pipeline: Pipeline;
  stepId: number;
}>();

defineEmits<{
  (event: 'update:step-id', stepId: number | null): true;
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
const workflow = computed(() => {
  return findWorkflow(pipeline.value.workflows || [], stepId.value);
});
const stream = ref<EventSource>();
const log = ref<LogLine[]>();
const consoleElement = ref<Element>();
const logsDeleted = computed(() => pipeline.value.logs_deleted);

const loadedLogs = computed(() => !!log.value);
const hasLogs = computed(
  () =>
    // we do not have logs for skipped steps
    repo?.value && pipeline.value && step.value && step.value.state !== 'skipped',
);
const autoScroll = useStorage('crow:log-auto-scroll', false);
const showActions = ref(false);
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
const agentDetails = ref<{ name?: string } | null>(null);

const agentDisplay = computed(() => {
  const agentId = workflow?.value?.agent_id;
  if (!agentId) return '';

  const agentName = agentDetails.value?.name;
  if (agentName) {
    return agentName;
  }
  return agentId.toString();
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

function writeLog(line: Partial<LogLine>) {
  logBuffer.value.push({
    index: line.index ?? 0,
    number: (line.index ?? 0) + 1,
    text: processText(line.text ?? ''),
    time: line.time ?? 0,
    type: null, // TODO: implement way to detect errors and warnings
  });
}

function scrollToBottom() {
  nextTick(() => {
    if (!consoleElement.value) {
      return;
    }

    const element = consoleElement.value as HTMLElement;
    if (element.scrollHeight <= element.clientHeight) {
      return;
    }

    element.scrollTop = element.scrollHeight;
  });
}

function scrollToTop() {
  if (!consoleElement.value) {
    return;
  }

  const element = consoleElement.value as HTMLElement;
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
    logs?.forEach((line) => writeLog({ index: line.line, text: line.data, time: line.time }));
    flushLogs(false);
  } else {
    loadedStepSlug.value = stepSlug.value;
    stream.value = apiClient.streamLogs(repo.value.id, pipeline.value.number, step.value.id, (line) => {
      writeLog({ index: line.line, text: line.data, time: line.time });
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

function findWorkflow(workflows: PipelineWorkflow[], pid: number): PipelineWorkflow | undefined {
  // Handle empty workflows case
  if (!workflows || !workflows.length) {
    return undefined;
  }

  // First try to find the workflow with a direct step match
  for (const workflow of workflows) {
    if (!workflow.children || !workflow.children.length) continue;

    // Check if any child step matches the pid
    if (workflow.children.some((step) => step.pid === pid)) {
      return workflow;
    }
  }

  // If direct lookup fails, use the original reduce logic as fallback
  return workflows.reduce(
    (prev, workflow) => {
      if (!workflow.children) return prev;

      const foundWorkflow = workflow.children.reduce(
        (prevChild, step) => {
          if (step.pid === pid) {
            return workflow;
          }
          return prevChild;
        },
        undefined as PipelineWorkflow | undefined,
      );

      if (foundWorkflow) {
        return foundWorkflow;
      }
      return prev;
    },
    undefined as PipelineWorkflow | undefined,
  );
}

function updateScrollState() {
  const el = consoleElement.value as HTMLElement | undefined;
  if (!el) return;
  isScrollable.value = el.scrollHeight > el.clientHeight;
  atTop.value = el.scrollTop === 0;
  atBottom.value = el.scrollTop + el.clientHeight >= el.scrollHeight - 5; // more forgiving for rounding
}

async function loadAgentDetails() {
  const agentId = workflow?.value?.agent_id;
  if (!agentId) {
    agentDetails.value = null;
    return;
  }

  try {
    agentDetails.value = await apiClient.getAgent(agentId);
  } catch (error) {
    console.warn(`Failed to load agent ${agentId}:`, error);
    agentDetails.value = null;
  }
}

onMounted(() => {
  if (consoleElement.value) {
    (consoleElement.value as HTMLElement).addEventListener('scroll', updateScrollState);
    nextTick(updateScrollState);
  }
});

watch(log, () => {
  nextTick(() => {
    updateScrollState();
    setTimeout(updateScrollState, 50); // ensure update after rendering
  });
});

onBeforeUnmount(() => {
  if (consoleElement.value) {
    (consoleElement.value as HTMLElement).removeEventListener('scroll', updateScrollState);
  }
});

onMounted(async () => {
  await loadLogs();
  await loadAgentDetails();
});

onBeforeUnmount(() => {
  stream.value?.close();
});

watch(stepSlug, async () => {
  await loadLogs();
});

watch(workflow, async () => {
  await loadAgentDetails();
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

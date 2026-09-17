<!--
  Index of the warnings and errors the log classifier found in a step, so a
  problem buried thousands of lines down is one click away rather than
  something you have to scroll for. Renders nothing on a clean run.
-->
<template>
  <div v-if="allIssues.length > 0" data-log-issues class="rounded-lg border bg-muted/40">
    <div class="flex items-center gap-2 px-2 py-1.5">
      <TooltipProvider v-if="errorCount > 0">
        <Tooltip>
          <TooltipTrigger as-child>
            <button
              type="button"
              class="flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs transition-colors"
              :class="
                isSeverityMuted('error')
                  ? 'text-muted-foreground line-through hover:bg-accent'
                  : 'bg-red-600/40 text-foreground hover:bg-red-600/60 dark:bg-red-800/50 dark:hover:bg-red-800/70'
              "
              :aria-pressed="!isSeverityMuted('error')"
              @click="toggleSeverity('error')"
            >
              <Icon name="ph:x-circle" class="h-3.5 w-3.5" />
              {{ $t('repo.pipeline.issues.errors', { count: errorCount }) }}
            </button>
          </TooltipTrigger>
          <TooltipContent>
            {{
              isSeverityMuted('error') ? $t('repo.pipeline.issues.show_errors') : $t('repo.pipeline.issues.hide_errors')
            }}
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
      <TooltipProvider v-if="warningCount > 0">
        <Tooltip>
          <TooltipTrigger as-child>
            <button
              type="button"
              class="flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs transition-colors"
              :class="
                isSeverityMuted('warning')
                  ? 'text-muted-foreground line-through hover:bg-accent'
                  : 'bg-yellow-600/40 text-foreground hover:bg-yellow-600/60 dark:bg-yellow-800/50 dark:hover:bg-yellow-800/70'
              "
              :aria-pressed="!isSeverityMuted('warning')"
              @click="toggleSeverity('warning')"
            >
              <Icon name="ph:warning" class="h-3.5 w-3.5" />
              {{ $t('repo.pipeline.issues.warnings', { count: warningCount }) }}
            </button>
          </TooltipTrigger>
          <TooltipContent>
            {{
              isSeverityMuted('warning')
                ? $t('repo.pipeline.issues.show_warnings')
                : $t('repo.pipeline.issues.hide_warnings')
            }}
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>

      <div class="ml-auto flex items-center gap-1">
        <span v-if="issues.length > 0" class="whitespace-nowrap text-xs text-muted-foreground">
          {{ $t('repo.pipeline.issues.position', { current: currentIssueIndex + 1, total: issues.length }) }}
        </span>
        <TooltipProvider v-if="issues.length > 0">
          <Tooltip>
            <TooltipTrigger as-child>
              <Button variant="ghost" size="icon-sm" data-log-issue-prev @click="goToIssue(currentIssueIndex - 1)">
                <Icon name="chevron-up" class="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{{ $t('repo.pipeline.issues.previous') }}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
        <TooltipProvider v-if="issues.length > 0">
          <Tooltip>
            <TooltipTrigger as-child>
              <Button variant="ghost" size="icon-sm" data-log-issue-next @click="goToIssue(currentIssueIndex + 1)">
                <Icon name="chevron-down" class="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{{ $t('repo.pipeline.issues.next') }}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
        <TooltipProvider v-if="issues.length > 0">
          <Tooltip>
            <TooltipTrigger as-child>
              <Button variant="ghost" size="icon-sm" @click="copyIssues">
                <Icon :name="issuesCopied ? 'ph:check' : 'ph:copy'" class="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{{ issuesCopied ? $t('copied') : $t('repo.pipeline.issues.copy') }}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
        <Button
          variant="ghost"
          size="icon-sm"
          :aria-expanded="issuesExpanded"
          :aria-label="issuesExpanded ? $t('repo.pipeline.issues.collapse') : $t('repo.pipeline.issues.expand')"
          @click="issuesExpanded = !issuesExpanded"
        >
          <Icon name="chevron-down" class="h-4 w-4 transition-transform" :class="{ 'rotate-180': issuesExpanded }" />
        </Button>
      </div>
    </div>

    <div v-if="issuesExpanded" class="max-h-56 overflow-y-auto border-t">
      <button
        v-for="issue in listedIssues"
        :key="issue.index"
        type="button"
        data-log-issue-entry
        class="grid w-full grid-cols-[auto_3rem_minmax(0,1fr)] items-baseline gap-2 px-2 py-1 text-left text-xs transition-colors hover:bg-accent md:grid-cols-[auto_4rem_minmax(0,1fr)]"
        :class="{ 'bg-blue-600/20': isCurrentIssue(issue) }"
        @click="selectIssue(issue)"
      >
        <span
          class="h-2 w-2 shrink-0 self-center rounded-full"
          :class="issue.severity === 'error' ? 'bg-red-600 dark:bg-red-500' : 'bg-yellow-500'"
        />
        <span data-log-issue-number class="font-mono text-right text-muted-foreground">{{ issue.number }}</span>
        <span class="flex min-w-0 items-baseline gap-2">
          <span class="min-w-0 flex-1 truncate font-mono text-foreground">{{ issue.text }}</span>
          <span v-if="issue.extraLines > 0" class="shrink-0 text-muted-foreground">
            {{ $t('repo.pipeline.issues.more_lines', { count: issue.extraLines }) }}
          </span>
          <span v-if="issue.section" class="hidden max-w-[16rem] shrink-0 truncate text-muted-foreground md:block">
            {{ issue.section }}
          </span>
        </span>
      </button>
      <p v-if="hasHiddenIssues" class="px-2 py-1.5 text-xs text-muted-foreground">
        {{ $t('repo.pipeline.issues.truncated', { shown: listedIssues.length, total: issues.length }) }}
      </p>
    </div>
  </div>
</template>

<script lang="ts" setup>
import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import useNotifications from '~/compositions/useNotifications';
import type { IssueLogLine, LogIssue, LogIssueSeverity } from '~/lib/log-issues';
import { buildIssueIndex } from '~/lib/log-issues';
import { useStorage } from '@vueuse/core';
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

const props = defineProps<{ lines: readonly IssueLogLine[] }>();
const emit = defineEmits<{ (event: 'jump', lineNumber: number): true }>();

const notifications = useNotifications();
const i18n = useI18n();

const ISSUE_LIST_LIMIT = 200;
const issuesExpanded = ref(false);
const issuesCopied = ref(false);
let issuesCopiedTimer: ReturnType<typeof setTimeout> | undefined;
// Kept as an array rather than a Set so it survives a round trip through
// localStorage, and remembered per browser like the other log-view settings.
const mutedSeverities = useStorage<LogIssueSeverity[]>('crow:log-issue-severities', []);
const currentIssueIndex = ref(0);

const allIssues = computed(() => buildIssueIndex(props.lines));
const warningCount = computed(() => allIssues.value.filter((issue) => issue.severity === 'warning').length);
const errorCount = computed(() => allIssues.value.length - warningCount.value);
/** The issues the severity chips currently leave visible, in log order. */
const issues = computed(() => allIssues.value.filter((issue) => !isSeverityMuted(issue.severity)));
const listedIssues = computed(() => issues.value.slice(0, ISSUE_LIST_LIMIT));
const hasHiddenIssues = computed(() => issues.value.length > listedIssues.value.length);

function isSeverityMuted(severity: LogIssueSeverity): boolean {
  return mutedSeverities.value.includes(severity);
}

function toggleSeverity(severity: LogIssueSeverity) {
  const next = isSeverityMuted(severity)
    ? mutedSeverities.value.filter((muted) => muted !== severity)
    : [...mutedSeverities.value, severity];
  mutedSeverities.value = next;
  currentIssueIndex.value = 0;
}

function goToIssue(index: number) {
  if (issues.value.length === 0) return;
  const count = issues.value.length;
  currentIssueIndex.value = ((index % count) + count) % count;
  emit('jump', issues.value[currentIssueIndex.value].number);
}

function selectIssue(issue: LogIssue) {
  const index = issues.value.indexOf(issue);
  if (index !== -1) {
    goToIssue(index);
  }
}

function isCurrentIssue(issue: LogIssue): boolean {
  return issues.value[currentIssueIndex.value] === issue;
}

async function copyIssues() {
  const text = issues.value.map((issue) => `${issue.number}\t${issue.severity}\t${issue.text}`).join('\n');
  try {
    await navigator.clipboard.writeText(text);
    issuesCopied.value = true;
    clearTimeout(issuesCopiedTimer);
    issuesCopiedTimer = setTimeout(() => {
      issuesCopied.value = false;
    }, 2000);
  } catch {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      text: i18n.t('repo.pipeline.log_copy_error'),
      type: 'error',
      duration: 3000,
    });
  }
}

// A step that streams new lines can shrink or grow the index under the cursor.
watch(issues, (list) => {
  if (currentIssueIndex.value >= list.length) {
    currentIssueIndex.value = 0;
  }
});

onBeforeUnmount(() => {
  if (issuesCopiedTimer) {
    clearTimeout(issuesCopiedTimer);
  }
});
</script>

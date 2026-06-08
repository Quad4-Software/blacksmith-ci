<template>
  <Panel>
    <div class="flex flex-col gap-4">
      <!-- Heading -->
      <div class="flex items-center gap-3">
        <Icon :name="headerIcon" size="1.5rem" class="size-8" :class="headerIconClass" />
        <div class="flex flex-col">
          <span class="text-lg font-semibold">
            <template v-if="approvalState === 'approved'">{{ $t('workflow.approval.title_approved') }}</template>
            <template v-else-if="approvalState === 'declined'">{{ $t('workflow.approval.title_declined') }}</template>
            <template v-else-if="approvalState === 'expired'">{{ $t('workflow.approval.title_expired') }}</template>
            <template v-else>{{ $t('workflow.approval.title_pending') }}</template>
          </span>
          <span class="text-sm text-muted-foreground">{{ workflow.name }}</span>
        </div>
      </div>

      <!-- Deadline -->
      <div v-if="deadlineLabel" class="flex items-center gap-2 text-sm text-muted-foreground">
        <Icon name="lucide:hourglass" size="1rem" class="size-4" />
        <span>{{ deadlineLabel }}</span>
      </div>

      <!-- Approvers -->
      <div class="flex flex-col gap-1 text-sm">
        <span class="font-medium">{{ $t('workflow.approval.approvers') }}</span>
        <span class="text-muted-foreground">{{ approversLabel }}</span>
      </div>

      <!-- Reason input (pending only) -->
      <Textarea
        v-if="isPending"
        v-model="reason"
        :placeholder="$t('workflow.approval.reason_placeholder')"
        class="min-h-20"
        :disabled="isBusy"
      />

      <!-- Action buttons (pending only) -->
      <div v-if="isPending" class="flex flex-wrap items-center gap-2">
        <Button
          variant="outline"
          class="border-green-500 hover:border-green-600 hover:bg-green-50 dark:hover:bg-green-950/20"
          :disabled="!canAct || isBusy"
          @click="onApprove"
        >
          <Icon v-if="isApproving" name="spinner" class="size-5 animate-spin" />
          <template v-else>{{ $t('workflow.approval.approve') }}</template>
        </Button>
        <Button variant="destructive" :disabled="!canAct || isBusy" @click="onDecline">
          <Icon v-if="isDeclining" name="spinner" class="size-5 animate-spin" />
          <template v-else>{{ $t('workflow.approval.decline') }}</template>
        </Button>
        <span v-if="!canApprove && !approvalLoading" class="text-sm text-muted-foreground">
          {{ $t('workflow.approval.cant_approve') }}
        </span>
      </div>

      <!-- Audit trail -->
      <div class="flex flex-col gap-2 border-t border-border pt-4">
        <span class="text-sm font-medium">{{ $t('workflow.approval.audit_trail') }}</span>
        <ul v-if="events.length > 0" class="flex flex-col gap-1 text-xs">
          <li v-for="evt in events" :key="evt.id" class="flex items-center gap-2 text-muted-foreground">
            <Icon :name="iconForAction(evt.action)" class="size-3.5 shrink-0" :class="colorForAction(evt.action)" />
            <span>{{ actionLabel(evt.action) }}</span>
            <span v-if="evt.reason" class="italic">{{ evt.reason }}</span>
            <span class="ml-auto tabular-nums">{{ formatEventTime(evt.created_at) }}</span>
          </li>
        </ul>
        <span v-else class="text-xs italic text-muted-foreground">{{ $t('workflow.approval.audit_empty') }}</span>
      </div>
    </div>
  </Panel>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref, toRef, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import Panel from '~/components/layout/Panel.vue';
import { Button } from '~/components/ui/button';
import { Textarea } from '~/components/ui/textarea';
import useApiClient from '~/compositions/useApiClient';
import { useDate } from '~/compositions/useDate';
import useNotifications from '~/compositions/useNotifications';
import type { IconNames } from '~/components/atomic/Icon.types';
import type { ApprovalState, PipelineWorkflow, WorkflowApprovalEvent } from '~/lib/api/types';

const props = defineProps<{
  repoId: number;
  pipelineNumber: number;
  workflow: PipelineWorkflow;
}>();

const emit = defineEmits<{
  (event: 'updated', workflow: PipelineWorkflow): void;
}>();

const apiClient = useApiClient();
const notifications = useNotifications();
const { t } = useI18n();
const { prettyDuration, toLocaleString } = useDate();

const workflow = toRef(props, 'workflow');

const reason = ref('');
const isApproving = ref(false);
const isDeclining = ref(false);
const canApprove = ref(false);
const approvalLoading = ref(true);
const events = ref<WorkflowApprovalEvent[]>([]);

const isBusy = computed(() => isApproving.value || isDeclining.value);
const canAct = computed(() => canApprove.value);

const approvalState = computed<ApprovalState>(() => workflow.value.approval_state ?? '');
const isPending = computed(() => approvalState.value === 'pending');

const headerIcon = computed<IconNames>(() => {
  switch (approvalState.value) {
    case 'approved':
      return 'status-success';
    case 'declined':
      return 'status-declined';
    case 'expired':
      return 'status-blocked';
    default:
      return 'lucide:lock';
  }
});

const headerIconClass = computed(() => {
  switch (approvalState.value) {
    case 'approved':
      return 'text-green-500';
    case 'declined':
      return 'text-amber-500';
    case 'expired':
      return 'text-amber-500';
    default:
      return 'text-yellow-500';
  }
});

const approversLabel = computed(() => {
  const users = (workflow.value.approval_users ?? []).map((u) => u);
  const teams = (workflow.value.approval_teams ?? []).map((g) => `@${g}`);
  const parts = [...users, ...teams];
  return parts.length > 0 ? parts.join(', ') : t('workflow.approval.no_approvers');
});

const deadlineLabel = computed(() => {
  const deadlineSec = workflow.value.approval_deadline;
  if (!deadlineSec || deadlineSec <= 0) {
    return '';
  }
  const deadlineMs = deadlineSec * 1000;
  // Past-tense rendering for terminal states: show the deadline as a
  // historical timestamp instead of a live countdown.
  if (!isPending.value) {
    return t('workflow.approval.deadline_was', { at: toLocaleString(new Date(deadlineMs)) });
  }
  const remaining = deadlineMs - Date.now();
  if (remaining <= 0) {
    return t('workflow.approval.expired');
  }
  return t('workflow.approval.expires_in', { delta: prettyDuration(remaining) });
});

function iconForAction(action: WorkflowApprovalEvent['action']): IconNames {
  if (action === 'approved') return 'status-success';
  if (action === 'declined') return 'status-declined';
  return 'status-blocked';
}

function colorForAction(action: WorkflowApprovalEvent['action']): string {
  if (action === 'approved') return 'text-green-500';
  if (action === 'declined') return 'text-destructive';
  return 'text-muted-foreground';
}

function actionLabel(action: WorkflowApprovalEvent['action']): string {
  switch (action) {
    case 'approved':
      return t('workflow.approval.audit_action.approved');
    case 'declined':
      return t('workflow.approval.audit_action.declined');
    case 'expired':
      return t('workflow.approval.audit_action.expired');
    default:
      return action;
  }
}

function formatEventTime(unixSeconds: number): string {
  return toLocaleString(new Date(unixSeconds * 1000));
}

async function loadApprovalState() {
  try {
    const state = await apiClient.getWorkflowApproval(props.repoId, props.pipelineNumber, workflow.value.id);
    events.value = state.events ?? [];
  } catch (error) {
    console.error(error);
  }
}

async function loadCanApprove() {
  // Only pending gates accept approve/decline. Skip the API call for terminal
  // states — buttons aren't rendered anyway.
  if (!isPending.value) {
    canApprove.value = false;
    approvalLoading.value = false;
    return;
  }
  approvalLoading.value = true;
  try {
    const { workflows } = await apiClient.getUserPendingApprovals();
    canApprove.value = (workflows ?? []).some((wf) => wf.id === workflow.value.id);
  } catch (error) {
    console.error(error);
    canApprove.value = false;
  } finally {
    approvalLoading.value = false;
  }
}

async function onApprove() {
  if (!canAct.value || isBusy.value) return;
  isApproving.value = true;
  try {
    const updated = await apiClient.workflowApprove(
      props.repoId,
      props.pipelineNumber,
      workflow.value.id,
      reason.value,
    );
    reason.value = '';
    emit('updated', updated);
    await loadApprovalState();
    notifications.notify({
      title: t('notify_title_success'),
      text: t('workflow.approval.notify_approve_success'),
      type: 'success',
      duration: 6000,
    });
  } catch (error) {
    notifications.notify({
      title: t('notify_title_error'),
      text: error instanceof Error ? error.message : t('unknown_error'),
      type: 'error',
      duration: 9000,
    });
  } finally {
    isApproving.value = false;
  }
}

async function onDecline() {
  if (!canAct.value || isBusy.value) return;
  isDeclining.value = true;
  try {
    const updated = await apiClient.workflowDecline(
      props.repoId,
      props.pipelineNumber,
      workflow.value.id,
      reason.value,
    );
    reason.value = '';
    emit('updated', updated);
    await loadApprovalState();
    notifications.notify({
      title: t('notify_title_success'),
      text: t('workflow.approval.notify_decline_success'),
      type: 'success',
      duration: 6000,
    });
  } catch (error) {
    notifications.notify({
      title: t('notify_title_error'),
      text: error instanceof Error ? error.message : t('unknown_error'),
      type: 'error',
      duration: 9000,
    });
  } finally {
    isDeclining.value = false;
  }
}

onMounted(async () => {
  await Promise.all([loadApprovalState(), loadCanApprove()]);
});

// Re-fetch the audit trail when the workflow changes (e.g., pipeline reloaded).
watch(
  () => workflow.value.id,
  async () => {
    await loadApprovalState();
  },
);

// Re-check approve permission when state changes (e.g., approval just landed
// via SSE while the panel was open).
watch(
  () => workflow.value.approval_state,
  async () => {
    await loadCanApprove();
  },
);
</script>

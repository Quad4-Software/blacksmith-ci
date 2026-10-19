<template>
  <div class="flex items-center gap-2">
    <Button
      v-if="agent.restart_requested || agent.restart_queued"
      variant="outline"
      size="sm"
      :disabled="isLoading"
      data-testid="agent-restart-cancel"
      @click="cancelRestart"
    >
      <Icon v-if="isLoading" name="loading" class="h-4 w-4 animate-spin" />
      {{ $t('admin.settings.agents.restart.cancel') }}
    </Button>
    <span v-else :title="acceptsRestart ? undefined : $t('admin.settings.agents.restart.not_accepted')">
      <Button
        variant="outline"
        size="sm"
        :disabled="!acceptsRestart || isLoading"
        data-testid="agent-restart-request"
        @click="confirmOpen = true"
      >
        <Icon v-if="isLoading" name="loading" class="h-4 w-4 animate-spin" />
        <Icon v-else name="ph:arrow-clockwise" class="h-4 w-4" />
        {{ $t('admin.settings.agents.restart.request') }}
      </Button>
    </span>

    <AlertDialog :open="confirmOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('admin.settings.agents.restart.confirm_title') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('admin.settings.agents.restart.confirm_text') }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel @click="confirmOpen = false">{{ $t('cancel') }}</AlertDialogCancel>
          <AlertDialogAction data-testid="agent-restart-confirm" @click="requestRestart">
            {{ $t('admin.settings.agents.restart.request') }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
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
import { Button } from '~/components/ui/button';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useNotifications from '~/compositions/useNotifications';
import type { Agent } from '~/lib/api/types';

const props = defineProps<{
  agent: Agent;
}>();

const emit = defineEmits<{
  updated: [agent: Agent];
}>();

const apiClient = useApiClient();
const notifications = useNotifications();
const { t } = useI18n();

const confirmOpen = ref(false);
const acceptsRestart = computed(() => props.agent.remote_update === 'restart');

function notifyError(err: unknown) {
  notifications.notify({
    title: t('notify_title_error'),
    text: err instanceof Error ? err.message : String(err),
    type: 'error',
    duration: 6000,
  });
}

const { doSubmit: requestRestart, isLoading: requesting } = useAsyncAction(async () => {
  confirmOpen.value = false;
  emit('updated', await apiClient.requestAgentRestart(props.agent.id));
  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.agents.restart.requested'),
    type: 'success',
    duration: 6000,
  });
}, notifyError);

const { doSubmit: cancelRestart, isLoading: cancelling } = useAsyncAction(async () => {
  emit('updated', await apiClient.cancelAgentRestart(props.agent.id));
}, notifyError);

const isLoading = computed(() => requesting.value || cancelling.value);
</script>

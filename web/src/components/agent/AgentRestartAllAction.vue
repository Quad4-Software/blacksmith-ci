<template>
  <Button variant="outline" :disabled="isLoading" data-testid="agent-restart-all" @click="confirmOpen = true">
    <Icon v-if="isLoading" name="loading" class="size-4 mr-1 animate-spin" />
    <Icon v-else name="ph:arrow-clockwise" class="size-4 mr-1" />
    {{ $t('admin.settings.agents.restart.all') }}
  </Button>

  <AlertDialog :open="confirmOpen">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('admin.settings.agents.restart.all_confirm_title') }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{ $t('admin.settings.agents.restart.all_confirm_text', { version: config.version, count: maxParallel }) }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel @click="confirmOpen = false">{{ $t('cancel') }}</AlertDialogCancel>
        <AlertDialogAction data-testid="agent-restart-all-confirm" @click="queueRestarts">
          {{ $t('admin.settings.agents.restart.all') }}
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
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
import useConfig from '~/compositions/useConfig';
import useNotifications from '~/compositions/useNotifications';

const emit = defineEmits<{
  queued: [count: number];
}>();

const apiClient = useApiClient();
const notifications = useNotifications();
const config = useConfig();
const { t } = useI18n();

const confirmOpen = ref(false);
const maxParallel = computed(() => config.agentAutoRestart?.max_parallel ?? 1);

function notifyError(err: unknown) {
  notifications.notify({
    title: t('notify_title_error'),
    text: err instanceof Error ? err.message : String(err),
    type: 'error',
    duration: 6000,
  });
}

const { doSubmit: queueRestarts, isLoading } = useAsyncAction(async () => {
  confirmOpen.value = false;
  const { queued } = await apiClient.queueAgentRestarts();
  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.agents.restart.all_queued', queued),
    type: 'success',
    duration: 6000,
  });
  emit('queued', queued);
}, notifyError);
</script>

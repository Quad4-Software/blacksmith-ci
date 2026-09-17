<template>
  <div class="flex flex-wrap items-center gap-3">
    <Button :disabled="isRepairingRepo" @click="repairRepo">
      <Icon v-if="isRepairingRepo" name="spinner" class="mr-0 h-4 w-4 animate-spin" />
      <Icon v-else name="ph:arrow-counter-clockwise" class="mr-0 h-4 w-4" />
      {{ $t('repo.settings.actions.repair.repair') }}
    </Button>
    <Button v-if="isActive" :disabled="isDeactivatingRepo" @click="deactivateRepo">
      <Icon v-if="isDeactivatingRepo" name="spinner" class="mr-0 h-4 w-4 animate-spin" />
      <Icon v-else name="turn-off" class="mr-0 h-4 w-4" />
      {{ $t('repo.settings.actions.disable.disable') }}
    </Button>
    <Button v-else :disabled="isActivatingRepo" @click="activateRepo">
      <Icon v-if="isActivatingRepo" name="spinner" class="mr-0 h-4 w-4 animate-spin" />
      <Icon v-else name="turn-off" class="mr-0 h-4 w-4" />
      {{ $t('repo.settings.actions.enable.enable') }}
    </Button>

    <!-- Delete with AlertDialog -->
    <AlertDialog v-model:open="showDeleteDialog">
      <AlertDialogTrigger as-child>
        <Button variant="destructive" :disabled="isDeletingRepo">
          <Icon v-if="isDeletingRepo" name="spinner" class="mr-0 h-4 w-4 animate-spin" />
          <Icon v-else name="trash" class="mr-0 h-4 w-4" />
          {{ $t('repo.settings.actions.delete.delete') }}
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('repo.settings.actions.delete.delete') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('repo.settings.actions.delete.notify_text_confirm') }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
          <AlertDialogAction as-child>
            <Button variant="destructive" @click="confirmDelete">
              {{ $t('repo.settings.actions.delete.delete') }}
            </Button>
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
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
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { requiredInject } from '~/compositions/useInjectProvide';
import { suppress404Errors } from '~/compositions/useNotificationControl';
import useNotifications from '~/compositions/useNotifications';

const apiClient = useApiClient();
const router = useRouter();
const notifications = useNotifications();
const i18n = useI18n();

const repo = requiredInject('repo');
const showDeleteDialog = ref(false);

const { doSubmit: repairRepo, isLoading: isRepairingRepo } = useAsyncAction(async () => {
  await apiClient.repairRepo(repo.value.id);
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('repo.settings.actions.repair.notify_text_success'),
    type: 'success',
    duration: 6000,
  });
});

const { doSubmit: deleteRepoAction, isLoading: isDeletingRepo } = useAsyncAction(async () => {
  // Suppress 404 errors that may occur from background fetches after deletion
  suppress404Errors(3000);
  await apiClient.deleteRepo(repo.value.id);
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('repo.settings.actions.delete.notify_text_success'),
    type: 'success',
    duration: 6000,
  });
  await router.replace({ name: 'repos' });
});

async function confirmDelete() {
  showDeleteDialog.value = false;
  await deleteRepoAction();
}

const { doSubmit: activateRepo, isLoading: isActivatingRepo } = useAsyncAction(async () => {
  await apiClient.activateRepo(repo.value.forge_remote_id);
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('repo.settings.actions.enable.notify_text_success'),
    type: 'success',
    duration: 6000,
  });
});

const { doSubmit: deactivateRepo, isLoading: isDeactivatingRepo } = useAsyncAction(async () => {
  // Suppress 404 errors that may occur from background fetches after deactivation
  suppress404Errors(3000);
  await apiClient.deleteRepo(repo.value.id, false);
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('repo.settings.actions.disable.notify_text_success'),
    type: 'success',
    duration: 6000,
  });
  await router.replace({ name: 'repos' });
});

const isActive = computed(() => repo?.value.active);
</script>

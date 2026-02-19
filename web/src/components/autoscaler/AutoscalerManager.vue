<template>
  <Settings :title="$t('admin.settings.autoscalers.autoscalers')" :description>
    <template #headerActions>
      <Button v-if="selectedAutoscaler" @click="selectedAutoscaler = undefined">
        <Icon name="back" class="size-4 mr-1" />{{ $t('admin.settings.autoscalers.show') }}
      </Button>
      <Button v-else @click="showAddAutoscaler">
        <Icon name="plus" class="size-4 mr-1" />{{ $t('admin.settings.autoscalers.add') }}
      </Button>
    </template>
    <template #headerEnd>
      <slot name="headerEnd" />
    </template>

    <!-- Show token after creation -->
    <Panel
      v-if="createdAutoscalerToken"
      class="mb-6 !bg-yellow-50 dark:!bg-yellow-900/20 !border-yellow-200 dark:!border-yellow-800"
    >
      <div class="flex items-start gap-3">
        <Icon name="alert" class="h-5 w-5 text-yellow-600 dark:text-yellow-400 shrink-0 mt-0.5" />
        <div class="flex-1 min-w-0">
          <h3 class="text-sm font-semibold text-yellow-800 dark:text-yellow-200 mb-2">
            {{ $t('admin.settings.autoscalers.token_warning') }}
          </h3>
          <div class="flex items-center gap-2">
            <code
              class="flex-1 px-3 py-2 bg-white dark:bg-gray-800 border border-yellow-300 dark:border-yellow-700 rounded text-xs font-mono overflow-x-auto"
            >
              {{ createdAutoscalerToken }}
            </code>
            <IconButton icon="content-copy" :title="$t('admin.settings.autoscalers.token_copy')" @click="copyToken" />
          </div>
        </div>
        <IconButton icon="close" class="shrink-0" @click="createdAutoscalerToken = undefined" />
      </div>
    </Panel>

    <AutoscalerList
      v-if="!selectedAutoscaler"
      :loading="loading"
      :autoscalers="autoscalers"
      :is-deleting="isDeleting"
      :autoscaler-filters="autoscalerFilters"
      :current-page="page"
      :has-more="hasMore"
      :is-jumping-to-last="isJumpingToLast"
      @update:current-page="goToPage"
      @jump-to-last="handleJumpToLast"
      @update-filters="(filters) => emit('updateFilters', filters)"
      @toggle-enabled="toggleEnabled"
      @delete="deleteAutoscaler"
      @filters-changed="resetPage"
    />
    <AutoscalerForm
      v-else
      v-model="selectedAutoscaler"
      :is-saving="isSaving"
      @save="saveAutoscaler"
      @cancel="selectedAutoscaler = undefined"
    />
  </Settings>
</template>

<script lang="ts" setup>
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import IconButton from '~/components/atomic/IconButton.vue';
import Panel from '~/components/layout/Panel.vue';
import Settings from '~/components/layout/Settings.vue';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useNotifications from '~/compositions/useNotifications';
import { usePaginationWithPages } from '~/compositions/usePaginate';
import type { Autoscaler, AutoscalerWithToken } from '~/lib/api/types';

import AutoscalerForm from './AutoscalerForm.vue';
import AutoscalerList from './AutoscalerList.vue';

const props = defineProps<{
  description: string;
  loadAutoscalers: (page: number) => Promise<Autoscaler[] | null>;
  createAutoscaler: (autoscaler: Partial<Autoscaler>) => Promise<AutoscalerWithToken>;
  updateAutoscaler: (autoscaler: Autoscaler) => Promise<Autoscaler | void>;
  deleteAutoscaler: (autoscaler: Autoscaler) => Promise<unknown>;
  autoscalerFilters: {
    search: string;
    enabled?: string;
    sort: string;
    order: string;
  };
}>();

const emit = defineEmits<{
  updateFilters: [filters: { search?: string; enabled?: string; sort?: string; order?: string }];
}>();

const notifications = useNotifications();
const { t } = useI18n();

const selectedAutoscaler = ref<Partial<Autoscaler>>();
const createdAutoscalerToken = ref<string>();

const {
  page,
  resetPage,
  goToPage,
  goToLastPage,
  data: autoscalers,
  loading,
  hasMore,
} = usePaginationWithPages(props.loadAutoscalers);

const isJumpingToLast = ref(false);

async function handleJumpToLast() {
  isJumpingToLast.value = true;
  try {
    await goToLastPage();
  } finally {
    isJumpingToLast.value = false;
  }
}

const { doSubmit: saveAutoscaler, isLoading: isSaving } = useAsyncAction(async () => {
  if (!selectedAutoscaler.value) {
    throw new Error("Unexpected: Can't get autoscaler");
  }

  const result = await props.createAutoscaler(selectedAutoscaler.value);
  createdAutoscalerToken.value = result.token;
  selectedAutoscaler.value = undefined;
  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.autoscalers.notify_text_created'),
    type: 'success',
    duration: 6000,
  });
  await resetPage();
});

const { doSubmit: toggleEnabled } = useAsyncAction(async (_autoscaler: Autoscaler) => {
  const updatedAutoscaler = { ..._autoscaler, enabled: !_autoscaler.enabled };
  await props.updateAutoscaler(updatedAutoscaler);
  notifications.notify({
    title: t('notify_title_success'),
    text: updatedAutoscaler.enabled
      ? t('admin.settings.autoscalers.notify_text_enabled')
      : t('admin.settings.autoscalers.notify_text_disabled'),
    type: 'success',
    duration: 3000,
  });
  await resetPage();
});

const { doSubmit: deleteAutoscaler, isLoading: isDeleting } = useAsyncAction(async (_autoscaler: Autoscaler) => {
  // eslint-disable-next-line no-alert
  if (!confirm(t('admin.settings.autoscalers.delete_confirm'))) {
    return;
  }

  await props.deleteAutoscaler(_autoscaler);
  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.autoscalers.notify_text_deleted'),
    type: 'success',
    duration: 6000,
  });
  await resetPage();
});

function showAddAutoscaler() {
  selectedAutoscaler.value = { name: '' };
}

async function copyToken() {
  if (!createdAutoscalerToken.value) return;

  try {
    await navigator.clipboard.writeText(createdAutoscalerToken.value);
    notifications.notify({
      title: t('notify_title_success'),
      text: 'Token copied to clipboard',
      type: 'success',
      duration: 3000,
    });
  } catch {
    notifications.notify({
      title: t('notify_title_error'),
      text: 'Failed to copy token',
      type: 'error',
      duration: 3000,
    });
  }
}
</script>

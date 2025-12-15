<template>
  <div class="mx-auto w-full max-w-none px-4">
    <AutoscalerManager
      :description="$t('admin.settings.autoscalers.desc')"
      :load-autoscalers="loadAutoscalers"
      :create-autoscaler="createAutoscaler"
      :update-autoscaler="updateAutoscaler"
      :delete-autoscaler="deleteAutoscaler"
      :autoscaler-filters="autoscalerFilters"
      @update-filters="(filters) => Object.assign(autoscalerFilters, filters)"
    />
  </div>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import AutoscalerManager from '~/components/autoscaler/AutoscalerManager.vue';
import useApiClient from '~/compositions/useApiClient';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { Autoscaler } from '~/lib/api/types';

const apiClient = useApiClient();

const autoscalerFilters = ref<{
  search: string;
  enabled?: string;
  sort: 'status' | 'id' | 'name' | 'last_contact';
  order: 'asc' | 'desc';
}>({
  search: '',
  enabled: undefined,
  sort: 'status',
  order: 'asc',
});

const loadAutoscalers = (page: number) =>
  apiClient.getAutoscalers({
    page,
    ...autoscalerFilters.value,
    enabled:
      autoscalerFilters.value.enabled === 'true'
        ? true
        : autoscalerFilters.value.enabled === 'false'
          ? false
          : undefined,
  });

const createAutoscaler = (autoscaler: Partial<Autoscaler>) => apiClient.createAutoscaler(autoscaler);
const updateAutoscaler = (autoscaler: Autoscaler) => apiClient.updateAutoscaler(autoscaler);
const deleteAutoscaler = (autoscaler: Autoscaler) => apiClient.deleteAutoscaler(autoscaler);

const { t } = useI18n();
useCrowTitle(computed(() => [t('admin.settings.autoscalers.autoscalers')]));
</script>

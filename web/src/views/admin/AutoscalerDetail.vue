<template>
  <div class="flex flex-col h-full p-4 gap-4">
    <header class="flex items-center gap-3">
      <h1 class="text-xl font-semibold">
        {{ autoscaler?.name ?? $t('admin.settings.autoscalers.detail_loading') }}
      </h1>
      <span v-if="autoscaler" class="text-sm text-muted-foreground">{{
        $t('admin.settings.autoscalers.detail_id', { id: autoscaler.id })
      }}</span>
    </header>

    <Tabs v-model="activeTab" class="flex-1 min-h-0 flex flex-col">
      <TabsList>
        <TabsTrigger value="info">{{ $t('admin.settings.autoscalers.tab_info') }}</TabsTrigger>
        <TabsTrigger value="logs">{{ $t('admin.settings.autoscalers.tab_logs') }}</TabsTrigger>
      </TabsList>

      <TabsContent value="info" class="flex-1 overflow-auto">
        <dl v-if="autoscaler" class="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-2 text-sm">
          <dt class="text-muted-foreground">{{ $t('admin.settings.autoscalers.detail_provider') }}</dt>
          <dd>{{ autoscaler.provider }}</dd>
          <dt class="text-muted-foreground">{{ $t('admin.settings.autoscalers.detail_instance_type') }}</dt>
          <dd>{{ autoscaler.instance_type }}</dd>
          <dt class="text-muted-foreground">{{ $t('admin.settings.autoscalers.version.label') }}</dt>
          <dd>{{ autoscaler.version }}</dd>
          <dt class="text-muted-foreground">{{ $t('admin.settings.autoscalers.detail_active_agents') }}</dt>
          <dd>{{ autoscaler.active_agents }}</dd>
          <dt class="text-muted-foreground">{{ $t('admin.settings.autoscalers.detail_pending_agents') }}</dt>
          <dd>{{ autoscaler.pending_agents }}</dd>
        </dl>
      </TabsContent>

      <TabsContent value="logs" class="flex-1 min-h-0">
        <ProcLogViewer source="autoscaler" :source-id="id" />
      </TabsContent>
    </Tabs>
  </div>
</template>

<script lang="ts" setup>
import { onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';

import ProcLogViewer from '~/components/log/ProcLogViewer.vue';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '~/components/ui/tabs';
import useApiClient from '~/compositions/useApiClient';
import type { Autoscaler } from '~/lib/api/types/autoscaler';

const route = useRoute();
const apiClient = useApiClient();
const id = Number(route.params.id);

const activeTab = ref<'info' | 'logs'>('info');
const autoscaler = ref<Autoscaler | null>(null);

onMounted(async () => {
  autoscaler.value = await apiClient.getAutoscaler(id);
});
</script>

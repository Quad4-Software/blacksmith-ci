<template>
  <div class="flex flex-col h-full p-4 gap-4">
    <header class="flex items-center gap-3">
      <h1 class="text-xl font-semibold">
        {{ agent?.name ?? $t('admin.settings.agents.detail_loading') }}
      </h1>
      <span v-if="agent" class="text-sm text-muted-foreground">{{
        $t('admin.settings.agents.detail_id', { id: agent.id })
      }}</span>
    </header>

    <Tabs v-model="activeTab" class="flex-1 min-h-0 flex flex-col">
      <TabsList>
        <TabsTrigger value="info">{{ $t('admin.settings.agents.tab_info') }}</TabsTrigger>
        <TabsTrigger value="tasks">{{ $t('admin.settings.agents.tab_tasks') }}</TabsTrigger>
        <TabsTrigger value="logs">{{ $t('admin.settings.agents.tab_logs') }}</TabsTrigger>
      </TabsList>

      <TabsContent value="info" class="flex-1 overflow-auto">
        <dl v-if="agent" class="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-2 text-sm">
          <dt class="text-muted-foreground">{{ $t('admin.settings.agents.platform.platform') }}</dt>
          <dd>{{ agent.platform }}</dd>
          <dt class="text-muted-foreground">{{ $t('admin.settings.agents.backend.backend') }}</dt>
          <dd>{{ agent.backend }}</dd>
          <dt class="text-muted-foreground">{{ $t('admin.settings.agents.version.label') }}</dt>
          <dd>{{ agent.version }}</dd>
        </dl>
      </TabsContent>

      <TabsContent value="tasks" class="flex-1 overflow-auto">
        <p class="text-sm text-muted-foreground">{{ $t('admin.settings.agents.tab_tasks_coming_soon') }}</p>
      </TabsContent>

      <TabsContent value="logs" class="flex-1 min-h-0">
        <ProcLogViewer source="agent" :source-id="id" />
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
import type { Agent } from '~/lib/api/types/agent';

const route = useRoute();
const apiClient = useApiClient();
const id = Number(route.params.id);

const activeTab = ref<'info' | 'tasks' | 'logs'>('info');
const agent = ref<Agent | null>(null);

onMounted(async () => {
  agent.value = await apiClient.getAgent(id);
});
</script>

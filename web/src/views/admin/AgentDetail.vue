<template>
  <div class="flex flex-col h-full p-4 gap-4 w-full min-w-0 flex-1">
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
        <div class="flex items-center gap-2 mb-4">
          <h3 class="text-lg font-semibold">
            {{ $t('admin.settings.agents.tasks_running_count', { count: tasks?.length ?? 0 }) }}
          </h3>
          <div class="flex-1" />
          <Button variant="outline" size="sm" :disabled="tasksLoading" @click="loadTasks">
            <Icon name="lucide:refresh-cw" class="h-4 w-4" :class="{ 'animate-spin': tasksLoading }" />
          </Button>
        </div>

        <QueueTaskList
          v-if="tasksWithStatus.length > 0"
          :tasks="tasksWithStatus"
          :get-status-color="() => 'bg-blue-500'"
          :get-status-label="() => $t('admin.settings.queue.running')"
          :get-status-badge-variant="() => 'default'"
          :time-ago="date.timeAgo"
          :pretty-duration="date.prettyDuration"
        />
        <div v-else-if="!tasksLoading" class="text-center py-12 text-muted-foreground">
          <Icon name="list" class="h-12 w-12 mx-auto mb-3 opacity-50" />
          <p>{{ $t('admin.settings.agents.tasks_empty') }}</p>
        </div>
        <div v-else class="flex items-center justify-center py-12 text-muted-foreground">
          <Icon name="loading" class="h-6 w-6 animate-spin" />
        </div>
      </TabsContent>

      <TabsContent value="logs" class="flex-1 min-h-0">
        <ProcLogViewer source="agent" :source-id="id" />
      </TabsContent>
    </Tabs>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute } from 'vue-router';

import QueueTaskList from '~/components/admin/settings/queue/QueueTaskList.vue';
import Icon from '~/components/atomic/Icon.vue';
import ProcLogViewer from '~/components/log/ProcLogViewer.vue';
import { Button } from '~/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '~/components/ui/tabs';
import useApiClient from '~/compositions/useApiClient';
import { useDate } from '~/compositions/useDate';
import useNotifications from '~/compositions/useNotifications';
import type { Agent, Task } from '~/lib/api/types';

const route = useRoute();
const apiClient = useApiClient();
const date = useDate();
const notifications = useNotifications();
const { t } = useI18n();
const id = Number(route.params.id);

const activeTab = ref<'info' | 'tasks' | 'logs'>('info');
const agent = ref<Agent | null>(null);
const tasks = ref<Task[]>();
const tasksLoading = ref(false);

const tasksWithStatus = computed(() => (tasks.value ?? []).map((task) => ({ ...task, status: 'running' as const })));

async function loadTasks() {
  tasksLoading.value = true;
  try {
    tasks.value = (await apiClient.getAgentTasks(id)) ?? [];
  } catch (err) {
    notifications.notify({
      title: t('notify_title_error'),
      text: err instanceof Error ? err.message : String(err),
      type: 'error',
      duration: 6000,
    });
  } finally {
    tasksLoading.value = false;
  }
}

watch(activeTab, (tab) => {
  if (tab === 'tasks' && tasks.value === undefined) {
    loadTasks();
  }
});

onMounted(async () => {
  agent.value = await apiClient.getAgent(id);
});
</script>

<template>
  <div class="flex flex-col h-full p-4 md:p-6 gap-4 w-full min-w-0 flex-1">
    <!-- Back link -->
    <router-link
      :to="{ name: 'agents' }"
      class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground self-start"
    >
      <Icon name="chevron-left" class="h-4 w-4" />
      {{ $t('admin.settings.agents.back_to_agents') }}
    </router-link>

    <!-- Header -->
    <header v-if="agent" class="flex flex-col gap-2">
      <div class="flex items-center gap-3 flex-wrap">
        <span
          class="shrink-0 w-3 h-3 rounded-full"
          :class="[getAgentStatusColor(agent), getAgentStatusAnimation(agent)]"
          :title="getAgentStatusTitle(agent, t)"
        />
        <h1 class="text-xl font-semibold truncate">{{ getAgentDisplayName(agent) }}</h1>
        <UiBadge v-if="agent.no_schedule" variant="secondary">{{ $t('disabled') }}</UiBadge>
        <UiBadge v-if="agent.org_id === -1" variant="outline">
          {{ $t('admin.settings.agents.type.global') }}
        </UiBadge>
      </div>
      <div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
        <span>{{ $t('admin.settings.agents.detail_id', { id: agent.id }) }}</span>
        <template v-if="agent.platform">
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          <span aria-hidden="true">•</span>
          <span>{{ agent.platform }}</span>
        </template>
        <template v-if="agent.backend">
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          <span aria-hidden="true">•</span>
          <span>{{ agent.backend }}</span>
        </template>
        <template v-if="agent.org_id !== -1">
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          <span aria-hidden="true">•</span>
          <router-link :to="`/orgs/${agent.org_id}`" class="hover:underline">
            {{ $t('admin.settings.agents.org_prefix') }} {{ agent.org_id }}
          </router-link>
        </template>
      </div>
    </header>

    <div v-else class="flex items-center gap-2 text-muted-foreground">
      <Icon name="loading" class="h-4 w-4 animate-spin" />
      <span>{{ $t('admin.settings.agents.detail_loading') }}</span>
    </div>

    <!-- Tabs -->
    <Tabs v-if="agent" v-model="activeTab" class="flex-1 min-h-0 flex flex-col">
      <TabsList class="self-start">
        <TabsTrigger value="info">{{ $t('admin.settings.agents.tab_info') }}</TabsTrigger>
        <TabsTrigger value="tasks" class="gap-2">
          {{ $t('admin.settings.agents.tab_tasks') }}
          <UiBadge v-if="tasks && tasks.length > 0" variant="default" class="ml-1">
            {{ tasks.length }}
          </UiBadge>
        </TabsTrigger>
        <TabsTrigger value="logs">{{ $t('admin.settings.agents.tab_logs') }}</TabsTrigger>
      </TabsList>

      <!-- Info -->
      <TabsContent value="info" class="flex-1 overflow-auto">
        <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          <!-- Identity -->
          <Card>
            <CardHeader>
              <CardTitle class="text-sm">{{ $t('admin.settings.agents.section_identity') }}</CardTitle>
            </CardHeader>
            <CardContent class="space-y-3">
              <MetadataField :label="$t('admin.settings.agents.name.name')" :value="agent.name" />
              <MetadataField :label="$t('admin.settings.agents.agent_id')" :value="String(agent.id)" />
              <MetadataField
                v-if="agent.priority && agent.priority !== 0"
                :label="$t('admin.settings.agents.priority_label')"
                :value="String(agent.priority)"
              />
            </CardContent>
          </Card>

          <!-- Runtime -->
          <Card>
            <CardHeader>
              <CardTitle class="text-sm">{{ $t('admin.settings.agents.section_runtime') }}</CardTitle>
            </CardHeader>
            <CardContent class="space-y-3">
              <MetadataField :label="$t('admin.settings.agents.version.label')">
                <span class="inline-flex items-center gap-1 font-mono text-sm">
                  {{ agent.version || '—' }}
                  <Icon
                    v-if="agent.version && isAgentVersionMismatch(agent.version, config.version)"
                    name="alert"
                    class="h-4 w-4 text-yellow-500"
                    :title="$t('admin.settings.agents.version.mismatch')"
                  />
                </span>
              </MetadataField>
              <MetadataField
                v-if="agent.build_date"
                :label="$t('admin.settings.agents.build_date')"
                :value="agent.build_date"
              />
              <MetadataField :label="$t('admin.settings.agents.last_contact.label')">
                {{ agent.last_contact ? date.timeAgo(agent.last_contact * 1000) : $t('admin.settings.agents.never') }}
              </MetadataField>
              <MetadataField :label="$t('admin.settings.agents.workflows.label')">
                <span class="text-sm">
                  <span class="font-semibold">{{ agent.running_workflows ?? 0 }}</span>
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  <span class="text-muted-foreground" aria-hidden="true"> / </span>
                  <span class="text-muted-foreground">{{ agent.capacity || 0 }}</span>
                </span>
              </MetadataField>
            </CardContent>
          </Card>

          <!-- Limits (Docker only) -->
          <Card v-if="agent.backend === 'docker'">
            <CardHeader>
              <CardTitle class="text-sm">{{ $t('admin.settings.agents.section_limits') }}</CardTitle>
            </CardHeader>
            <CardContent class="space-y-3">
              <MetadataField :label="$t('admin.settings.agents.limit_mem')" :value="formatBytes(agent.limit_mem, t)" />
              <MetadataField
                :label="$t('admin.settings.agents.limit_cpu_quota')"
                :value="formatCPUQuota(agent.limit_cpu_quota, t)"
              />
            </CardContent>
          </Card>
        </div>

        <!-- Custom labels (full width) -->
        <Card class="mt-4">
          <CardHeader>
            <CardTitle class="text-sm">{{ $t('admin.settings.agents.section_labels') }}</CardTitle>
          </CardHeader>
          <CardContent>
            <div v-if="agent.custom_labels && Object.keys(agent.custom_labels).length > 0" class="flex flex-wrap gap-2">
              <UiBadge v-for="(value, key) in agent.custom_labels" :key="`${key}-${value}`" variant="outline">
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span class="font-semibold">{{ key }}<span aria-hidden="true">:</span></span>
                <span class="ml-1">{{ value }}</span>
              </UiBadge>
            </div>
            <p v-else class="text-sm text-muted-foreground">
              {{ $t('admin.settings.agents.no_custom_labels') }}
            </p>
          </CardContent>
        </Card>
      </TabsContent>

      <!-- Tasks -->
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

      <!-- Logs -->
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
import MetadataField from '~/components/repo/pipeline/MetadataField.vue';
import { Badge as UiBadge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import Card from '~/components/ui/card/Card.vue';
import CardContent from '~/components/ui/card/CardContent.vue';
import CardHeader from '~/components/ui/card/CardHeader.vue';
import CardTitle from '~/components/ui/card/CardTitle.vue';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '~/components/ui/tabs';
import useApiClient from '~/compositions/useApiClient';
import useConfig from '~/compositions/useConfig';
import { useDate } from '~/compositions/useDate';
import useNotifications from '~/compositions/useNotifications';
import {
  formatBytes,
  formatCPUQuota,
  getAgentDisplayName,
  getAgentStatusAnimation,
  getAgentStatusColor,
  getAgentStatusTitle,
  isAgentVersionMismatch,
} from '~/lib/agent-utils';
import type { Agent, Task } from '~/lib/api/types';

const route = useRoute();
const apiClient = useApiClient();
const config = useConfig();
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

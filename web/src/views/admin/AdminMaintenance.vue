<template>
  <Settings :title="$t('admin.settings.maintenance.maintenance')" :description="$t('admin.settings.maintenance.desc')">
    <div class="space-y-6">
      <!-- Main Operation Panel -->
      <Card>
        <CardHeader>
          <CardTitle>{{ $t('admin.settings.maintenance.operations.vacuum_log_table') }}</CardTitle>
          <CardDescription>{{ $t('admin.settings.maintenance.operations.vacuum_log_table_desc') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
          <!-- Configuration Row -->
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-6 gap-4 items-end pb-2">
            <!-- Schedule -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.schedule') }}</Label>
              <Input
                v-model="vacuumLogTableConfig.schedule"
                :placeholder="$t('admin.settings.maintenance.schedule_placeholder')"
                :disabled="vacuumEnvVarsSet"
              />
            </div>

            <!-- Status with Enable/Disable -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.status') }}</Label>
              <div class="flex items-center h-9 space-x-2">
                <Checkbox
                  :checked="vacuumLogTableConfig.enabled"
                  :disabled="vacuumEnvVarsSet"
                  @update:checked="(v) => (vacuumLogTableConfig.enabled = v === true)"
                />
                <Label
                  class="cursor-pointer"
                  @click="!vacuumEnvVarsSet && (vacuumLogTableConfig.enabled = !vacuumLogTableConfig.enabled)"
                >
                  {{ $t('admin.settings.maintenance.enabled') }}
                </Label>
              </div>
            </div>

            <!-- Total Runs -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.total_runs') }}</Label>
              <div class="text-sm text-muted-foreground flex items-center h-9">
                {{ vacuumLogTableConfig.total_runs || 0 }}
              </div>
            </div>

            <!-- Options (placeholder to match K8s layout) -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.options') }}</Label>
              <div class="flex items-center h-9">
                <span class="text-sm text-muted-foreground">{{ $t('admin.settings.maintenance.none') }}</span>
              </div>
            </div>

            <!-- Last Run -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.last_run') }}</Label>
              <div class="text-sm text-muted-foreground flex items-center h-9">
                {{ formatLastRun(vacuumLogTableConfig.last_run || '') }}
              </div>
            </div>

            <!-- Actions -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.actions') }}</Label>
              <div class="flex items-center h-9 space-x-2">
                <Button
                  variant="ghost"
                  size="icon"
                  :title="
                    vacuumEnvVarsSet
                      ? $t('admin.settings.maintenance.env_vars_set_tooltip')
                      : $t('admin.settings.maintenance.save_config')
                  "
                  :disabled="isSaving || vacuumEnvVarsSet"
                  @click="saveConfig(false)"
                >
                  <Icon v-if="!isSaving" name="content-save-outline" class="h-4 w-4" />
                  <Icon v-else name="loading" class="h-4 w-4 animate-spin" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  :title="$t('admin.settings.maintenance.run_now')"
                  :disabled="isRunning"
                  @click="runNow"
                >
                  <Icon v-if="!isRunning" name="play-outline" class="h-4 w-4" />
                  <Icon v-else name="loading" class="h-4 w-4 animate-spin" />
                </Button>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Stale Agent Cleanup Operation -->
      <Card>
        <CardHeader>
          <CardTitle>{{ $t('admin.settings.maintenance.operations.stale_agent_cleanup') }}</CardTitle>
          <CardDescription>{{ $t('admin.settings.maintenance.operations.stale_agent_cleanup_desc') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
          <!-- Configuration Row -->
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-6 gap-4 items-end pb-2">
            <!-- Schedule -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.schedule') }}</Label>
              <Input
                v-model="staleAgentCleanupConfig.schedule"
                :placeholder="$t('admin.settings.maintenance.schedule_placeholder')"
                :disabled="staleAgentEnvVarsSet"
              />
            </div>

            <!-- Status with Enable/Disable -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.status') }}</Label>
              <div class="flex items-center h-9 space-x-2">
                <Checkbox
                  :checked="staleAgentCleanupConfig.enabled"
                  :disabled="staleAgentEnvVarsSet"
                  @update:checked="(v) => (staleAgentCleanupConfig.enabled = v === true)"
                />
                <Label
                  class="cursor-pointer"
                  @click="!staleAgentEnvVarsSet && (staleAgentCleanupConfig.enabled = !staleAgentCleanupConfig.enabled)"
                >
                  {{ $t('admin.settings.maintenance.enabled') }}
                </Label>
              </div>
            </div>

            <!-- Total Runs -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.total_runs') }}</Label>
              <div class="text-sm text-muted-foreground flex items-center h-9">
                {{ staleAgentCleanupConfig.total_runs || 0 }}
              </div>
            </div>

            <!-- Options (placeholder to match layout) -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.options') }}</Label>
              <div class="flex items-center h-9">
                <span class="text-sm text-muted-foreground">{{
                  $t('admin.settings.maintenance.stale_duration_24h')
                }}</span>
              </div>
            </div>

            <!-- Last Run -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.last_run') }}</Label>
              <div class="text-sm text-muted-foreground flex items-center h-9">
                {{ formatLastRun(staleAgentCleanupConfig.last_run || '') }}
              </div>
            </div>

            <!-- Actions -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.actions') }}</Label>
              <div class="flex items-center h-9 space-x-2">
                <Button
                  variant="ghost"
                  size="icon"
                  :title="
                    staleAgentEnvVarsSet
                      ? $t('admin.settings.maintenance.env_vars_set_tooltip')
                      : $t('admin.settings.maintenance.save_config')
                  "
                  :disabled="isSavingStaleAgent || staleAgentEnvVarsSet"
                  @click="saveStaleAgentConfig(false)"
                >
                  <Icon v-if="!isSavingStaleAgent" name="content-save-outline" class="h-4 w-4" />
                  <Icon v-else name="loading" class="h-4 w-4 animate-spin" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  :title="$t('admin.settings.maintenance.run_stale_agent_cleanup')"
                  :disabled="isRunningStaleAgentCleanup"
                  @click="runStaleAgentCleanup"
                >
                  <Icon v-if="!isRunningStaleAgentCleanup" name="play-outline" class="h-4 w-4" />
                  <Icon v-else name="loading" class="h-4 w-4 animate-spin" />
                </Button>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Kubernetes Resource Cleanup Operation -->
      <Card v-if="config.kubernetes_cleanup_available">
        <CardHeader>
          <CardTitle>{{ $t('admin.settings.maintenance.operations.kubernetes_cleanup') }}</CardTitle>
          <CardDescription>{{ $t('admin.settings.maintenance.operations.kubernetes_cleanup_desc') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
          <!-- Configuration Row -->
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-6 gap-4 items-end pb-2">
            <!-- Schedule -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.schedule') }}</Label>
              <Input
                v-model="kubernetesCleanupConfig.schedule"
                :placeholder="$t('admin.settings.maintenance.schedule_placeholder')"
                :disabled="kubernetesEnvVarsSet"
              />
            </div>

            <!-- Status with Enable/Disable -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.status') }}</Label>
              <div class="flex items-center h-9 space-x-2">
                <Checkbox
                  :checked="kubernetesCleanupConfig.enabled"
                  :disabled="kubernetesEnvVarsSet"
                  @update:checked="(v) => (kubernetesCleanupConfig.enabled = v === true)"
                />
                <Label
                  class="cursor-pointer"
                  @click="!kubernetesEnvVarsSet && (kubernetesCleanupConfig.enabled = !kubernetesCleanupConfig.enabled)"
                >
                  {{ $t('admin.settings.maintenance.enabled') }}
                </Label>
              </div>
            </div>

            <!-- Total Runs -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.total_runs') }}</Label>
              <div class="text-sm text-muted-foreground flex items-center h-9">
                {{ kubernetesCleanupConfig.total_runs || 0 }}
              </div>
            </div>

            <!-- Options -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.options') }}</Label>
              <div class="flex items-center h-9 space-x-2">
                <Checkbox :checked="k8sCleanupDryRun" @update:checked="(v) => (k8sCleanupDryRun = v === true)" />
                <Label class="cursor-pointer" @click="k8sCleanupDryRun = !k8sCleanupDryRun">
                  {{ $t('admin.settings.maintenance.dry_run') }}
                </Label>
              </div>
            </div>

            <!-- Last Run -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.last_run') }}</Label>
              <div class="text-sm text-muted-foreground flex items-center h-9">
                {{ formatLastK8sCleanup() }}
              </div>
            </div>

            <!-- Actions -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.actions') }}</Label>
              <div class="flex items-center h-9 space-x-2">
                <Button
                  variant="ghost"
                  size="icon"
                  :title="
                    kubernetesEnvVarsSet
                      ? $t('admin.settings.maintenance.env_vars_set_tooltip')
                      : $t('admin.settings.maintenance.save_config')
                  "
                  :disabled="isSavingK8s || kubernetesEnvVarsSet"
                  @click="saveK8sConfig(false)"
                >
                  <Icon v-if="!isSavingK8s" name="content-save-outline" class="h-4 w-4" />
                  <Icon v-else name="loading" class="h-4 w-4 animate-spin" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  :title="
                    kubernetesCleanupConfig.enabled
                      ? $t('admin.settings.maintenance.run_kubernetes_cleanup')
                      : $t('admin.settings.maintenance.k8s_cleanup_disabled')
                  "
                  :disabled="!kubernetesCleanupConfig.enabled || isRunningK8sCleanup"
                  @click="runK8sCleanup"
                >
                  <Icon v-if="!isRunningK8sCleanup" name="play-outline" class="h-4 w-4" />
                  <Icon v-else name="loading" class="h-4 w-4 animate-spin" />
                </Button>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Docker Resource Cleanup Operation -->
      <Card v-if="dockerCleanupAvailable">
        <CardHeader>
          <CardTitle>{{ $t('admin.settings.maintenance.operations.docker_cleanup') }}</CardTitle>
          <CardDescription>{{ $t('admin.settings.maintenance.operations.docker_cleanup_desc') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
          <!-- Configuration Row -->
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-6 gap-4 items-end pb-2">
            <!-- Schedule -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.schedule') }}</Label>
              <Input
                v-model="dockerCleanupConfig.schedule"
                :placeholder="$t('admin.settings.maintenance.schedule_placeholder')"
              />
            </div>

            <!-- Status with Enable/Disable -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.status') }}</Label>
              <div class="flex items-center h-9 space-x-2">
                <Checkbox
                  :checked="dockerCleanupConfig.enabled"
                  @update:checked="(v) => (dockerCleanupConfig.enabled = v === true)"
                />
                <Label class="cursor-pointer" @click="dockerCleanupConfig.enabled = !dockerCleanupConfig.enabled">
                  {{ $t('admin.settings.maintenance.enabled') }}
                </Label>
              </div>
            </div>

            <!-- Total Runs -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.total_runs') }}</Label>
              <div class="text-sm text-muted-foreground flex items-center h-9">
                {{ dockerCleanupConfig.total_runs || 0 }}
              </div>
            </div>

            <!-- Stale Threshold -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.stale_threshold') }}</Label>
              <Input
                v-model="dockerStaleThresholdDisplay"
                placeholder="1h"
                :title="$t('admin.settings.maintenance.stale_threshold_desc')"
              />
            </div>

            <!-- Last Run -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.last_run') }}</Label>
              <div class="text-sm text-muted-foreground flex items-center h-9">
                {{ formatLastRun(dockerCleanupConfig.last_run || '') }}
              </div>
            </div>

            <!-- Actions -->
            <div class="flex flex-col gap-2">
              <Label>{{ $t('admin.settings.maintenance.actions') }}</Label>
              <div class="flex items-center h-9 space-x-2">
                <Button
                  variant="ghost"
                  size="icon"
                  :title="$t('admin.settings.maintenance.save_config')"
                  :disabled="isSavingDocker"
                  @click="saveDockerConfig(false)"
                >
                  <Icon v-if="!isSavingDocker" name="content-save-outline" class="h-4 w-4" />
                  <Icon v-else name="loading" class="h-4 w-4 animate-spin" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  :title="$t('admin.settings.maintenance.run_docker_cleanup')"
                  :disabled="!dockerCleanupConfig.enabled || isRunningDockerCleanup"
                  @click="runDockerCleanup"
                >
                  <Icon v-if="!isRunningDockerCleanup" name="play-outline" class="h-4 w-4" />
                  <Icon v-else name="loading" class="h-4 w-4 animate-spin" />
                </Button>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Recent Logs -->
      <Card v-if="logs.length > 0">
        <CardHeader>
          <CardTitle>{{ $t('admin.settings.maintenance.recent_logs') }}</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{{ $t('admin.settings.maintenance.run_time') }}</TableHead>
                  <TableHead>{{ $t('admin.settings.maintenance.name') }}</TableHead>
                  <TableHead class="text-center">{{ $t('admin.settings.maintenance.status_column') }}</TableHead>
                  <TableHead>{{ $t('admin.settings.maintenance.duration') }}</TableHead>
                  <TableHead>{{ $t('admin.settings.maintenance.error') }}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-for="log in logs" :key="log.id">
                  <TableCell class="whitespace-nowrap">{{ formatDate(log.run_time) }}</TableCell>
                  <TableCell class="whitespace-nowrap">{{ getOperationName(log) }}</TableCell>
                  <TableCell class="text-center">
                    <div class="flex justify-center items-center">
                      <PipelineStatusIcon :status="log.success ? 'success' : 'error'" />
                    </div>
                  </TableCell>
                  <TableCell class="whitespace-nowrap">{{ formatDuration(log.duration) }}</TableCell>
                  <TableCell class="whitespace-nowrap text-destructive">{{ log.error || '-' }}</TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </div>
          <!-- Load More Button -->
          <div v-if="hasMoreLogs" class="mt-4 flex justify-center">
            <Button variant="outline" size="sm" :disabled="isLoadingMore" @click="loadMoreLogs">
              <Icon v-if="isLoadingMore" name="loading" class="h-4 w-4 animate-spin mr-2" />
              {{ $t('admin.settings.maintenance.load_more') }}
            </Button>
          </div>
        </CardContent>
      </Card>

      <!-- Action Log Panel -->
      <Card>
        <CardHeader>
          <div class="flex items-center justify-between">
            <div>
              <CardTitle>{{ $t('admin.settings.maintenance.action_log') }}</CardTitle>
              <CardDescription>{{ $t('admin.settings.maintenance.recent_operations') }}</CardDescription>
            </div>
            <Button v-if="actionLogs.length > 0" variant="outline" size="sm" @click="clearActionLogs">
              <Icon name="trash" class="h-4 w-4 mr-2" />
              {{ $t('admin.settings.maintenance.wipe_log') }}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <div class="bg-muted/50 border rounded-lg p-4 max-h-96 overflow-y-auto">
            <div v-if="actionLogs.length === 0" class="text-center text-muted-foreground py-8">
              {{ $t('admin.settings.maintenance.no_action_logs') }}
            </div>
            <div v-else class="font-mono text-xs space-y-0.5">
              <div v-for="(log, index) in actionLogs" :key="index" class="text-foreground/90">
                {{ log }}
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  </Settings>
</template>

<script lang="ts" setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import Settings from '~/components/layout/Settings.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import { Button } from '~/components/ui/button';
import Card from '~/components/ui/card/Card.vue';
import CardContent from '~/components/ui/card/CardContent.vue';
import CardDescription from '~/components/ui/card/CardDescription.vue';
import CardHeader from '~/components/ui/card/CardHeader.vue';
import CardTitle from '~/components/ui/card/CardTitle.vue';
import { Checkbox } from '~/components/ui/checkbox';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import useApiClient from '~/compositions/useApiClient';
import { useDate } from '~/compositions/useDate';
import useNotifications from '~/compositions/useNotifications';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { MaintenanceConfig, MaintenanceLog, MaintenanceOperationConfig, MaintenanceStats } from '~/lib/api/types';

const apiClient = useApiClient();
const notifications = useNotifications();
const { toLocaleString } = useDate();
const { t } = useI18n();

// Utility function to format duration
const formatDuration = (durationMs: number) => {
  if (durationMs < 1000) {
    return `${durationMs}ms`;
  } else if (durationMs < 60000) {
    return `${(durationMs / 1000).toFixed(1)}s`;
  } else if (durationMs < 3600000) {
    return `${(durationMs / 60000).toFixed(1)}m`;
  } else {
    return `${(durationMs / 3600000).toFixed(1)}h`;
  }
};

const config = ref<MaintenanceConfig>({
  id: 0,
  action_type: 'vacuum',
  enabled: false,
  schedule: '0 5 * * *',
  total_runs: 0,
  successful_runs: 0,
  last_run: '',
  last_run_success: false,
  last_run_duration: 0,
  last_run_error: '',
  created: 0,
  updated: 0,
  kubernetes_cleanup_available: false,
  docker_cleanup_available: false,
  env_vars_set: false,
});

// Single log vacuum operation configuration
const vacuumLogTableConfig = ref<MaintenanceOperationConfig>({
  id: 0,
  operation: 'vacuumLogTable',
  enabled: true,
  schedule: '0 5 * * *',
  total_runs: 0,
  last_run: '',
  created: 0,
  updated: 0,
});

// Kubernetes cleanup operation configuration
const kubernetesCleanupConfig = ref<MaintenanceOperationConfig>({
  id: 0,
  operation: 'kubernetesCleanup',
  enabled: false,
  schedule: '0 5 * * *',
  total_runs: 0,
  last_run: '',
  created: 0,
  updated: 0,
});

// Stale agent cleanup operation configuration
const staleAgentCleanupConfig = ref<MaintenanceOperationConfig>({
  id: 0,
  operation: 'staleAgentCleanup',
  enabled: true,
  schedule: '0 5 * * *',
  total_runs: 0,
  last_run: '',
  created: 0,
  updated: 0,
});

// Docker cleanup operation configuration
const dockerCleanupConfig = ref<MaintenanceOperationConfig & { stale_threshold?: number }>({
  id: 0,
  operation: 'dockerCleanup',
  enabled: false,
  schedule: '0 */1 * * *',
  total_runs: 0,
  last_run: '',
  created: 0,
  updated: 0,
  stale_threshold: 3600,
});
const dockerCleanupAvailable = ref(false);

// Computed property to display stale threshold as human-readable duration
const dockerStaleThresholdDisplay = computed({
  get: () => {
    const seconds = dockerCleanupConfig.value.stale_threshold || 3600;
    if (seconds >= 86400) return `${seconds / 86400}d`;
    if (seconds >= 3600) return `${seconds / 3600}h`;
    if (seconds >= 60) return `${seconds / 60}m`;
    return `${seconds}s`;
  },
  set: (val: string) => {
    const match = val.match(/^(\d+(?:\.\d+)?)\s*([dhms])?$/i);
    if (match) {
      const num = Number.parseFloat(match[1]);
      const unit = (match[2] || 'h').toLowerCase();
      switch (unit) {
        case 'd':
          dockerCleanupConfig.value.stale_threshold = num * 86400;
          break;
        case 'h':
          dockerCleanupConfig.value.stale_threshold = num * 3600;
          break;
        case 'm':
          dockerCleanupConfig.value.stale_threshold = num * 60;
          break;
        case 's':
          dockerCleanupConfig.value.stale_threshold = num;
          break;
      }
    }
  },
});

const logs = ref<MaintenanceLog[]>([]);
const stats = ref<MaintenanceStats | null>(null);
const isSaving = ref(false);
const isRunning = ref(false);
const isSavingK8s = ref(false);
const isRunningK8sCleanup = ref(false);
const isSavingStaleAgent = ref(false);
const isRunningStaleAgentCleanup = ref(false);
const isSavingDocker = ref(false);
const isRunningDockerCleanup = ref(false);
const k8sCleanupDryRun = ref(false);
const actionLogs = ref<string[]>([]);
const isLoadingMore = ref(false);
const logsLimit = ref(10);
const hasMoreLogs = ref(true);

const vacuumEnvVarsSet = ref(false);
const kubernetesEnvVarsSet = ref(false);
const staleAgentEnvVarsSet = ref(false);

// Clear action logs
const clearActionLogs = () => {
  actionLogs.value = [];
};

const formatLastRun = (lastRun: string) => {
  if (!lastRun) return t('admin.settings.maintenance.never');
  return toLocaleString(new Date(lastRun));
};

const formatLastK8sCleanup = () => {
  const k8sLog = logs.value.find((log) => log.details && log.details.includes('Kubernetes resource cleanup'));
  if (k8sLog) {
    return toLocaleString(new Date(k8sLog.run_time));
  }
  return t('admin.settings.maintenance.never');
};

const getOperationName = (log: MaintenanceLog) => {
  if (log.details) {
    // Check for various patterns that indicate Kubernetes cleanup
    if (
      log.details.includes('Kubernetes resource cleanup') ||
      log.details.includes('Kubernetes cleanup') ||
      log.details.includes('K8s cleanup') ||
      log.details.includes('kubernetes_cleanup')
    ) {
      return t('admin.settings.maintenance.operations.kubernetes_cleanup');
    }
    // Check for stale agent cleanup
    if (log.details.includes('Stale agent cleanup') || log.details.includes('stale_agent_cleanup')) {
      return t('admin.settings.maintenance.operations.stale_agent_cleanup');
    }
    // Check for Docker cleanup
    if (log.details.includes('Docker resource cleanup') || log.details.includes('docker_cleanup')) {
      return t('admin.settings.maintenance.operations.docker_cleanup');
    }
  }
  return t('admin.settings.maintenance.operations.vacuum_log_table');
};

const loadData = async (reset = false) => {
  try {
    if (reset) {
      logsLimit.value = 10;
      hasMoreLogs.value = true;
    }

    // Store previous logs to detect new scheduled executions
    const previousLogs = [...logs.value];

    // Load configs for each action separately, plus logs and stats
    const [vacuumConfigData, k8sConfigData, staleAgentConfigData, dockerConfigData, logsData, statsData] =
      await Promise.all([
        apiClient.getMaintenanceConfigByAction('vacuum').catch((error) => {
          console.error('Failed to load vacuum config:', error);
          return null;
        }),
        apiClient.getMaintenanceConfigByAction('kubernetes_cleanup').catch((error) => {
          console.error('Failed to load kubernetes cleanup config:', error);
          return null;
        }),
        apiClient.getMaintenanceConfigByAction('stale_agent_cleanup').catch((error) => {
          console.error('Failed to load stale agent cleanup config:', error);
          return null;
        }),
        apiClient.getMaintenanceConfigByAction('docker_cleanup').catch((error) => {
          console.error('Failed to load docker cleanup config:', error);
          return null;
        }),
        apiClient.getMaintenanceLogs(logsLimit.value).catch((error) => {
          console.error('Failed to load maintenance logs:', error);
          return [];
        }),
        apiClient.getMaintenanceStats().catch((error) => {
          console.error('Failed to load maintenance stats:', error);
          return null;
        }),
      ]);

    if (vacuumConfigData) {
      vacuumLogTableConfig.value.enabled = vacuumConfigData.enabled;
      vacuumLogTableConfig.value.schedule = vacuumConfigData.schedule;
      vacuumLogTableConfig.value.operation = 'vacuumLogTable';
      vacuumLogTableConfig.value.id = vacuumConfigData.id;
      vacuumLogTableConfig.value.created = vacuumConfigData.created;
      vacuumLogTableConfig.value.updated = vacuumConfigData.updated;
      // Copy statistics fields
      vacuumLogTableConfig.value.total_runs = vacuumConfigData.total_runs || 0;
      vacuumLogTableConfig.value.last_run = vacuumConfigData.last_run || '';

      vacuumEnvVarsSet.value = vacuumConfigData.env_vars_set || false;
    } else {
      vacuumLogTableConfig.value = {
        id: 0,
        operation: 'vacuumLogTable',
        enabled: true,
        schedule: '0 5 * * *',
        total_runs: 0,
        last_run: '',
        created: 0,
        updated: 0,
      };
    }

    if (k8sConfigData) {
      kubernetesCleanupConfig.value.enabled = k8sConfigData.enabled;
      kubernetesCleanupConfig.value.schedule = k8sConfigData.schedule;
      kubernetesCleanupConfig.value.operation = 'kubernetesCleanup';
      kubernetesCleanupConfig.value.id = k8sConfigData.id;
      kubernetesCleanupConfig.value.created = k8sConfigData.created;
      kubernetesCleanupConfig.value.updated = k8sConfigData.updated;
      // Copy statistics fields
      kubernetesCleanupConfig.value.total_runs = k8sConfigData.total_runs || 0;
      kubernetesCleanupConfig.value.last_run = k8sConfigData.last_run || '';

      k8sCleanupDryRun.value = k8sConfigData.dry_run || false;

      // Set kubernetes_cleanup_available in global config
      config.value.kubernetes_cleanup_available = k8sConfigData.kubernetes_cleanup_available || false;

      // Set kubernetes-specific env vars flag
      kubernetesEnvVarsSet.value = k8sConfigData.env_vars_set || false;
    } else {
      kubernetesCleanupConfig.value = {
        id: 0,
        operation: 'kubernetesCleanup',
        enabled: false,
        schedule: '0 5 * * *',
        total_runs: 0,
        last_run: '',
        created: 0,
        updated: 0,
      };
    }

    if (staleAgentConfigData) {
      staleAgentCleanupConfig.value.enabled = staleAgentConfigData.enabled;
      staleAgentCleanupConfig.value.schedule = staleAgentConfigData.schedule;
      staleAgentCleanupConfig.value.operation = 'staleAgentCleanup';
      staleAgentCleanupConfig.value.id = staleAgentConfigData.id;
      staleAgentCleanupConfig.value.created = staleAgentConfigData.created;
      staleAgentCleanupConfig.value.updated = staleAgentConfigData.updated;
      // Copy statistics fields
      staleAgentCleanupConfig.value.total_runs = staleAgentConfigData.total_runs || 0;
      staleAgentCleanupConfig.value.last_run = staleAgentConfigData.last_run || '';

      staleAgentEnvVarsSet.value = staleAgentConfigData.env_vars_set || false;
    } else {
      staleAgentCleanupConfig.value = {
        id: 0,
        operation: 'staleAgentCleanup',
        enabled: true,
        schedule: '0 5 * * *',
        total_runs: 0,
        last_run: '',
        created: 0,
        updated: 0,
      };
    }

    if (dockerConfigData) {
      dockerCleanupConfig.value.enabled = dockerConfigData.enabled;
      dockerCleanupConfig.value.schedule = dockerConfigData.schedule;
      dockerCleanupConfig.value.operation = 'dockerCleanup';
      dockerCleanupConfig.value.id = dockerConfigData.id;
      dockerCleanupConfig.value.created = dockerConfigData.created;
      dockerCleanupConfig.value.updated = dockerConfigData.updated;
      dockerCleanupConfig.value.total_runs = dockerConfigData.total_runs || 0;
      dockerCleanupConfig.value.last_run = dockerConfigData.last_run || '';
      dockerCleanupConfig.value.stale_threshold = dockerConfigData.stale_threshold || 3600;
      dockerCleanupAvailable.value = dockerConfigData.docker_cleanup_available || false;
    }

    logs.value = logsData || [];
    hasMoreLogs.value = (logsData || []).length === logsLimit.value;
    stats.value = statsData;

    // Generate action logs for new scheduled executions or initial page load
    let logsToProcess: MaintenanceLog[] = [];

    if (!reset && previousLogs.length > 0 && logsData && logsData.length > 0) {
      // Find new logs that weren't in the previous logs (for periodic updates)
      logsToProcess = logsData.filter((newLog) => !previousLogs.some((prevLog) => prevLog.id === newLog.id));
    } else if (reset && logsData && logsData.length > 0) {
      // Initial page load or manual refresh - generate action logs for recent logs
      // Clear existing action logs to avoid duplicates
      actionLogs.value = [];
      // Process last 5 logs to avoid spam, but reverse to maintain chronological order
      logsToProcess = logsData.slice(0, 5).reverse();
    }

    if (logsToProcess.length > 0) {
      // Add action log entries for processed logs
      for (const newLog of logsToProcess) {
        const logTime = new Date(newLog.run_time);
        const year = logTime.getFullYear().toString();
        const month = (logTime.getMonth() + 1).toString();
        const day = logTime.getDate().toString();
        const time = logTime.toLocaleTimeString('en-US', {
          hour12: false,
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
        });
        const jobTitle = getOperationName(newLog);

        // Add divider for new scheduled run
        if (actionLogs.value.length > 0) {
          actionLogs.value.unshift(`[${year}/${month}/${day} ${time}] [${jobTitle}] ${'─'.repeat(30)}`);
        }

        // Determine if this was a manual or scheduled run
        const triggerType = newLog.details?.includes('Trigger: Manual') ? 'Manual' : 'Scheduled';

        actionLogs.value.unshift(
          `[${year}/${month}/${day} ${time}] [${jobTitle}] ${triggerType} maintenance execution detected`,
        );

        if (newLog.success) {
          actionLogs.value.unshift(
            `[${year}/${month}/${day} ${time}] [${jobTitle}] ✅ Operation completed in ${formatDuration(newLog.duration)}`,
          );

          if (newLog.details) {
            // Parse operation details
            const operations = [];
            if (newLog.details.includes('VACUUM')) {
              operations.push('VACUUM');
            }
            if (newLog.details.includes('OPTIMIZE')) {
              operations.push('OPTIMIZE');
            }
            if (newLog.details.includes('ANALYZE')) {
              operations.push('ANALYZE');
            }

            if (operations.length > 0) {
              actionLogs.value.unshift(
                `[${year}/${month}/${day} ${time}] [${jobTitle}] Operations performed: ${operations.join(', ')}`,
              );
            }

            // Check if size information is available in details
            const beforeMatch = newLog.details.match(/Before: ([\d.]+) MB/);
            const afterMatch = newLog.details.match(/After: ([\d.]+) MB/);
            const savedMatch = newLog.details.match(/Saved: ([\d.]+) MB/);

            if (beforeMatch && afterMatch) {
              actionLogs.value.unshift(
                `[${year}/${month}/${day} ${time}] [${jobTitle}] Table size before: ${beforeMatch[1]} MB`,
              );
              actionLogs.value.unshift(
                `[${year}/${month}/${day} ${time}] [${jobTitle}] Table size after: ${afterMatch[1]} MB`,
              );
              if (savedMatch && Number.parseFloat(savedMatch[1]) > 0) {
                actionLogs.value.unshift(
                  `[${year}/${month}/${day} ${time}] [${jobTitle}] Space reclaimed: ${savedMatch[1]} MB`,
                );
              } else {
                actionLogs.value.unshift(`[${year}/${month}/${day} ${time}] [${jobTitle}] Space reclaimed: 0.00 MB`);
              }
            }

            // Parse Stale agent cleanup details
            if (newLog.details.includes('Stale agent cleanup')) {
              const totalAgentsMatch = newLog.details.match(/Total agents: (\d+)/);
              const staleFoundMatch = newLog.details.match(/Stale found: (\d+)/);
              const deletedMatch = newLog.details.match(/Deleted: (\d+)/);

              if (totalAgentsMatch) {
                const totalAgents = Number.parseInt(totalAgentsMatch[1]);
                const staleFound = staleFoundMatch ? Number.parseInt(staleFoundMatch[1]) : 0;
                const deleted = deletedMatch ? Number.parseInt(deletedMatch[1]) : 0;

                actionLogs.value.unshift(
                  `[${year}/${month}/${day} ${time}] [${jobTitle}] Total agents: ${totalAgents}`,
                );

                if (staleFound === 0) {
                  actionLogs.value.unshift(
                    `[${year}/${month}/${day} ${time}] [${jobTitle}] No stale agents found - all agents are active!`,
                  );
                } else {
                  actionLogs.value.unshift(
                    `[${year}/${month}/${day} ${time}] [${jobTitle}] Found ${staleFound} stale agents`,
                  );

                  if (deleted > 0) {
                    actionLogs.value.unshift(
                      `[${year}/${month}/${day} ${time}] [${jobTitle}] Successfully deleted ${deleted} stale agents`,
                    );
                  }

                  if (staleFound > deleted) {
                    const failed = staleFound - deleted;
                    actionLogs.value.unshift(
                      `[${year}/${month}/${day} ${time}] [${jobTitle}] Failed to delete ${failed} agents`,
                    );
                  }
                }
              }
            }

            // Parse Docker cleanup details
            if (newLog.details.includes('Docker resource cleanup')) {
              const agentMatch = newLog.details.match(/Agent: ([^ ]+)/);
              const agentInfo = agentMatch ? ` (agent: ${agentMatch[1]})` : '';
              const containersMatch = newLog.details.match(/Containers: found=(\d+) removed=(\d+)/);
              const networksMatch = newLog.details.match(/Networks: found=(\d+) removed=(\d+)/);
              const volumesMatch = newLog.details.match(/Volumes: found=(\d+) removed=(\d+)/);

              const parts = [];
              if (containersMatch) parts.push(`Containers: ${containersMatch[2]}/${containersMatch[1]} removed`);
              if (networksMatch) parts.push(`Networks: ${networksMatch[2]}/${networksMatch[1]} removed`);
              if (volumesMatch) parts.push(`Volumes: ${volumesMatch[2]}/${volumesMatch[1]} removed`);

              if (parts.length > 0) {
                actionLogs.value.unshift(
                  `[${year}/${month}/${day} ${time}] [${jobTitle}] ${parts.join(', ')}${agentInfo}`,
                );
              } else {
                actionLogs.value.unshift(
                  `[${year}/${month}/${day} ${time}] [${jobTitle}] No stale resources found${agentInfo}`,
                );
              }
            }

            // Parse Kubernetes cleanup details
            if (newLog.details.includes('Kubernetes resource cleanup')) {
              const resourcesFoundMatch = newLog.details.match(/Resources found: (\d+)/);
              const servicesMatch = newLog.details.match(/Services: (\d+)/);
              const pvcsMatch = newLog.details.match(/PVCs: (\d+)/);
              const secretsMatch = newLog.details.match(/Secrets: (\d+)/);
              const deletedMatch = newLog.details.match(/Deleted: (\d+)/);
              const failedMatch = newLog.details.match(/Failed: (\d+)/);
              const wouldDeleteMatch = newLog.details.match(/Would delete: (\d+)/);

              if (newLog.details.includes('No stale resources found')) {
                actionLogs.value.unshift(
                  `[${year}/${month}/${day} ${time}] [${jobTitle}] No stale resources found - all clean!`,
                );
              } else if (resourcesFoundMatch) {
                const totalFound = Number.parseInt(resourcesFoundMatch[1]);
                const services = servicesMatch ? Number.parseInt(servicesMatch[1]) : 0;
                const pvcs = pvcsMatch ? Number.parseInt(pvcsMatch[1]) : 0;
                const secrets = secretsMatch ? Number.parseInt(secretsMatch[1]) : 0;

                actionLogs.value.unshift(
                  `[${year}/${month}/${day} ${time}] [${jobTitle}] Found ${totalFound} stale resources: ${services} services, ${pvcs} PVCs, ${secrets} secrets`,
                );

                if (wouldDeleteMatch) {
                  const wouldDelete = Number.parseInt(wouldDeleteMatch[1]);
                  actionLogs.value.unshift(
                    `[${year}/${month}/${day} ${time}] [${jobTitle}] Dry run - would delete ${wouldDelete} resources`,
                  );
                } else if (deletedMatch) {
                  const deleted = Number.parseInt(deletedMatch[1]);
                  actionLogs.value.unshift(
                    `[${year}/${month}/${day} ${time}] [${jobTitle}] Successfully deleted ${deleted} stale resources`,
                  );

                  if (failedMatch) {
                    const failed = Number.parseInt(failedMatch[1]);
                    if (failed > 0) {
                      actionLogs.value.unshift(
                        `[${year}/${month}/${day} ${time}] [${jobTitle}] Failed to delete ${failed} resources`,
                      );
                    }
                  }
                }
              }
            }
          }
        } else {
          actionLogs.value.unshift(
            `[${year}/${month}/${day} ${time}] [${jobTitle}] ❌ Operation failed after ${formatDuration(newLog.duration)}`,
          );
          if (newLog.error) {
            actionLogs.value.unshift(`[${year}/${month}/${day} ${time}] [${jobTitle}] Error: ${newLog.error}`);
          }
        }

        // Keep only the last 50 action log lines
        if (actionLogs.value.length > 50) {
          actionLogs.value = actionLogs.value.slice(0, 50);
        }
      }

      // (action logs are rebuilt from server data on each load)
    }
  } catch (error) {
    console.error('Failed to load maintenance data:', error);
    // Use defaults if everything fails
    vacuumLogTableConfig.value = {
      id: 0,
      operation: 'vacuumLogTable',
      enabled: false,
      schedule: '0 5 * * *',
      total_runs: 0,
      last_run: '',
      created: 0,
      updated: 0,
    };
  }
};

const loadMoreLogs = async () => {
  if (!hasMoreLogs.value || isLoadingMore.value) return;

  isLoadingMore.value = true;
  try {
    logsLimit.value += 10;
    const moreLogsData = await apiClient.getMaintenanceLogs(logsLimit.value);
    logs.value = moreLogsData || [];
    hasMoreLogs.value = (moreLogsData || []).length === logsLimit.value;

    // Force scroll container to recognize new content height
    await nextTick();
  } catch (error) {
    console.error('Failed to load more logs:', error);
  } finally {
    isLoadingMore.value = false;
  }
};

const addActionLog = (message: string, jobType: string = 'vacuum') => {
  const now = new Date();
  const year = now.getFullYear().toString();
  const month = (now.getMonth() + 1).toString();
  const day = now.getDate().toString();
  const time = now.toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
  let jobTitle = t('admin.settings.maintenance.operations.vacuum_log_table');
  if (jobType === 'kubernetes') {
    jobTitle = t('admin.settings.maintenance.operations.kubernetes_cleanup');
  } else if (jobType === 'stale_agent') {
    jobTitle = t('admin.settings.maintenance.operations.stale_agent_cleanup');
  } else if (jobType === 'docker') {
    jobTitle = t('admin.settings.maintenance.operations.docker_cleanup');
  }
  actionLogs.value.unshift(`[${year}/${month}/${day} ${time}] [${jobTitle}] ${message}`);
  // Keep only the last 50 action log lines
  if (actionLogs.value.length > 50) {
    actionLogs.value = actionLogs.value.slice(0, 50);
  }
};

const saveStaleAgentConfig = async (silent = false) => {
  isSavingStaleAgent.value = true;
  if (!silent) {
    addActionLog('Saving stale agent cleanup configuration...', 'stale_agent');
  }

  try {
    // Track what's changing
    const originalEnabled = staleAgentCleanupConfig.value.enabled;
    const originalSchedule = staleAgentCleanupConfig.value.schedule;

    const updatedConfig = await apiClient.updateMaintenanceConfigByAction('stale_agent_cleanup', {
      enabled: staleAgentCleanupConfig.value.enabled,
      schedule: staleAgentCleanupConfig.value.schedule,
    });

    // Update local config with server response
    staleAgentCleanupConfig.value.enabled = updatedConfig.enabled;
    staleAgentCleanupConfig.value.schedule = updatedConfig.schedule;
    staleAgentCleanupConfig.value.id = updatedConfig.id;
    staleAgentCleanupConfig.value.created = updatedConfig.created;
    staleAgentCleanupConfig.value.updated = updatedConfig.updated;

    const changes = [];
    if (originalEnabled !== updatedConfig.enabled) {
      changes.push(`enabled: ${originalEnabled} → ${updatedConfig.enabled}`);
    }
    if (originalSchedule !== updatedConfig.schedule) {
      changes.push(`schedule: ${originalSchedule} → ${updatedConfig.schedule}`);
    }

    if (changes.length > 0) {
      if (silent) {
        addActionLog(`${changes.join(', ')}`, 'stale_agent');
      } else {
        addActionLog(`Configuration saved with changes: ${changes.join(', ')}`, 'stale_agent');
      }
    } else if (!silent) {
      addActionLog('Configuration saved (no changes)', 'stale_agent');
    }

    if (!silent) {
      notifications.notify({
        title: t('admin.settings.maintenance.notify_text_config_saved'),
        type: 'success',
      });
    }
  } catch (error) {
    console.error('Failed to save stale agent cleanup configuration:', error);
    const errorMessage = (error as Error).message || 'Unknown error';
    addActionLog(`Error saving configuration: ${errorMessage}`, 'stale_agent');

    if (!silent) {
      notifications.notify({
        title: t('admin.settings.maintenance.notify_text_config_error'),
        type: 'error',
      });
    }
  } finally {
    isSavingStaleAgent.value = false;
  }
};

const saveDockerConfig = async (silent = false) => {
  isSavingDocker.value = true;
  if (!silent) {
    addActionLog('Saving Docker cleanup configuration...', 'docker');
  }

  try {
    const updatedConfig = await apiClient.updateMaintenanceConfigByAction('docker_cleanup', {
      enabled: dockerCleanupConfig.value.enabled,
      schedule: dockerCleanupConfig.value.schedule,
      stale_threshold: dockerCleanupConfig.value.stale_threshold || 3600,
    });

    dockerCleanupConfig.value.enabled = updatedConfig.enabled;
    dockerCleanupConfig.value.schedule = updatedConfig.schedule;
    dockerCleanupConfig.value.id = updatedConfig.id;
    dockerCleanupConfig.value.created = updatedConfig.created;
    dockerCleanupConfig.value.updated = updatedConfig.updated;
    dockerCleanupConfig.value.stale_threshold = updatedConfig.stale_threshold || 3600;

    if (!silent) {
      addActionLog('Configuration saved', 'docker');
      notifications.notify({
        title: t('admin.settings.maintenance.notify_text_config_saved'),
        type: 'success',
      });
    }
  } catch (error) {
    console.error('Failed to save Docker cleanup configuration:', error);
    addActionLog(`Error saving configuration: ${(error as Error).message}`, 'docker');
    if (!silent) {
      notifications.notify({
        title: t('admin.settings.maintenance.notify_text_config_error'),
        type: 'error',
      });
    }
  } finally {
    isSavingDocker.value = false;
  }
};

const runDockerCleanup = async () => {
  isRunningDockerCleanup.value = true;
  addActionLog('Triggering Docker resource cleanup...', 'docker');

  try {
    await apiClient.runMaintenanceNow('dockerCleanup');
    addActionLog('Docker cleanup triggered on agent, waiting for result...', 'docker');
    notifications.notify({
      title: t('admin.settings.maintenance.notify_text_docker_cleanup_started'),
      type: 'success',
    });

    // Poll for results since cleanup can take 10+ seconds
    const previousLogCount = logs.value.length;
    const pollInterval = 5000;
    const maxAttempts = 12; // up to 60s
    let attempts = 0;
    const poll = async () => {
      attempts++;
      await loadData();
      if (logs.value.length > previousLogCount || attempts >= maxAttempts) {
        isRunningDockerCleanup.value = false;
      } else {
        setTimeout(() => void poll(), pollInterval);
      }
    };
    setTimeout(() => void poll(), pollInterval);
  } catch (error) {
    console.error('Failed to trigger Docker cleanup:', error);
    addActionLog(`Error: ${(error as Error).message}`, 'docker');
    notifications.notify({
      title: t('admin.settings.maintenance.notify_text_docker_cleanup_error'),
      type: 'error',
    });
    isRunningDockerCleanup.value = false;
  }
};

const saveK8sConfig = async (silent = false) => {
  isSavingK8s.value = true;
  if (!silent) {
    addActionLog('Saving Kubernetes cleanup configuration...', 'kubernetes');
  }

  try {
    // Track what's changing
    const originalEnabled = kubernetesCleanupConfig.value.enabled;
    const originalSchedule = kubernetesCleanupConfig.value.schedule;
    const originalDryRun = k8sCleanupDryRun.value;

    const updatedConfig = await apiClient.updateMaintenanceConfigByAction('kubernetes_cleanup', {
      enabled: kubernetesCleanupConfig.value.enabled,
      schedule: kubernetesCleanupConfig.value.schedule,
      dry_run: k8sCleanupDryRun.value,
    });

    // Update local config with server response
    kubernetesCleanupConfig.value.enabled = updatedConfig.enabled;
    kubernetesCleanupConfig.value.schedule = updatedConfig.schedule;
    kubernetesCleanupConfig.value.id = updatedConfig.id;
    kubernetesCleanupConfig.value.created = updatedConfig.created;
    kubernetesCleanupConfig.value.updated = updatedConfig.updated;
    k8sCleanupDryRun.value = updatedConfig.dry_run || false;

    config.value.kubernetes_cleanup_available = updatedConfig.kubernetes_cleanup_available || false;

    const changes = [];
    if (originalEnabled !== updatedConfig.enabled) {
      changes.push(`enabled: ${originalEnabled} → ${updatedConfig.enabled}`);
    }
    if (originalSchedule !== updatedConfig.schedule) {
      changes.push(`schedule: ${originalSchedule} → ${updatedConfig.schedule}`);
    }
    if (originalDryRun !== updatedConfig.dry_run) {
      changes.push(`dry_run: ${originalDryRun} → ${updatedConfig.dry_run}`);
    }

    if (changes.length > 0) {
      if (silent) {
        addActionLog(`${changes.join(', ')}`, 'kubernetes');
      } else {
        addActionLog(`Configuration saved with changes: ${changes.join(', ')}`, 'kubernetes');
      }
    } else if (!silent) {
      addActionLog('Configuration saved (no changes)', 'kubernetes');
    }

    if (!silent) {
      notifications.notify({
        title: t('admin.settings.maintenance.notify_text_config_saved'),
        type: 'success',
      });
    }
  } catch (error) {
    console.error('Failed to save Kubernetes cleanup configuration:', error);
    const errorMessage = (error as Error).message || 'Unknown error';
    addActionLog(`Error saving configuration: ${errorMessage}`, 'kubernetes');

    if (!silent) {
      notifications.notify({
        title: t('admin.settings.maintenance.notify_text_config_error'),
        type: 'error',
      });
    }
  } finally {
    isSavingK8s.value = false;
  }
};

const saveConfig = async (silent = false) => {
  isSaving.value = true;
  if (!silent) {
    addActionLog('Saving vacuum maintenance configuration...');
  }

  try {
    // Track what's changing
    const originalEnabled = vacuumLogTableConfig.value.enabled;
    const originalSchedule = vacuumLogTableConfig.value.schedule;

    // Save vacuum-specific config
    const updatedConfig = await apiClient.updateMaintenanceConfigByAction('vacuum', {
      enabled: vacuumLogTableConfig.value.enabled,
      schedule: vacuumLogTableConfig.value.schedule,
    });

    // Update local config with server response
    vacuumLogTableConfig.value.enabled = updatedConfig.enabled;
    vacuumLogTableConfig.value.schedule = updatedConfig.schedule;
    vacuumLogTableConfig.value.id = updatedConfig.id;
    vacuumLogTableConfig.value.created = updatedConfig.created;
    vacuumLogTableConfig.value.updated = updatedConfig.updated;

    // Show specific changes
    const changes = [];
    if (originalEnabled !== updatedConfig.enabled) {
      changes.push(`enabled: ${originalEnabled} → ${updatedConfig.enabled}`);
    }
    if (originalSchedule !== updatedConfig.schedule) {
      changes.push(`schedule: ${originalSchedule} → ${updatedConfig.schedule}`);
    }

    if (changes.length > 0) {
      if (silent) {
        addActionLog(`${changes.join(', ')}`);
      } else {
        addActionLog(`Configuration saved with changes: ${changes.join(', ')}`);
      }
    } else if (!silent) {
      addActionLog('Configuration saved (no changes)');
    }

    if (!silent) {
      notifications.notify({
        title: t('admin.settings.maintenance.notify_text_config_saved'),
        type: 'success',
      });
    }
  } catch (error) {
    console.error('Failed to save vacuum maintenance configuration:', error);
    const errorMessage = (error as Error).message || 'Unknown error';
    addActionLog(`Error saving configuration: ${errorMessage}`);
    if (!silent) {
      notifications.notify({
        title: t('admin.settings.maintenance.notify_text_config_error'),
        type: 'error',
      });
    }
  } finally {
    isSaving.value = false;
  }
};

const runNow = async () => {
  isRunning.value = true;

  // Add divider if there are existing logs
  if (actionLogs.value.length > 0) {
    addActionLog('─'.repeat(50));
  }

  addActionLog('Starting log vacuum maintenance operation...');

  try {
    // const _logCountBefore = logs.value.length;
    const startTime = Date.now();

    // Get initial stats for comparison
    // const _initialStats = stats.value;

    await apiClient.runMaintenanceNow('vacuumLogTable');

    notifications.notify({
      title: t('admin.settings.maintenance.notify_text_run_started'),
      type: 'success',
    });

    // Poll for completion with exponential backoff
    let pollCount = 0;
    const maxPolls = 5; // Try up to 5 times
    const pollDelays = [2000, 3000, 5000, 8000, 10000]; // Wait 2s, 3s, 5s, 8s, 10s

    const checkForCompletion = async () => {
      await loadData(true);

      // Check if we have any logs and if the latest one is from after we started
      if (logs.value.length > 0) {
        const latestLog = logs.value[0];
        const latestLogTime = new Date(latestLog.run_time).getTime();

        // Check if this log is from our current operation (with 2 second tolerance for clock differences)
        if (latestLogTime >= startTime - 2000) {
          const duration = latestLog.duration;
          const success = latestLog.success;
          const details = latestLog.details;

          // Log the actual details we received for debugging
          // console.log('Maintenance log details:', details);

          if (success) {
            if (details) {
              // Parse operation details
              const operations = [];
              if (details.includes('VACUUM')) {
                operations.push('VACUUM');
              }
              if (details.includes('OPTIMIZE')) {
                operations.push('OPTIMIZE');
              }
              if (details.includes('ANALYZE')) {
                operations.push('ANALYZE');
              }

              addActionLog(`✅ Operation completed in ${formatDuration(duration)}`);
              addActionLog(`Operations performed: ${operations.join(', ')}`);

              // Check if size information is available in details
              const beforeMatch = details.match(/Before: ([\d.]+) MB/);
              const afterMatch = details.match(/After: ([\d.]+) MB/);
              const savedMatch = details.match(/Saved: ([\d.]+) MB/);

              if (beforeMatch && afterMatch) {
                addActionLog(`Table size before: ${beforeMatch[1]} MB`);
                addActionLog(`Table size after: ${afterMatch[1]} MB`);
                if (savedMatch && Number.parseFloat(savedMatch[1]) > 0) {
                  addActionLog(`Space reclaimed: ${savedMatch[1]} MB`);
                } else {
                  addActionLog(`Space reclaimed: 0.00 MB`);
                }
              } else {
                // No size information available
                addActionLog(`Note: Database size statistics were not captured for this operation`);
              }
            } else {
              addActionLog(`✅ Operation completed in ${formatDuration(duration)}`);
            }
          } else {
            addActionLog(`❌ Operation failed after ${formatDuration(duration)}`);
            if (latestLog.error) {
              addActionLog(`Error: ${latestLog.error}`);
            }
          }

          isRunning.value = false;
          return; // Exit after processing the log
        }
      }

      // No new log entry found yet, try again if we haven't exceeded max polls
      pollCount++;
      if (pollCount < maxPolls) {
        addActionLog(`Still running... checking again in ${pollDelays[pollCount] / 1000}s`);
        setTimeout(checkForCompletion, pollDelays[pollCount]);
        return; // Don't set isRunning to false yet
      } else {
        // Exceeded max polls, show generic completion message
        const totalDuration = Date.now() - startTime;
        addActionLog(`Operation completed (${formatDuration(totalDuration)} total)`);

        // Do one final data reload to ensure we have the latest logs
        setTimeout(() => loadData(true), 2000);
      }

      isRunning.value = false;
    };

    // Start polling after initial delay
    setTimeout(checkForCompletion, pollDelays[0]);
  } catch (error) {
    const errorMessage = (error as Error).message || 'Unknown error';
    addActionLog(`❌ Error starting operation: ${errorMessage}`);
    notifications.notify({
      title: t('admin.settings.maintenance.notify_text_run_error'),
      type: 'error',
    });
    isRunning.value = false;
  }
};

const runStaleAgentCleanup = async () => {
  isRunningStaleAgentCleanup.value = true;

  // Add divider if there are existing logs
  if (actionLogs.value.length > 0) {
    addActionLog('─'.repeat(50), 'stale_agent');
  }

  addActionLog('Starting stale agent cleanup...', 'stale_agent');

  try {
    const startTime = Date.now();

    await apiClient.runMaintenanceNow('staleAgentCleanup');

    notifications.notify({
      title: t('admin.settings.maintenance.notify_text_stale_agent_cleanup_started'),
      type: 'success',
    });

    // Poll for completion with exponential backoff
    let pollCount = 0;
    const maxPolls = 5; // Try up to 5 times
    const pollDelays = [2000, 3000, 5000, 8000, 10000]; // Wait 2s, 3s, 5s, 8s, 10s

    const checkForCompletion = async () => {
      await loadData(true);

      // Check if we have any logs and if the latest one is from after we started
      if (logs.value.length > 0) {
        const latestLog = logs.value[0];
        const latestLogTime = new Date(latestLog.run_time).getTime();

        // Check if this log is from our current operation (with 2 second tolerance for clock differences)
        if (
          latestLogTime >= startTime - 2000 &&
          latestLog.details &&
          latestLog.details.includes('Stale agent cleanup')
        ) {
          const duration = latestLog.duration;
          const success = latestLog.success;

          if (success) {
            addActionLog(`✅ Stale agent cleanup completed in ${formatDuration(duration)}`, 'stale_agent');

            // Parse details for stale agents found/deleted
            const details = latestLog.details;
            const totalAgentsMatch = details.match(/Total agents: (\d+)/);
            const staleFoundMatch = details.match(/Stale found: (\d+)/);
            const deletedMatch = details.match(/Deleted: (\d+)/);

            if (totalAgentsMatch) {
              const totalAgents = Number.parseInt(totalAgentsMatch[1]);
              const staleFound = staleFoundMatch ? Number.parseInt(staleFoundMatch[1]) : 0;
              const deleted = deletedMatch ? Number.parseInt(deletedMatch[1]) : 0;

              addActionLog(`Total agents: ${totalAgents}`, 'stale_agent');

              if (staleFound === 0) {
                addActionLog('No stale agents found - all agents are active!', 'stale_agent');
              } else {
                addActionLog(`Found ${staleFound} stale agents`, 'stale_agent');
                if (deleted > 0) {
                  addActionLog(`Successfully deleted ${deleted} stale agents`, 'stale_agent');
                }
                if (staleFound > deleted) {
                  addActionLog(`Failed to delete ${staleFound - deleted} agents`, 'stale_agent');
                }
              }
            } else if (staleFoundMatch) {
              // Fallback if total agents not in details
              const staleFound = Number.parseInt(staleFoundMatch[1]);
              const deleted = deletedMatch ? Number.parseInt(deletedMatch[1]) : 0;

              if (staleFound === 0) {
                addActionLog('No stale agents found', 'stale_agent');
              } else {
                addActionLog(`Found ${staleFound} stale agents`, 'stale_agent');
                if (deleted > 0) {
                  addActionLog(`Successfully deleted ${deleted} agents`, 'stale_agent');
                }
                if (staleFound > deleted) {
                  addActionLog(`Failed to delete ${staleFound - deleted} agents`, 'stale_agent');
                }
              }
            }
          } else {
            addActionLog(`❌ Stale agent cleanup failed after ${formatDuration(duration)}`, 'stale_agent');
            if (latestLog.error) {
              addActionLog(`Error: ${latestLog.error}`, 'stale_agent');
            }
          }

          isRunningStaleAgentCleanup.value = false;
          return; // Exit after processing the log
        }
      }

      // No new log entry found yet, try again if we haven't exceeded max polls
      pollCount++;
      if (pollCount < maxPolls) {
        addActionLog(`Still running... checking again in ${pollDelays[pollCount] / 1000}s`, 'stale_agent');
        setTimeout(checkForCompletion, pollDelays[pollCount]);
        return; // Don't set isRunning to false yet
      } else {
        // Exceeded max polls, show generic completion message
        const totalDuration = Date.now() - startTime;
        addActionLog(`Operation completed (${formatDuration(totalDuration)} total)`, 'stale_agent');

        // Do one final data reload to ensure we have the latest logs
        setTimeout(() => loadData(true), 2000);
      }

      isRunningStaleAgentCleanup.value = false;
    };

    // Start polling after initial delay
    setTimeout(checkForCompletion, pollDelays[0]);
  } catch (error) {
    const errorMessage = (error as Error).message || 'Unknown error';
    addActionLog(`❌ Error starting stale agent cleanup: ${errorMessage}`, 'stale_agent');
    notifications.notify({
      title: t('admin.settings.maintenance.notify_text_stale_agent_cleanup_error'),
      type: 'error',
    });
    isRunningStaleAgentCleanup.value = false;
  }
};

const runK8sCleanup = async () => {
  isRunningK8sCleanup.value = true;

  // Add divider if there are existing logs
  if (actionLogs.value.length > 0) {
    addActionLog('─'.repeat(50), 'kubernetes');
  }

  addActionLog(`Starting Kubernetes resource cleanup${k8sCleanupDryRun.value ? ' (dry run)' : ''}...`, 'kubernetes');

  try {
    const startTime = Date.now();

    await apiClient.runMaintenanceNow('kubernetesCleanup', k8sCleanupDryRun.value);

    notifications.notify({
      title: t('admin.settings.maintenance.notify_text_k8s_cleanup_started'),
      type: 'success',
    });

    // Poll for completion with exponential backoff
    let pollCount = 0;
    const maxPolls = 5; // Try up to 5 times
    const pollDelays = [2000, 3000, 5000, 8000, 10000]; // Wait 2s, 3s, 5s, 8s, 10s

    const checkForCompletion = async () => {
      await loadData(true);

      // Check if we have any logs and if the latest one is from after we started
      if (logs.value.length > 0) {
        const latestLog = logs.value[0];
        const latestLogTime = new Date(latestLog.run_time).getTime();

        // Check if this log is from our current operation (with 2 second tolerance for clock differences)
        if (
          latestLogTime >= startTime - 2000 &&
          latestLog.details &&
          latestLog.details.includes('Kubernetes resource cleanup')
        ) {
          const duration = latestLog.duration;
          const success = latestLog.success;

          if (success) {
            addActionLog(`✅ Kubernetes cleanup completed in ${formatDuration(duration)}`, 'kubernetes');
          } else {
            addActionLog(`❌ Kubernetes cleanup failed after ${formatDuration(duration)}`, 'kubernetes');
            if (latestLog.error) {
              addActionLog(`Error: ${latestLog.error}`, 'kubernetes');
            }
          }

          isRunningK8sCleanup.value = false;
          return; // Exit after processing the log
        }
      }

      // No new log entry found yet, try again if we haven't exceeded max polls
      pollCount++;
      if (pollCount < maxPolls) {
        addActionLog(`Still running... checking again in ${pollDelays[pollCount] / 1000}s`, 'kubernetes');
        setTimeout(checkForCompletion, pollDelays[pollCount]);
        return; // Don't set isRunning to false yet
      } else {
        // Exceeded max polls, show generic completion message
        const totalDuration = Date.now() - startTime;
        addActionLog(`Operation completed (${formatDuration(totalDuration)} total)`, 'kubernetes');

        // Do one final data reload to ensure we have the latest logs
        setTimeout(() => loadData(true), 2000);
      }

      isRunningK8sCleanup.value = false;
    };

    // Start polling after initial delay
    setTimeout(checkForCompletion, pollDelays[0]);
  } catch (error) {
    const errorMessage = (error as Error).message || 'Unknown error';
    addActionLog(`❌ Error starting Kubernetes cleanup: ${errorMessage}`, 'kubernetes');
    notifications.notify({
      title: t('admin.settings.maintenance.notify_text_k8s_cleanup_error'),
      type: 'error',
    });
    isRunningK8sCleanup.value = false;
  }
};

const formatDate = (dateString: string) => {
  const date = new Date(dateString);
  const year = date.getFullYear().toString();
  const month = (date.getMonth() + 1).toString();
  const day = date.getDate().toString();
  const hours = date.getHours().toString().padStart(2, '0');
  const minutes = date.getMinutes().toString().padStart(2, '0');
  return `${year}/${month}/${day} ${hours}:${minutes}`;
};

// Watch for changes to enabled state and auto-save
watch(
  () => vacuumLogTableConfig.value.enabled,
  (newValue, oldValue) => {
    // Only auto-save if the value actually changed and we're not in initial loading
    if (oldValue !== undefined && newValue !== oldValue) {
      saveConfig(true); // Silent save
    }
  },
);

onMounted(() => {
  loadData(true);

  // Poll for new scheduled executions every 30 seconds
  const pollInterval = setInterval(() => {
    loadData();
  }, 30000);

  // Clean up interval when component is unmounted
  onUnmounted(() => {
    clearInterval(pollInterval);
  });
});

useCrowTitle(computed(() => [t('admin.settings.maintenance.maintenance'), t('admin.settings.settings')]));
</script>

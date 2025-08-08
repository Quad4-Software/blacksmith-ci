<template>
  <Settings :title="$t('admin.settings.maintenance.maintenance')" :description="$t('admin.settings.maintenance.desc')">
    <!-- Single Log Vacuum Operation -->
    <div class="space-y-6">
      <!-- Main Operation Panel -->
      <Panel>
        <div class="space-y-4">
          <!-- Operation Description -->
          <div class="flex items-center space-x-3">
            <div>
              <div class="font-medium text-wp-text-100">
                {{ $t('admin.settings.maintenance.operations.vacuum_log_table') }}
              </div>
              <div class="text-sm text-wp-text-alt-100">
                {{ $t('admin.settings.maintenance.operations.vacuum_log_table_desc') }}
              </div>
            </div>
          </div>

          <!-- Configuration Row -->
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-6 gap-4 items-center pb-2">
            <!-- Schedule -->
            <div class="flex flex-col">
              <MetadataInputField v-slot="{ id }" :label="`${$t('admin.settings.maintenance.schedule')} (UTC)`">
                <div class="flex items-center h-10">
                  <TextField
                    :id="id"
                    v-model="vacuumLogTableConfig.schedule"
                    :placeholder="$t('admin.settings.maintenance.schedule_placeholder')"
                    class="w-full"
                    :disabled="vacuumEnvVarsSet"
                  />
                </div>
              </MetadataInputField>
            </div>

            <!-- Status with Enable/Disable -->
            <div class="flex flex-col">
              <MetadataInputField :label="$t('admin.settings.maintenance.status')">
                <div class="flex items-center h-10">
                  <Checkbox
                    v-model="vacuumLogTableConfig.enabled"
                    :label="$t('admin.settings.maintenance.enabled')"
                    :disabled="vacuumEnvVarsSet"
                  />
                </div>
              </MetadataInputField>
            </div>

            <!-- Total Runs -->
            <div>
              <MetadataInputField :label="$t('admin.settings.maintenance.total_runs')">
                <div class="text-sm text-wp-text-100 flex items-center h-10">
                  {{ vacuumLogTableConfig.total_runs || 0 }}
                </div>
              </MetadataInputField>
            </div>

            <!-- Options (placeholder to match K8s layout) -->
            <div>
              <MetadataInputField :label="$t('admin.settings.maintenance.options')">
                <div class="flex items-center h-10">
                  <span class="text-sm text-wp-text-alt-100">{{
                    $t('admin.settings.maintenance.no_options') || '-'
                  }}</span>
                </div>
              </MetadataInputField>
            </div>

            <!-- Last Run -->
            <div>
              <MetadataInputField :label="$t('admin.settings.maintenance.last_run')">
                <div class="text-sm text-wp-text-100 flex items-center h-10">
                  {{ formatLastRun(vacuumLogTableConfig.last_run || '') }}
                </div>
              </MetadataInputField>
            </div>

            <!-- Actions -->
            <div>
              <MetadataInputField :label="$t('admin.settings.maintenance.actions')">
                <div class="flex items-center h-10 space-x-2">
                  <IconButton
                    icon="content-save-outline"
                    :title="
                      vacuumEnvVarsSet
                        ? $t('admin.settings.maintenance.env_vars_set_tooltip')
                        : $t('admin.settings.maintenance.save_config')
                    "
                    class="h-8 w-8"
                    :is-loading="isSaving"
                    :disabled="vacuumEnvVarsSet"
                    @click="saveConfig(false)"
                  />
                  <IconButton
                    icon="play-outline"
                    :title="$t('admin.settings.maintenance.run_now')"
                    class="h-8 w-8"
                    :is-loading="isRunning"
                    @click="runNow"
                  />
                </div>
              </MetadataInputField>
            </div>
          </div>
        </div>
      </Panel>

      <!-- Kubernetes Resource Cleanup Operation -->
      <Panel v-if="config.kubernetes_cleanup_available">
        <div class="space-y-4">
          <div class="flex items-center space-x-3">
            <div>
              <div class="font-medium text-wp-text-100">
                {{ $t('admin.settings.maintenance.operations.kubernetes_cleanup') }}
              </div>
              <div class="text-sm text-wp-text-alt-100">
                {{ $t('admin.settings.maintenance.operations.kubernetes_cleanup_desc') }}
              </div>
            </div>
          </div>

          <!-- Configuration Row -->
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-6 gap-4 items-center pb-2">
            <!-- Schedule -->
            <div class="flex flex-col">
              <MetadataInputField v-slot="{ id }" :label="`${$t('admin.settings.maintenance.schedule')} (UTC)`">
                <div class="flex items-center h-10">
                  <TextField
                    :id="id"
                    v-model="kubernetesCleanupConfig.schedule"
                    :placeholder="$t('admin.settings.maintenance.schedule_placeholder')"
                    class="w-full"
                    :disabled="kubernetesEnvVarsSet"
                  />
                </div>
              </MetadataInputField>
            </div>

            <!-- Status with Enable/Disable -->
            <div class="flex flex-col">
              <MetadataInputField :label="$t('admin.settings.maintenance.status')">
                <div class="flex items-center h-10">
                  <Checkbox
                    v-model="kubernetesCleanupConfig.enabled"
                    :label="$t('admin.settings.maintenance.enabled')"
                    :disabled="kubernetesEnvVarsSet"
                  />
                </div>
              </MetadataInputField>
            </div>

            <!-- Total Runs -->
            <div>
              <MetadataInputField :label="$t('admin.settings.maintenance.total_runs')">
                <div class="text-sm text-wp-text-100 flex items-center h-10">
                  {{ kubernetesCleanupConfig.total_runs || 0 }}
                </div>
              </MetadataInputField>
            </div>

            <!-- Options -->
            <div>
              <MetadataInputField :label="$t('admin.settings.maintenance.options')">
                <div class="flex items-center h-10">
                  <Checkbox v-model="k8sCleanupDryRun" :label="$t('admin.settings.maintenance.dry_run')" />
                </div>
              </MetadataInputField>
            </div>

            <!-- Last Run -->
            <div>
              <MetadataInputField :label="$t('admin.settings.maintenance.last_run')">
                <div class="text-sm text-wp-text-100 flex items-center h-10">
                  {{ formatLastK8sCleanup() }}
                </div>
              </MetadataInputField>
            </div>

            <!-- Actions -->
            <div>
              <MetadataInputField :label="$t('admin.settings.maintenance.actions')">
                <div class="flex items-center h-10 space-x-2">
                  <IconButton
                    icon="content-save-outline"
                    :title="
                      kubernetesEnvVarsSet
                        ? $t('admin.settings.maintenance.env_vars_set_tooltip')
                        : $t('admin.settings.maintenance.save_config')
                    "
                    class="h-8 w-8"
                    :is-loading="isSavingK8s"
                    :disabled="kubernetesEnvVarsSet"
                    @click="saveK8sConfig(false)"
                  />
                  <IconButton
                    icon="play-outline"
                    :title="
                      kubernetesCleanupConfig.enabled
                        ? $t('admin.settings.maintenance.run_kubernetes_cleanup')
                        : $t('admin.settings.maintenance.k8s_cleanup_disabled')
                    "
                    class="h-8 w-8"
                    :disabled="!kubernetesCleanupConfig.enabled"
                    :is-loading="isRunningK8sCleanup"
                    @click="runK8sCleanup"
                  />
                </div>
              </MetadataInputField>
            </div>
          </div>
        </div>
      </Panel>

      <!-- Recent Logs -->
      <Panel v-if="logs.length > 0" :title="$t('admin.settings.maintenance.recent_logs')">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-wp-background-400">
            <thead class="bg-wp-background-300">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium text-wp-text-alt-100 uppercase tracking-wider">
                  {{ $t('admin.settings.maintenance.run_time') }}
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-wp-text-alt-100 uppercase tracking-wider">
                  {{ $t('admin.settings.maintenance.name') }}
                </th>
                <th class="px-6 py-3 text-center text-xs font-medium text-wp-text-alt-100 uppercase tracking-wider">
                  {{ $t('admin.settings.maintenance.status_column') }}
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-wp-text-alt-100 uppercase tracking-wider">
                  {{ $t('admin.settings.maintenance.duration') }}
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-wp-text-alt-100 uppercase tracking-wider">
                  {{ $t('admin.settings.maintenance.error') }}
                </th>
              </tr>
            </thead>
            <tbody class="bg-wp-background-100 divide-y divide-wp-background-400">
              <tr v-for="(log, index) in logs" :key="log.id">
                <td
                  class="px-6 whitespace-nowrap text-sm text-wp-text-100"
                  :class="index === logs.length - 1 ? 'pt-2 pb-1' : 'py-2'"
                >
                  {{ formatDate(log.run_time) }}
                </td>
                <td
                  class="px-6 whitespace-nowrap text-sm text-wp-text-100"
                  :class="index === logs.length - 1 ? 'pt-2 pb-1' : 'py-2'"
                >
                  {{ getOperationName(log) }}
                </td>
                <td
                  class="px-6 whitespace-nowrap text-center"
                  :class="index === logs.length - 1 ? 'pt-2 pb-1' : 'py-2'"
                >
                  <div class="flex justify-center items-center">
                    <PipelineStatusIcon :status="log.success ? 'success' : 'error'" />
                  </div>
                </td>
                <td
                  class="px-6 whitespace-nowrap text-sm text-wp-text-100"
                  :class="index === logs.length - 1 ? 'pt-2 pb-1' : 'py-2'"
                >
                  {{ formatDuration(log.duration) }}
                </td>
                <td
                  class="px-6 whitespace-nowrap text-sm text-wp-error-100"
                  :class="index === logs.length - 1 ? 'pt-2 pb-1' : 'py-2'"
                >
                  {{ log.error || '-' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <!-- Load More Button -->
        <div v-if="hasMoreLogs" class="mt-4 flex justify-center">
          <Button class="text-sm px-3 py-1" text="Load More" :is-loading="isLoadingMore" @click="loadMoreLogs" />
        </div>
      </Panel>

      <!-- Action Log Panel -->
      <Panel title="Action Log">
        <div class="flex items-center justify-between mb-4">
          <span class="text-sm text-wp-text-alt-100">{{ $t('admin.settings.maintenance.recent_operations') }}</span>
          <Button v-if="actionLogs.length > 0" class="text-sm" text="Wipe Log" @click="clearActionLogs" />
        </div>
        <div class="bg-wp-background-200 border border-wp-background-400 rounded-lg p-4 max-h-96 overflow-y-auto">
          <div v-if="actionLogs.length === 0" class="text-center text-wp-text-alt-100 py-8">
            {{ $t('admin.settings.maintenance.no_action_logs') }}
          </div>
          <div v-else class="font-mono text-xs">
            <div v-for="(log, index) in actionLogs" :key="index" class="text-wp-text-100 mb-1">
              {{ log }}
            </div>
          </div>
        </div>
      </Panel>
    </div>
  </Settings>
</template>

<script lang="ts" setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Button from '~/components/atomic/Button.vue';
import IconButton from '~/components/atomic/IconButton.vue';
import Checkbox from '~/components/form/CheckboxMaintenance.vue';
import MetadataInputField from '~/components/form/MetadataInputField.vue';
import TextField from '~/components/form/TextField.vue';
import Panel from '~/components/layout/Panel.vue';
import Settings from '~/components/layout/Settings.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
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

const logs = ref<MaintenanceLog[]>([]);
const stats = ref<MaintenanceStats | null>(null);
const isSaving = ref(false);
const isRunning = ref(false);
const isSavingK8s = ref(false);
const isRunningK8sCleanup = ref(false);
const k8sCleanupDryRun = ref(false);
const actionLogs = ref<string[]>([]);
const isLoadingMore = ref(false);
const logsLimit = ref(10);
const hasMoreLogs = ref(true);

const vacuumEnvVarsSet = ref(false);
const kubernetesEnvVarsSet = ref(false);

// Load action logs from localStorage on startup
const loadActionLogs = () => {
  try {
    const savedLogs = localStorage.getItem('maintenance-action-logs');
    if (savedLogs) {
      actionLogs.value = JSON.parse(savedLogs);
    }
  } catch (error) {
    console.error('Failed to load action logs from localStorage:', error);
  }
};

// Save action logs to localStorage
const saveActionLogs = () => {
  try {
    localStorage.setItem('maintenance-action-logs', JSON.stringify(actionLogs.value));
  } catch (error) {
    console.error('Failed to save action logs to localStorage:', error);
  }
};

// Clear action logs
const clearActionLogs = () => {
  actionLogs.value = [];
  saveActionLogs();
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
    const [vacuumConfigData, k8sConfigData, logsData, statsData] = await Promise.all([
      apiClient.getMaintenanceConfigByAction('vacuum').catch((error) => {
        console.error('Failed to load vacuum config:', error);
        return null;
      }),
      apiClient.getMaintenanceConfigByAction('kubernetes_cleanup').catch((error) => {
        console.error('Failed to load kubernetes cleanup config:', error);
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

        actionLogs.value.unshift(
          `[${year}/${month}/${day} ${time}] [${jobTitle}] Scheduled maintenance execution detected`,
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

        // Keep only the last 50 logs to prevent localStorage from growing too large
        if (actionLogs.value.length > 50) {
          actionLogs.value = actionLogs.value.slice(0, 50);
        }
      }

      // Save updated action logs if we added any
      if (logsToProcess.length > 0) {
        saveActionLogs();
      }
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
  const jobTitle =
    jobType === 'kubernetes'
      ? t('admin.settings.maintenance.operations.kubernetes_cleanup')
      : t('admin.settings.maintenance.operations.vacuum_log_table');
  actionLogs.value.unshift(`[${year}/${month}/${day} ${time}] [${jobTitle}] ${message}`);
  // Keep only the last 50 logs to prevent localStorage from growing too large
  if (actionLogs.value.length > 50) {
    actionLogs.value = actionLogs.value.slice(0, 50);
  }
  saveActionLogs();
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

    const updatedConfig = await apiClient.updateMaintenanceConfigByAction('kubernetes_cleanup', {
      enabled: kubernetesCleanupConfig.value.enabled,
      schedule: kubernetesCleanupConfig.value.schedule,
    });

    // Update local config with server response
    kubernetesCleanupConfig.value.enabled = updatedConfig.enabled;
    kubernetesCleanupConfig.value.schedule = updatedConfig.schedule;
    kubernetesCleanupConfig.value.id = updatedConfig.id;
    kubernetesCleanupConfig.value.created = updatedConfig.created;
    kubernetesCleanupConfig.value.updated = updatedConfig.updated;

    config.value.kubernetes_cleanup_available = updatedConfig.kubernetes_cleanup_available || false;

    const changes = [];
    if (originalEnabled !== updatedConfig.enabled) {
      changes.push(`enabled: ${originalEnabled} → ${updatedConfig.enabled}`);
    }
    if (originalSchedule !== updatedConfig.schedule) {
      changes.push(`schedule: ${originalSchedule} → ${updatedConfig.schedule}`);
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
  loadActionLogs();
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

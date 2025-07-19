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
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4 items-center pb-2">
            <!-- Schedule -->
            <div>
              <MetadataInputField v-slot="{ id }" :label="$t('admin.settings.maintenance.schedule')">
                <TextField
                  :id="id"
                  v-model="vacuumLogTableConfig.schedule"
                  :placeholder="$t('admin.settings.maintenance.schedule_placeholder')"
                />
              </MetadataInputField>
            </div>

            <!-- Status with Enable/Disable -->
            <div class="flex flex-col">
              <MetadataInputField :label="$t('admin.settings.maintenance.status')">
                <div class="flex items-center h-10">
                  <Checkbox v-model="vacuumLogTableConfig.enabled" :label="$t('admin.settings.maintenance.enabled')" />
                </div>
              </MetadataInputField>
            </div>

            <!-- Total Runs -->
            <div>
              <MetadataInputField :label="$t('admin.settings.maintenance.total_runs')">
                <div class="text-sm text-wp-text-100 flex items-center h-10">{{ stats?.total_runs || 0 }}</div>
              </MetadataInputField>
            </div>

            <!-- Last Run -->
            <div>
              <MetadataInputField :label="$t('admin.settings.maintenance.last_run')">
                <div class="text-sm text-wp-text-100 flex items-center h-10">
                  {{ formatLastRun(stats?.last_run || '') }}
                </div>
              </MetadataInputField>
            </div>

            <!-- Actions -->
            <div>
              <MetadataInputField :label="$t('admin.settings.maintenance.actions')">
                <div class="flex items-center h-10">
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
                  {{ $t('admin.settings.maintenance.operations.vacuum_log_table') }}
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
import { computed, onMounted, ref, watch } from 'vue';
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
  enabled: false,
  schedule: '0 5 * * *',
  created: 0,
  updated: 0,
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

const logs = ref<MaintenanceLog[]>([]);
const stats = ref<MaintenanceStats | null>(null);
const isSaving = ref(false);
const isRunning = ref(false);
const actionLogs = ref<string[]>([]);
const isLoadingMore = ref(false);
const logsLimit = ref(10);
const hasMoreLogs = ref(true);

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

const loadData = async (reset = false) => {
  try {
    if (reset) {
      logsLimit.value = 10;
      hasMoreLogs.value = true;
    }

    // Load basic config, logs, and stats
    const [configData, logsData, statsData] = await Promise.all([
      apiClient.getMaintenanceConfig().catch((error) => {
        console.error('Failed to load maintenance config:', error);
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

    if (configData) {
      config.value = configData;
      // Use the global config for the log vacuum operation
      vacuumLogTableConfig.value.enabled = configData.enabled;
      vacuumLogTableConfig.value.schedule = configData.schedule;
      // Keep the operation-specific fields
      vacuumLogTableConfig.value.operation = 'vacuumLogTable';
      vacuumLogTableConfig.value.id = configData.id;
      vacuumLogTableConfig.value.created = configData.created;
      vacuumLogTableConfig.value.updated = configData.updated;
    } else {
      // Use defaults if config loading fails
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

    logs.value = logsData || [];
    hasMoreLogs.value = (logsData || []).length === logsLimit.value;
    stats.value = statsData;
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

const addActionLog = (message: string) => {
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
  const jobTitle = t('admin.settings.maintenance.operations.vacuum_log_table');
  actionLogs.value.unshift(`[${year}/${month}/${day} ${time}] [${jobTitle}] ${message}`);
  // Keep only the last 50 logs to prevent localStorage from growing too large
  if (actionLogs.value.length > 50) {
    actionLogs.value = actionLogs.value.slice(0, 50);
  }
  saveActionLogs();
};

const saveConfig = async (silent = false) => {
  isSaving.value = true;
  if (!silent) {
    addActionLog('Saving maintenance configuration...');
  }

  try {
    // Track what's changing
    const originalEnabled = config.value.enabled;
    const originalSchedule = config.value.schedule;

    // Update global config with current settings
    config.value.enabled = vacuumLogTableConfig.value.enabled;
    config.value.schedule = vacuumLogTableConfig.value.schedule;

    // Save global config (this should always work)
    const updatedConfig = await apiClient.updateMaintenanceConfig(config.value);

    // Update local config with server response
    config.value = updatedConfig;
    vacuumLogTableConfig.value.enabled = updatedConfig.enabled;
    vacuumLogTableConfig.value.schedule = updatedConfig.schedule;
    // Keep the operation-specific fields
    vacuumLogTableConfig.value.operation = 'vacuumLogTable';
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
    console.error('Failed to save maintenance configuration:', error);
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
        addActionLog('Check the maintenance logs below for details');

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
  loadData();
});

useCrowTitle(computed(() => [t('admin.settings.maintenance.maintenance'), t('admin.settings.settings')]));
</script>

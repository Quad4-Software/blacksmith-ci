<template>
  <!-- Filter controls -->
  <div v-if="!props.loading && props.agents" class="mb-6">
    <div class="flex flex-wrap items-center gap-2">
      <!-- Search input -->
      <div class="flex-1 min-w-[100px] max-w-[150px]">
        <TextField v-model="searchQuery" :placeholder="$t('admin.settings.agents.search')" type="text" />
      </div>

      <!-- Type filter -->
      <div class="w-[110px]">
        <SelectField
          v-model="typeFilter"
          :placeholder="$t('admin.settings.agents.filter_type')"
          :options="typeOptions"
        />
      </div>

      <!-- Platform filter -->
      <div class="w-[110px]">
        <SelectField
          v-model="platformFilter"
          :placeholder="$t('admin.settings.agents.filter_platform')"
          :options="platformOptions"
        />
      </div>

      <!-- Backend filter -->
      <div class="w-[110px]">
        <SelectField
          v-model="backendFilter"
          :placeholder="$t('admin.settings.agents.filter_backend')"
          :options="backendOptions"
        />
      </div>

      <!-- Sort dropdown with label -->
      <div class="flex items-center gap-1.5 md:ml-auto">
        <!-- Compact badge-style label -->
        <span
          class="inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium bg-wp-background-300 text-wp-text-alt-100 border border-wp-background-400 shrink-0"
        >
          {{ $t('admin.settings.agents.sort_by') }}
        </span>
        <div class="w-[130px]">
          <SelectField v-model="sortBy" :placeholder="$t('admin.settings.agents.sort_by')" :options="sortOptions" />
        </div>
      </div>

      <!-- Clear filters button -->
      <Button
        v-if="hasActiveFilters"
        :text="$t('admin.settings.agents.clear_filters')"
        color="gray"
        @click="clearFilters"
      />
    </div>
  </div>

  <!-- Agent Cards Grid -->
  <div v-if="!props.loading && props.agents" class="space-y-3">
    <div
      v-for="agent in filteredAgents"
      :key="agent.id"
      class="rounded-lg border border-wp-background-400 bg-wp-background-100 dark:bg-wp-background-200 shadow-sm hover:shadow-md transition-shadow duration-200"
    >
      <div class="p-4">
        <!-- Header Row: Name, Status Indicator, and Actions -->
        <div class="flex items-start justify-between mb-3">
          <!-- Agent Name and Status -->
          <div class="flex items-center gap-3 flex-1 min-w-0">
            <!-- Status Indicator -->
            <div
              class="shrink-0 w-3 h-3 rounded-full"
              :class="[getStatusColor(agent), getStatusAnimation(agent)]"
              :title="getStatusTitle(agent)"
            ></div>

            <!-- Agent Name -->
            <div class="min-w-0 flex-1">
              <h3 class="text-base font-semibold text-wp-text-100 truncate">
                {{ getAgentDisplayName(agent) }}
              </h3>
              <div class="flex items-center gap-2 mt-1">
                <span class="text-xs text-wp-text-alt-100"
                  >{{ $t('admin.settings.agents.agent_id') }} {{ agent.id }}</span
                >
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span v-if="agent.org_id !== -1" class="text-xs text-wp-text-alt-100" aria-hidden="true">•</span>
                <router-link
                  v-if="agent.org_id !== -1"
                  :to="`/orgs/${agent.org_id}`"
                  class="text-xs text-wp-control-info-default hover:underline"
                >
                  {{ $t('admin.settings.agents.org_prefix') }} {{ agent.org_id }}
                </router-link>
              </div>
            </div>

            <!-- Disabled Badge (if applicable) -->
            <Badge v-if="agent.no_schedule" :label="$t('disabled')" class="shrink-0" />
          </div>

          <!-- Action Buttons -->
          <div v-if="props.isAdmin || agent.org_id === props.userOrgId" class="flex items-center gap-1 shrink-0 ml-3">
            <IconButton
              icon="edit"
              :title="$t('admin.settings.agents.edit_agent')"
              class="h-8 w-8"
              @click="$emit('edit', agent)"
            />
            <IconButton
              icon="trash"
              :title="$t('admin.settings.agents.delete_agent')"
              class="h-8 w-8 hover:text-wp-error-100"
              :is-loading="props.isDeleting"
              @click="$emit('delete', agent)"
            />
          </div>
          <div v-else class="flex items-center shrink-0 ml-3">
            <Icon name="visibility-private" class="h-6 w-6 text-wp-text-alt-100" />
          </div>
        </div>

        <!-- Info Grid -->
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 mb-3">
          <!-- Version -->
          <div class="flex flex-col">
            <span class="text-xs font-medium text-wp-text-alt-100 mb-1">{{
              $t('admin.settings.agents.version.label')
            }}</span>
            <span
              v-if="agent.version"
              class="text-sm font-mono text-wp-text-100 flex items-center gap-1"
              :title="isVersionMismatch(agent.version) ? 'Version mismatch with server' : 'Version matches server'"
            >
              {{ agent.version }}
              <Icon
                v-if="isVersionMismatch(agent.version)"
                name="status-error"
                class="h-4 w-4 text-yellow-500"
                :title="$t('admin.settings.agents.version.mismatch')"
              />
            </span>
            <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
            <span v-else class="text-sm text-wp-text-alt-100" aria-label="not available">—</span>
          </div>

          <!-- Last Seen -->
          <div class="flex flex-col">
            <span class="text-xs font-medium text-wp-text-alt-100 mb-1">{{
              $t('admin.settings.agents.last_contact.label')
            }}</span>
            <span class="text-sm text-wp-text-100">
              {{ agent.last_contact ? date.timeAgo(agent.last_contact * 1000) : $t('admin.settings.agents.never') }}
            </span>
          </div>

          <!-- Workflows -->
          <div class="flex flex-col">
            <span class="text-xs font-medium text-wp-text-alt-100 mb-1">{{
              $t('admin.settings.agents.workflows.label')
            }}</span>
            <div class="flex items-center gap-2">
              <span class="text-sm text-wp-text-100">
                <span class="font-semibold">{{ agent.running_workflows ?? 0 }}</span>
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span class="text-wp-text-alt-100" aria-hidden="true"> / </span>
                <span class="text-wp-text-alt-100">{{ agent.capacity || 0 }}</span>
              </span>
              <span
                v-if="agent.running_workflows && agent.running_workflows > 0"
                class="inline-flex items-center justify-center px-2 py-0.5 text-xs font-medium rounded-full bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
                :title="`${agent.running_workflows} active workflow${agent.running_workflows > 1 ? 's' : ''}`"
              >
                {{ agent.running_workflows }} {{ $t('admin.settings.agents.workflows.active') }}
              </span>
            </div>
          </div>

          <!-- Priority -->
          <div v-if="agent.priority && agent.priority !== 0" class="flex flex-col">
            <span class="text-xs font-medium text-wp-text-alt-100 mb-1">{{
              $t('admin.settings.agents.priority_label')
            }}</span>
            <span class="text-sm text-wp-text-100 font-semibold">{{ agent.priority }}</span>
          </div>
        </div>

        <!-- Technical Details Section -->
        <div class="pt-3 border-t border-wp-background-400 space-y-2">
          <!-- System Properties -->
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-xs font-medium text-wp-text-alt-100 mr-1">{{
              $t('admin.settings.agents.system_labels')
            }}</span>
            <!-- Type Badge -->
            <Badge :label="$t('admin.settings.agents.type.badge')" :value="agent.org_id === -1 ? 'global' : 'scoped'" />

            <!-- Platform Badge -->
            <Badge v-if="agent.platform" :label="$t('admin.settings.agents.platform.badge')" :value="agent.platform" />

            <!-- Backend Badge -->
            <Badge v-if="agent.backend" :label="$t('admin.settings.agents.backend.badge')" :value="agent.backend" />
          </div>

          <!-- Custom Labels Section -->
          <div
            v-if="agent.custom_labels && Object.keys(agent.custom_labels).length > 0"
            class="flex flex-wrap items-start gap-2"
          >
            <span class="text-xs font-medium text-wp-text-alt-100 mr-1 mt-0.5">{{
              $t('admin.settings.agents.user_labels')
            }}</span>
            <div class="flex flex-wrap gap-1.5">
              <span
                v-for="(value, key) in agent.custom_labels"
                :key="`${key}-${value}`"
                class="inline-flex items-center px-2 py-1 text-xs font-medium rounded bg-wp-background-300 text-wp-text-100 border border-wp-background-400"
                :title="`${key}: ${value}`"
              >
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span class="font-semibold text-wp-text-alt-100">{{ key }}<span aria-hidden="true">:</span></span>
                <span class="ml-1">{{ value }}</span>
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Empty States -->
    <div v-if="filteredAgents.length === 0 && props.agents?.length > 0" class="text-center py-12 text-wp-text-alt-100">
      <Icon name="question" class="h-12 w-12 mx-auto mb-3 opacity-50" />
      <p class="text-base">{{ $t('admin.settings.agents.no_match') }}</p>
      <Button
        v-if="hasActiveFilters"
        :text="$t('admin.settings.agents.clear_filters')"
        color="gray"
        class="mt-4"
        @click="clearFilters"
      />
    </div>

    <div v-if="props.agents?.length === 0" class="text-center py-12 text-wp-text-alt-100">
      <Icon name="list" class="h-12 w-12 mx-auto mb-3 opacity-50" />
      <p class="text-base">{{ $t('admin.settings.agents.none') }}</p>
    </div>
  </div>

  <!-- Loading State -->
  <div v-else class="flex justify-center py-12">
    <Icon name="loading" class="animate-spin h-8 w-8" />
  </div>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Badge from '~/components/atomic/Badge.vue';
import Button from '~/components/atomic/Button.vue';
import Icon from '~/components/atomic/Icon.vue';
import IconButton from '~/components/atomic/IconButton.vue';
import SelectField from '~/components/form/SelectField.vue';
import TextField from '~/components/form/TextField.vue';
import { useDate } from '~/compositions/useDate';
import useConfig from '~/compositions/useConfig';
import type { Agent } from '~/lib/api/types';
import type { SelectOption } from '~/components/form/form.types';

const props = defineProps<{
  agents: Agent[];
  isDeleting: boolean;
  loading: boolean;
  isAdmin?: boolean;
  userOrgId?: number;
}>();

defineEmits<{
  (e: 'edit', agent: Agent): void;
  (e: 'delete', agent: Agent): void;
}>();

function getAgentDisplayName(agent: Agent): string {
  return agent.name || `Agent ${agent.id}`;
}

const date = useDate();
const { t } = useI18n();
const config = useConfig();

// Check if agent version differs from server version
function isVersionMismatch(agentVersion: string): boolean {
  const serverVersion = config.version;
  if (!serverVersion || !agentVersion) {
    return false;
  }
  return agentVersion !== serverVersion;
}

// Get status color based on agent health
function getStatusColor(agent: Agent): string {
  // Disabled agents
  if (agent.no_schedule) {
    return 'bg-wp-text-alt-100';
  }

  // Never contacted or last_contact is 0
  if (!agent.last_contact || agent.last_contact === 0) {
    return 'bg-red-500';
  }

  // Check last contact time (within 5 minutes = healthy)
  const now = Date.now() / 1000;
  const timeSinceContact = now - agent.last_contact;

  if (timeSinceContact < 300) {
    // Less than 5 minutes - healthy
    return 'bg-green-500';
  } else if (timeSinceContact < 600) {
    // 5-10 minutes - warning
    return 'bg-yellow-500';
  } else {
    // More than 10 minutes - offline
    return 'bg-red-500';
  }
}

// Get status title for tooltip
function getStatusTitle(agent: Agent): string {
  if (agent.no_schedule) {
    return 'Agent is disabled';
  }

  // Never contacted or last_contact is 0
  if (!agent.last_contact || agent.last_contact === 0) {
    return 'Agent has never connected';
  }

  const now = Date.now() / 1000;
  const timeSinceContact = now - agent.last_contact;

  if (timeSinceContact < 300) {
    return 'Agent is healthy and active';
  } else if (timeSinceContact < 600) {
    return 'Agent connection may be unstable';
  } else {
    return 'Agent is offline or unreachable';
  }
}

// Get status animation class
function getStatusAnimation(agent: Agent): string {
  // Only animate healthy agents
  if (agent.no_schedule || !agent.last_contact || agent.last_contact === 0) {
    return '';
  }

  const now = Date.now() / 1000;
  const timeSinceContact = now - agent.last_contact;

  // Only healthy agents (< 5 minutes) get the pulse animation
  if (timeSinceContact < 300) {
    return 'animate-pulse-slow';
  }

  return '';
}

// Filter state
const searchQuery = ref('');
const typeFilter = ref('');
const platformFilter = ref('');
const backendFilter = ref('');
const sortBy = ref('status');

// Compute available options from agents
const typeOptions = computed<SelectOption[]>(() => {
  const types = new Set<string>();
  props.agents?.forEach((agent) => {
    if (agent.org_id === -1) {
      types.add('global');
    } else {
      types.add('org');
    }
  });

  const options: SelectOption[] = [{ value: '', text: t('admin.settings.agents.filter_type') }];
  if (types.has('global')) {
    options.push({ value: 'global', text: 'Global' });
  }
  if (types.has('org')) {
    options.push({ value: 'org', text: 'Scoped' });
  }
  return options;
});

const platformOptions = computed<SelectOption[]>(() => {
  const platforms = new Set<string>();
  props.agents?.forEach((agent) => {
    if (agent.platform) {
      platforms.add(agent.platform);
    }
  });

  const options: SelectOption[] = [{ value: '', text: t('admin.settings.agents.filter_platform') }];
  Array.from(platforms)
    .sort()
    .forEach((platform) => {
      options.push({ value: platform, text: platform });
    });
  return options;
});

const backendOptions = computed<SelectOption[]>(() => {
  const backends = new Set<string>();
  props.agents?.forEach((agent) => {
    if (agent.backend) {
      backends.add(agent.backend);
    }
  });

  const options: SelectOption[] = [{ value: '', text: t('admin.settings.agents.filter_backend') }];
  Array.from(backends)
    .sort()
    .forEach((backend) => {
      options.push({ value: backend, text: backend });
    });
  return options;
});

// Sort options
const sortOptions = computed<SelectOption[]>(() => [
  { value: 'status', text: t('admin.settings.agents.sort.status') },
  { value: 'name', text: t('admin.settings.agents.sort.name') },
  { value: 'id', text: t('admin.settings.agents.sort.id') },
  { value: 'last_contact', text: t('admin.settings.agents.sort.last_contact') },
  { value: 'workflows', text: t('admin.settings.agents.sort.workflows') },
  { value: 'capacity', text: t('admin.settings.agents.sort.capacity') },
]);

// Check if any filters are active
const hasActiveFilters = computed(() => {
  return (
    searchQuery.value !== '' || typeFilter.value !== '' || platformFilter.value !== '' || backendFilter.value !== ''
  );
});

// Clear all filters
function clearFilters() {
  searchQuery.value = '';
  typeFilter.value = '';
  platformFilter.value = '';
  backendFilter.value = '';
}

// Helper function to get status priority for sorting
function getStatusPriority(agent: Agent): number {
  if (agent.no_schedule) return 4; // Disabled - lowest priority

  if (!agent.last_contact || agent.last_contact === 0) return 3; // Never connected

  const now = Date.now() / 1000;
  const timeSinceContact = now - agent.last_contact;

  if (timeSinceContact < 300) return 0; // Healthy - highest priority
  if (timeSinceContact < 600) return 1; // Warning
  return 2; // Offline
}

// Filtered and sorted agents
const filteredAgents = computed(() => {
  if (!props.agents) return [];

  // First, filter agents
  const filtered = props.agents.filter((agent) => {
    // Type filter
    if (typeFilter.value) {
      const isGlobal = agent.org_id === -1;
      if (typeFilter.value === 'global' && !isGlobal) return false;
      if (typeFilter.value === 'org' && isGlobal) return false;
    }

    // Platform filter
    if (platformFilter.value && agent.platform !== platformFilter.value) {
      return false;
    }

    // Backend filter
    if (backendFilter.value && agent.backend !== backendFilter.value) {
      return false;
    }

    // Search query - search in name and custom labels
    if (searchQuery.value) {
      const query = searchQuery.value.toLowerCase();
      const nameMatch = getAgentDisplayName(agent).toLowerCase().includes(query);

      // Search in custom labels
      let labelMatch = false;
      if (agent.custom_labels) {
        for (const [key, value] of Object.entries(agent.custom_labels)) {
          if (key.toLowerCase().includes(query) || value.toLowerCase().includes(query)) {
            labelMatch = true;
            break;
          }
        }
      }

      // Search in platform and backend
      const platformMatch = agent.platform?.toLowerCase().includes(query) || false;
      const backendMatch = agent.backend?.toLowerCase().includes(query) || false;

      if (!nameMatch && !labelMatch && !platformMatch && !backendMatch) {
        return false;
      }
    }

    return true;
  });

  // Then, sort agents
  return filtered.sort((a, b) => {
    switch (sortBy.value) {
      case 'status':
        return getStatusPriority(a) - getStatusPriority(b);
      case 'name':
        return getAgentDisplayName(a).localeCompare(getAgentDisplayName(b));
      case 'id':
        return a.id - b.id;
      case 'last_contact':
        return (b.last_contact || 0) - (a.last_contact || 0); // Most recent first
      case 'workflows':
        return (b.running_workflows || 0) - (a.running_workflows || 0); // Most workflows first
      case 'capacity':
        return (b.capacity || 0) - (a.capacity || 0); // Highest capacity first
      default:
        return 0;
    }
  });
});
</script>

<style scoped>
@keyframes pulse-slow {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}

.animate-pulse-slow {
  animation: pulse-slow 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}
</style>

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

      <!-- Sort dropdown with label and reverse toggle -->
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
        <!-- Reverse sort toggle -->
        <IconButton
          :icon="sortReverse ? 'sort-ascending' : 'sort-descending'"
          :title="
            sortReverse ? $t('admin.settings.agents.sort_ascending') : $t('admin.settings.agents.sort_descending')
          "
          class="shrink-0"
          @click="sortReverse = !sortReverse"
        />
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
      v-for="agent in displayedAgents"
      :key="agent.id"
      class="rounded-lg border border-wp-background-400 bg-wp-background-100 dark:bg-wp-background-200 shadow-sm hover:shadow-md transition-shadow duration-200"
    >
      <div class="p-4">
        <!-- Header Row: Name, Status Indicator, and Actions -->
        <div class="flex items-start justify-between mb-3">
          <!-- Agent Name and Status -->
          <div class="flex items-center gap-3 flex-1 min-w-0 md:flex-row">
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
              <!-- Desktop: single line metadata -->
              <div class="hidden md:flex items-center gap-2 mt-1">
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
                <!-- Docker Memory Limit -->
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span v-if="agent.backend === 'docker'" class="text-xs text-wp-text-alt-100" aria-hidden="true">•</span>
                <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                <span v-if="agent.backend === 'docker'" class="text-xs text-wp-text-alt">
                  {{ $t('admin.settings.agents.limit_mem') }}<span aria-hidden="true">:</span>
                  {{ formatBytes(agent.limit_mem) }}
                </span>
                <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                <!-- Docker CPU Quota -->
                <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                <span v-if="agent.backend === 'docker'" class="text-xs text-wp-text-alt">
                  {{ $t('admin.settings.agents.limit_cpu_quota') }}<span aria-hidden="true">:</span>
                  {{ formatCPUQuota(agent.limit_cpu_quota) }}
                </span>
                <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
              </div>
              <!-- Mobile: stacked metadata -->
              <div class="flex md:hidden flex-col gap-1 mt-1">
                <div class="flex items-center gap-2">
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
                <!-- Resource limits on separate line for mobile -->
                <div v-if="agent.backend === 'docker'" class="flex items-center gap-2">
                  <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                  <span class="text-xs text-wp-text-alt min-w-[50px]">
                    Mem/Workflow<span aria-hidden="true">:</span>
                    {{ formatBytes(agent.limit_mem) }}
                  </span>
                  <span class="text-xs text-wp-text-alt min-w-[50px]">
                    CPU/Workflow<span aria-hidden="true">:</span>
                    {{ formatCPUQuota(agent.limit_cpu_quota) }}
                  </span>
                  <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                </div>
              </div>
            </div>

            <!-- Disabled Badge (if applicable) -->
            <Badge v-if="agent.no_schedule" :label="$t('disabled')" class="shrink-0" />
          </div>

          <!-- Action Buttons (desktop only - moved to bottom on mobile) -->
          <div
            v-if="props.isAdmin || agent.org_id === props.userOrgId"
            class="hidden md:flex items-center gap-1 shrink-0 ml-3"
          >
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
          <div v-else class="hidden md:flex items-center shrink-0 ml-3">
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
                name="alert"
                class="h-4 w-4 text-yellow-500"
                :title="$t('admin.settings.agents.version.mismatch')"
              />
            </span>
            <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
            <span v-else class="text-sm text-wp-text-alt-100" aria-label="not available">—</span>
          </div>

          <!-- Last Seen (desktop: separate, mobile: combined with workflows) -->
          <div class="hidden sm:flex flex-col">
            <span class="text-xs font-medium text-wp-text-alt-100 mb-1">{{
              $t('admin.settings.agents.last_contact.label')
            }}</span>
            <span class="text-sm text-wp-text-100">
              {{ agent.last_contact ? date.timeAgo(agent.last_contact * 1000) : $t('admin.settings.agents.never') }}
            </span>
          </div>

          <!-- Workflows (desktop: separate, mobile: combined with last seen) -->
          <div class="hidden sm:flex flex-col">
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

          <!-- Mobile: Last Seen + Workflows combined in one row -->
          <div class="flex sm:hidden flex-col">
            <div class="grid grid-cols-2 gap-3 mb-1">
              <span class="text-xs font-medium text-wp-text-alt-100">{{
                $t('admin.settings.agents.last_contact.label')
              }}</span>
              <span class="text-xs font-medium text-wp-text-alt-100">{{
                $t('admin.settings.agents.workflows.label')
              }}</span>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <span class="text-sm text-wp-text-100">
                {{ agent.last_contact ? date.timeAgo(agent.last_contact * 1000) : $t('admin.settings.agents.never') }}
              </span>
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
        <div class="pt-3 border-t border-wp-background-400">
          <div class="flex flex-wrap items-center gap-2">
            <!-- Section Label -->
            <span class="text-xs font-medium text-wp-text-alt-100 mr-1">{{ $t('admin.settings.agents.labels') }}</span>

            <!-- Type Badge -->
            <Badge :label="$t('admin.settings.agents.type.badge')" :value="agent.org_id === -1 ? 'global' : 'scoped'" />

            <!-- Platform Badge -->
            <Badge v-if="agent.platform" :label="$t('admin.settings.agents.platform.badge')" :value="agent.platform" />

            <!-- Backend Badge -->
            <Badge v-if="agent.backend" :label="$t('admin.settings.agents.backend.badge')" :value="agent.backend" />

            <!-- Custom Labels (integrated inline) -->
            <template v-if="agent.custom_labels && Object.keys(agent.custom_labels).length > 0">
              <span
                v-for="(value, key) in agent.custom_labels"
                :key="`${key}-${value}`"
                class="inline-flex items-center px-2 py-1 text-xs font-medium rounded bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 border border-blue-200 dark:border-blue-800"
                :title="`Custom label: ${key} = ${value}`"
              >
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span class="font-semibold">{{ key }}<span aria-hidden="true">:</span></span>
                <span class="ml-1">{{ value }}</span>
              </span>
            </template>
          </div>
        </div>

        <!-- Action Buttons (mobile only - at bottom) -->
        <div class="flex md:hidden justify-end gap-1 pt-3 border-t border-wp-background-400 mt-3">
          <div v-if="props.isAdmin || agent.org_id === props.userOrgId" class="flex items-center gap-1">
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
          <div v-else class="flex items-center">
            <Icon name="visibility-private" class="h-6 w-6 text-wp-text-alt-100" />
          </div>
        </div>
      </div>
    </div>

    <!-- Pagination -->
    <Pagination
      v-if="displayedAgents.length > 0"
      :current-page="props.currentPage"
      :total-pages="totalPages"
      :has-more="props.hasMore"
      :is-jumping-to-last="props.isJumpingToLast"
      @update:current-page="$emit('update:current-page', $event)"
      @jump-to-last="$emit('jumpToLast')"
    />

    <!-- Empty States -->
    <div v-if="displayedAgents.length === 0 && props.agents?.length > 0" class="text-center py-12 text-wp-text-alt-100">
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
import { watchDebounced } from '@vueuse/core';

import Badge from '~/components/atomic/Badge.vue';
import Button from '~/components/atomic/Button.vue';
import Icon from '~/components/atomic/Icon.vue';
import IconButton from '~/components/atomic/IconButton.vue';
import Pagination from '~/components/atomic/Pagination.vue';
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
  agentFilters: {
    search: string;
    platform: string;
    backend: string;
    sort: string;
    order: string;
  };
  currentPage: number;
  hasMore?: boolean;
  isJumpingToLast?: boolean;
}>();

const emit = defineEmits<{
  edit: [agent: Agent];
  delete: [agent: Agent];
  filtersChanged: [];
  'update:current-page': [page: number];
  jumpToLast: [];
  updateFilters: [filters: { search?: string; platform?: string; backend?: string; sort?: string; order?: string }];
}>();

function getAgentDisplayName(agent: Agent): string {
  return agent.name || `Agent ${agent.id}`;
}

const date = useDate();
const { t } = useI18n();
const config = useConfig();

function formatBytes(bytes: number): string {
  if (bytes === 0) return t('admin.settings.agents.unlimited');
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${Math.round((bytes / k ** i) * 100) / 100} ${sizes[i]}`;
}

function formatCPUQuota(quota: number): string {
  // CPU quota is in microseconds per 100ms period
  // 100000 = 1 CPU core, 200000 = 2 CPU cores, etc.
  if (quota === 0) return t('admin.settings.agents.unlimited');
  const cores = quota / 100000;
  return cores >= 1 ? `${cores}` : `${quota}µs`;
}

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

  // Check last contact time
  const now = Date.now() / 1000;
  const timeSinceContact = now - agent.last_contact;

  if (timeSinceContact < 60) {
    // Less than 1 minute - healthy
    return 'bg-green-500';
  } else if (timeSinceContact < 300) {
    // 1-5 minutes - warning
    return 'bg-yellow-500';
  } else {
    // More than 5 minutes - offline
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

  if (timeSinceContact < 60) {
    return 'Agent is healthy and active';
  } else if (timeSinceContact < 300) {
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

  // Only healthy agents (< 1 minute) get the pulse animation
  if (timeSinceContact < 60) {
    return 'animate-pulse-slow';
  }

  return '';
}

// Local search state for immediate UI updates without losing focus
const localSearchQuery = ref(props.agentFilters.search);

// Debounced search to avoid losing focus while typing
watchDebounced(
  localSearchQuery,
  (newValue) => {
    emit('updateFilters', { search: newValue });
    emit('filtersChanged');
  },
  { debounce: 300 },
);

// Computed for template binding
const searchQuery = computed({
  get: () => localSearchQuery.value,
  set: (value: string) => {
    localSearchQuery.value = value;
  },
});

const typeFilter = ref(''); // Keep client-side only for type filter (global vs org)

const platformFilter = computed({
  get: () => props.agentFilters.platform,
  set: (value: string) => {
    emit('updateFilters', { platform: value });
    emit('filtersChanged');
  },
});

const backendFilter = computed({
  get: () => props.agentFilters.backend,
  set: (value: string) => {
    emit('updateFilters', { backend: value });
    emit('filtersChanged');
  },
});

const sortBy = computed({
  get: () => props.agentFilters.sort,
  set: (value: string) => {
    emit('updateFilters', { sort: value });
    emit('filtersChanged');
  },
});

const sortReverse = computed({
  get: () => props.agentFilters.order === 'desc',
  set: (value: boolean) => {
    emit('updateFilters', { order: value ? 'desc' : 'asc' });
    emit('filtersChanged');
  },
});

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

// Sort options - all are now server-side
const sortOptions = computed<SelectOption[]>(() => [
  { value: 'status', text: t('admin.settings.agents.sort.status') },
  { value: 'id', text: t('admin.settings.agents.sort.id') },
  { value: 'name', text: t('admin.settings.agents.sort.name') },
  { value: 'platform', text: t('admin.settings.agents.sort.platform') },
  { value: 'backend', text: t('admin.settings.agents.sort.backend') },
  { value: 'last_contact', text: t('admin.settings.agents.sort.last_contact') },
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

// Display agents - apply only client-side type filter
// All other filtering and sorting is done server-side
const displayedAgents = computed(() => {
  if (!props.agents) return [];

  // Apply only type filter client-side since it's UI-specific
  if (typeFilter.value) {
    return props.agents.filter((agent) => {
      const isGlobal = agent.org_id === -1;
      if (typeFilter.value === 'global' && !isGlobal) return false;
      if (typeFilter.value === 'org' && isGlobal) return false;
      return true;
    });
  }

  return props.agents;
});

// Calculate total pages for pagination
// Since we don't know the total count upfront, we estimate based on hasMore
// If hasMore is true, show current page + 1, otherwise current page is the last
const totalPages = computed(() => {
  if (props.hasMore) {
    return props.currentPage + 1;
  }
  return props.currentPage;
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

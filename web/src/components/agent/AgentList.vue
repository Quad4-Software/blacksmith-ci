<template>
  <!-- Filter controls -->
  <div v-if="!props.loading && props.agents" class="mb-4 space-y-2">
    <div class="flex flex-wrap gap-2">
      <!-- Search input -->
      <div class="flex-1 min-w-[200px]">
        <TextField
          v-model="searchQuery"
          :placeholder="$t('admin.settings.agents.search')"
          type="text"
          class="shadow-xs"
        />
      </div>

      <!-- Type filter -->
      <div class="min-w-[150px]">
        <SelectField
          v-model="typeFilter"
          :placeholder="$t('admin.settings.agents.filter_type')"
          :options="typeOptions"
        />
      </div>

      <!-- Platform filter -->
      <div class="min-w-[150px]">
        <SelectField
          v-model="platformFilter"
          :placeholder="$t('admin.settings.agents.filter_platform')"
          :options="platformOptions"
        />
      </div>

      <!-- Backend filter -->
      <div class="min-w-[150px]">
        <SelectField
          v-model="backendFilter"
          :placeholder="$t('admin.settings.agents.filter_backend')"
          :options="backendOptions"
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

  <div
    v-if="!props.loading && props.agents"
    class="space-y-2 text-wp-text-100 md:grid md:auto-rows-min md:grid-cols-[min-content_1fr_min-content] md:gap-x-2 md:gap-y-2 md:space-y-0"
  >
    <template v-for="agent in filteredAgents" :key="agent.id">
      <!-- Mobile: Agent Info + Actions Row / Desktop: First Row for this agent -->
      <div
        class="grid grid-cols-[1fr_auto] pb-2 gap-1 bg-wp-background-100 pt-4 dark:bg-wp-background-200 md:border-b md:border-wp-background-400 md:py-2 md:max-w-48 py-2"
      >
        <!-- Agent Name/Info -->
        <div class="flex flex-col md:block min-w-[70px] md:pl-0 pl-1 text-left">
          <span class="text-xs font-medium text-wp-text-100 truncate block">{{ getAgentDisplayName(agent) }}</span>
          <div class="text-xs text-wp-text-alt-100" title="Agent ID">{{ `ID: ${agent.id}` }}</div>
          <div class="text-xs text-wp-text-alt-100" title="Last contact">
            {{ agent.last_contact ? date.timeAgo(agent.last_contact * 1000) : $t('admin.settings.agents.never') }}
          </div>
          <div v-if="agent.capacity" class="text-xs text-wp-text-alt-100" title="Current/Max Workflows">
            {{ `Workflows: ${agent.running_workflows ?? 0}/${agent.capacity}` }}
          </div>
        </div>

        <!-- Action Buttons - Mobile only -->
        <div
          v-if="props.isAdmin || agent.org_id === props.userOrgId"
          class="flex items-center justify-start gap-1 md:hidden"
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
        <div v-else class="flex items-center justify-start md:hidden">
          <Icon name="visibility-private" class="h-6 w-6 text-wp-text-alt-100" />
        </div>
      </div>

      <!-- Desktop: Badges Column / Mobile: Badges Row -->
      <div
        class="min-w-0 flex items-center border-b border-wp-background-400 bg-wp-background-100 pb-4 md:pb-1 p-1 dark:bg-wp-background-200 md:border-b md:overflow-hidden"
      >
        <div class="flex flex-wrap items-center gap-2">
          <router-link v-if="agent.org_id !== -1" :to="`/orgs/${agent.org_id}`" class="hover:opacity-80">
            <Badge :label="$t('admin.settings.agents.org.badge')" :value="agent.org_id" />
          </router-link>
          <Badge v-if="agent.org_id !== -1" :label="$t('admin.settings.agents.type.badge')" value="scoped" />
          <Badge v-if="agent.org_id === -1" :label="$t('admin.settings.agents.type.badge')" value="global" />
          <Badge
            v-for="(value, key) in agent.custom_labels"
            :key="`${key}-${value}`"
            :label="$t('admin.settings.agents.custom_labels.badge')"
            :value="`${key}: ${value}`"
          />
          <Badge v-if="agent.platform" :label="$t('admin.settings.agents.platform.badge')" :value="agent.platform" />
          <Badge v-if="agent.backend" :label="$t('admin.settings.agents.backend.badge')" :value="agent.backend" />
          <Badge
            v-if="agent.capacity"
            :label="$t('admin.settings.agents.capacity.badge')"
            :value="`${agent.running_workflows ?? 0}/${agent.capacity}`"
          />
        </div>
      </div>

      <!-- Desktop: Action Buttons Column -->
      <div
        v-if="props.isAdmin || agent.org_id === props.userOrgId"
        class="hidden md:flex md:items-center md:justify-start md:gap-1 md:bg-wp-background-100 md:p-2 md:dark:bg-wp-background-200 md:border-b md:border-wp-background-400"
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
      <div
        v-else
        class="hidden md:flex md:items-center md:justify-end md:bg-wp-background-100 md:p-2 md:dark:bg-wp-background-200 md:border-b md:border-wp-background-400"
      >
        <Icon name="visibility-private" class="h-6 w-6 text-wp-text-alt-100" />
      </div>
    </template>
    <div v-if="filteredAgents.length === 0 && props.agents?.length > 0" class="ml-2 md:col-span-3">
      {{ $t('admin.settings.agents.no_match') }}
    </div>
    <div v-if="props.agents?.length === 0" class="ml-2 md:col-span-3">{{ $t('admin.settings.agents.none') }}</div>
  </div>

  <div v-else class="flex justify-center"><Icon name="loading" class="animate-spin" /></div>
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

// Filter state
const searchQuery = ref('');
const typeFilter = ref('');
const platformFilter = ref('');
const backendFilter = ref('');

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

// Filtered agents based on all filters
const filteredAgents = computed(() => {
  if (!props.agents) return [];

  return props.agents.filter((agent) => {
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
});
</script>

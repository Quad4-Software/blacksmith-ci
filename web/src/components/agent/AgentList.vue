<template>
  <!-- Filter controls -->
  <div v-if="props.agents" class="mb-4">
    <div class="flex flex-wrap items-center gap-2">
      <!-- Search input (left) -->
      <div class="w-[200px]">
        <SearchInput
          v-model="searchQuery"
          :placeholder="$t('admin.settings.agents.search')"
          type="text"
          class="[&_input]:h-8"
        />
      </div>

      <!-- Center spacer -->
      <div class="flex-1" />

      <!-- Platform filter (center) -->
      <DropdownMenu :modal="false">
        <DropdownMenuTrigger as-child>
          <Button variant="outline" size="sm" class="h-8">
            <Icon name="ph:hard-drives" class="mr-2 h-4 w-4" />
            <span v-if="selectedPlatforms.length === 0">{{ $t('admin.settings.agents.filter_platform') }}</span>
            <span v-else class="max-w-[120px] truncate">{{ selectedPlatforms.join(', ') }}</span>
            <template v-if="selectedPlatforms.length > 0">
              <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
              <span class="mx-2" aria-hidden="true">|</span>
              <UiBadge variant="secondary" class="rounded-sm px-1 font-normal">
                {{ selectedPlatforms.length }}
              </UiBadge>
            </template>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent class="w-[200px]" align="start">
          <DropdownMenuCheckboxItem
            v-for="option in platformOptions"
            :key="option.value"
            :model-value="selectedPlatforms.includes(option.value)"
            @update:model-value="(checked) => togglePlatform(option.value, !!checked)"
          >
            {{ option.text }}
          </DropdownMenuCheckboxItem>
          <template v-if="selectedPlatforms.length > 0">
            <DropdownMenuSeparator />
            <DropdownMenuItem @click="clearPlatforms">
              {{ $t('admin.settings.agents.clear_filter') }}
            </DropdownMenuItem>
          </template>
        </DropdownMenuContent>
      </DropdownMenu>

      <!-- Backend filter (center) -->
      <DropdownMenu :modal="false">
        <DropdownMenuTrigger as-child>
          <Button variant="outline" size="sm" class="h-8">
            <Icon name="ph:cube" class="mr-2 h-4 w-4" />
            <span v-if="selectedBackends.length === 0">{{ $t('admin.settings.agents.filter_backend') }}</span>
            <span v-else class="max-w-[120px] truncate">{{ selectedBackends.join(', ') }}</span>
            <template v-if="selectedBackends.length > 0">
              <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
              <span class="mx-2" aria-hidden="true">|</span>
              <UiBadge variant="secondary" class="rounded-sm px-1 font-normal">
                {{ selectedBackends.length }}
              </UiBadge>
            </template>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent class="w-[200px]" align="start">
          <DropdownMenuCheckboxItem
            v-for="option in backendOptions"
            :key="option.value"
            :model-value="selectedBackends.includes(option.value)"
            @update:model-value="(checked) => toggleBackend(option.value, !!checked)"
          >
            {{ option.text }}
          </DropdownMenuCheckboxItem>
          <template v-if="selectedBackends.length > 0">
            <DropdownMenuSeparator />
            <DropdownMenuItem @click="clearBackends">
              {{ $t('admin.settings.agents.clear_filter') }}
            </DropdownMenuItem>
          </template>
        </DropdownMenuContent>
      </DropdownMenu>

      <!-- Right spacer -->
      <div class="flex-1" />

      <!-- Sort dropdown (right) -->
      <DropdownMenu :modal="false">
        <DropdownMenuTrigger as-child>
          <Button variant="outline" size="sm" class="h-8">
            <Icon name="ph:arrows-down-up" class="mr-2 h-4 w-4" />
            {{ $t('sort') }}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent class="w-[200px]" align="end">
          <DropdownMenuRadioGroup :model-value="sortBy" @update:model-value="updateSort">
            <DropdownMenuRadioItem v-for="option in sortOptions" :key="option.value" :value="option.value">
              {{ option.text }}
            </DropdownMenuRadioItem>
          </DropdownMenuRadioGroup>
          <DropdownMenuSeparator />
          <DropdownMenuCheckboxItem :checked="sortReverse" @update:checked="updateSortOrder">
            {{ $t('admin.settings.agents.sort_descending') }}
          </DropdownMenuCheckboxItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  </div>

  <!-- Agent Cards Grid -->
  <div v-if="props.agents" class="relative">
    <!-- Subtle loading overlay -->
    <div
      v-if="props.loading"
      class="absolute inset-0 bg-background/30 backdrop-blur-[1px] z-10 pointer-events-none transition-opacity duration-200"
    />

    <div class="space-y-3 transition-opacity duration-200" :class="{ 'opacity-60': props.loading }">
      <Card v-for="agent in displayedAgents" :key="agent.id" class="hover:shadow-md transition-shadow duration-200">
        <div class="p-2">
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
                <h3 class="text-base font-semibold truncate">
                  <router-link
                    :to="{ name: 'admin-agent-detail', params: { id: agent.id } }"
                    class="group inline-flex items-center gap-1 text-primary hover:underline"
                    :title="$t('admin.settings.agents.view_details')"
                  >
                    {{ getAgentDisplayName(agent) }}
                    <Icon
                      name="chevron-right"
                      class="h-4 w-4 opacity-60 transition-transform group-hover:translate-x-0.5 group-hover:opacity-100"
                    />
                  </router-link>
                </h3>
                <!-- Desktop: single line metadata -->
                <div class="hidden md:flex items-center gap-2 mt-1">
                  <span class="text-xs text-muted-foreground"
                    >{{ $t('admin.settings.agents.agent_id') }} {{ agent.id }}</span
                  >
                  <template v-if="agent.org_ids?.length">
                    <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                    <span class="text-xs text-muted-foreground" aria-hidden="true">•</span>
                    <router-link
                      v-for="orgId in agent.org_ids"
                      :key="orgId"
                      :to="`/orgs/${orgId}`"
                      class="text-xs text-primary hover:underline"
                    >
                      {{ $t('admin.settings.agents.org_prefix') }} {{ orgId }}
                    </router-link>
                  </template>
                  <!-- Resources: per-workflow limit / host total -->
                  <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                  <template v-if="hasAgentResources(agent)">
                    <span class="text-xs text-muted-foreground" aria-hidden="true">•</span>
                    <span class="text-xs text-muted-foreground" :title="$t('admin.settings.agents.resource_hint')">
                      {{ $t('admin.settings.agents.resource_mem') }}<span aria-hidden="true">:</span>
                      {{ formatAgentMemory(agent, t) }}
                    </span>
                    <span class="text-xs text-muted-foreground" :title="$t('admin.settings.agents.resource_hint')">
                      {{ $t('admin.settings.agents.resource_cpu') }}<span aria-hidden="true">:</span>
                      {{ formatAgentCPU(agent, t) }}
                    </span>
                  </template>
                  <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                </div>
                <!-- Mobile: stacked metadata -->
                <div class="flex md:hidden flex-col gap-1 mt-1">
                  <div class="flex items-center gap-2">
                    <span class="text-xs text-muted-foreground"
                      >{{ $t('admin.settings.agents.agent_id') }} {{ agent.id }}</span
                    >
                    <template v-if="agent.org_ids?.length">
                      <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                      <span class="text-xs text-muted-foreground" aria-hidden="true">•</span>
                      <router-link
                        v-for="orgId in agent.org_ids"
                        :key="orgId"
                        :to="`/orgs/${orgId}`"
                        class="text-xs text-primary hover:underline"
                      >
                        {{ $t('admin.settings.agents.org_prefix') }} {{ orgId }}
                      </router-link>
                    </template>
                  </div>
                  <!-- Resources on separate line for mobile -->
                  <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                  <div v-if="hasAgentResources(agent)" class="flex items-center gap-2">
                    <span class="text-xs text-muted-foreground min-w-[50px]">
                      {{ $t('admin.settings.agents.resource_mem') }}<span aria-hidden="true">:</span>
                      {{ formatAgentMemory(agent, t) }}
                    </span>
                    <span class="text-xs text-muted-foreground min-w-[50px]">
                      {{ $t('admin.settings.agents.resource_cpu') }}<span aria-hidden="true">:</span>
                      {{ formatAgentCPU(agent, t) }}
                    </span>
                  </div>
                  <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                </div>
              </div>

              <!-- Disabled Badge (if applicable) -->
              <Badge v-if="agent.no_schedule" :label="$t('disabled')" class="shrink-0" />
            </div>

            <!-- Action Buttons (desktop only - moved to bottom on mobile) -->
            <div
              v-if="props.isAdmin || (props.userOrgId !== undefined && agent.org_ids?.includes(props.userOrgId))"
              class="hidden md:flex items-center gap-1 shrink-0 ml-3"
            >
              <Button
                variant="ghost"
                size="icon"
                :title="$t('admin.settings.agents.edit_agent')"
                @click="$emit('edit', agent)"
              >
                <Icon name="edit" class="h-4 w-4" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                :title="$t('admin.settings.agents.delete_agent')"
                :disabled="props.isDeleting"
                @click="$emit('delete', agent)"
              >
                <Icon v-if="!props.isDeleting" name="trash" class="h-4 w-4 text-destructive" />
                <Icon v-else name="loading" class="h-4 w-4 animate-spin" />
              </Button>
            </div>
            <div v-else class="hidden md:flex items-center shrink-0 ml-3">
              <Icon name="visibility-private" class="h-6 w-6 text-muted-foreground" />
            </div>
          </div>

          <!-- Info Grid -->
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 mb-3">
            <!-- Version -->
            <div class="flex flex-col">
              <span class="text-xs font-medium text-muted-foreground mb-1">{{
                $t('admin.settings.agents.version.label')
              }}</span>
              <span v-if="agent.version" class="text-sm font-mono text-foreground flex items-center gap-1">
                {{ agent.version }}
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span v-if="agent.build_date" class="text-muted-foreground">({{ agent.build_date }})</span>
                <AgentCompatibilityIcon :agent="agent" />
              </span>
              <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
              <span v-else class="text-sm text-muted-foreground" aria-label="not available">—</span>
            </div>

            <!-- Last Seen (desktop: separate, mobile: combined with workflows) -->
            <div class="hidden sm:flex flex-col">
              <span class="text-xs font-medium text-muted-foreground mb-1">{{
                $t('admin.settings.agents.last_contact.label')
              }}</span>
              <span class="text-sm text-foreground">
                {{ agent.last_contact ? date.timeAgo(agent.last_contact * 1000) : $t('admin.settings.agents.never') }}
              </span>
            </div>

            <!-- Workflows (desktop: separate, mobile: combined with last seen) -->
            <div class="hidden sm:flex flex-col">
              <span class="text-xs font-medium text-muted-foreground mb-1">{{
                $t('admin.settings.agents.workflows.label')
              }}</span>
              <div class="flex items-center gap-2">
                <span class="text-sm text-foreground">
                  <span class="font-semibold">{{ agent.running_workflows ?? 0 }}</span>
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  <span class="text-muted-foreground" aria-hidden="true"> / </span>
                  <span class="text-muted-foreground">{{ agent.capacity || 0 }}</span>
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
                <span class="text-xs font-medium text-muted-foreground">{{
                  $t('admin.settings.agents.last_contact.label')
                }}</span>
                <span class="text-xs font-medium text-muted-foreground">{{
                  $t('admin.settings.agents.workflows.label')
                }}</span>
              </div>
              <div class="grid grid-cols-2 gap-3">
                <span class="text-sm text-foreground">
                  {{ agent.last_contact ? date.timeAgo(agent.last_contact * 1000) : $t('admin.settings.agents.never') }}
                </span>
                <div class="flex items-center gap-2">
                  <span class="text-sm text-foreground">
                    <span class="font-semibold">{{ agent.running_workflows ?? 0 }}</span>
                    <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                    <span class="text-muted-foreground" aria-hidden="true"> / </span>
                    <span class="text-muted-foreground">{{ agent.capacity || 0 }}</span>
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
              <span class="text-xs font-medium text-muted-foreground mb-1">{{
                $t('admin.settings.agents.priority_label')
              }}</span>
              <span class="text-sm text-foreground font-semibold">{{ agent.priority }}</span>
            </div>
          </div>

          <!-- Technical Details Section -->
          <Separator class="my-3" />
          <div>
            <div class="flex flex-wrap items-center gap-2">
              <!-- Section Label -->
              <span class="text-xs font-medium text-muted-foreground mr-1">{{
                $t('admin.settings.agents.labels')
              }}</span>

              <!-- Type Badge - neutral secondary styling -->
              <UiBadge
                v-if="agent.owner_id === -1"
                variant="secondary"
                :title="$t('admin.settings.agents.type.system_desc')"
              >
                {{ $t('admin.settings.agents.type.system') }}
              </UiBadge>
              <UiBadge v-else-if="!agent.org_ids?.length" variant="secondary">
                {{ $t('admin.settings.agents.type.global') }}
              </UiBadge>

              <!-- Platform Badge -->
              <Badge
                v-if="agent.platform"
                :label="$t('admin.settings.agents.platform.badge')"
                :value="agent.platform"
              />

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
          <Separator class="my-3 md:hidden" />
          <div class="flex md:hidden justify-end gap-1">
            <div
              v-if="props.isAdmin || (props.userOrgId !== undefined && agent.org_ids?.includes(props.userOrgId))"
              class="flex items-center gap-1"
            >
              <Button
                variant="ghost"
                size="icon"
                :title="$t('admin.settings.agents.edit_agent')"
                @click="$emit('edit', agent)"
              >
                <Icon name="edit" class="h-4 w-4" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                :title="$t('admin.settings.agents.delete_agent')"
                :disabled="props.isDeleting"
                @click="$emit('delete', agent)"
              >
                <Icon v-if="!props.isDeleting" name="trash" class="h-4 w-4 text-destructive" />
                <Icon v-else name="loading" class="h-4 w-4 animate-spin" />
              </Button>
            </div>
            <div v-else class="flex items-center">
              <Icon name="visibility-private" class="h-6 w-6 text-muted-foreground" />
            </div>
          </div>
        </div>
      </Card>

      <!-- Empty States -->
      <div
        v-if="displayedAgents.length === 0 && props.agents?.length > 0"
        class="text-center py-12 text-muted-foreground"
      >
        <Icon name="question" class="h-12 w-12 mx-auto mb-3 opacity-50" />
        <p class="text-base">{{ $t('admin.settings.agents.no_match') }}</p>
      </div>

      <div v-if="props.agents?.length === 0" class="text-center py-12 text-muted-foreground">
        <Icon name="list" class="h-12 w-12 mx-auto mb-3 opacity-50" />
        <p class="text-base">{{ $t('admin.settings.agents.none') }}</p>
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
  </div>

  <!-- Loading State -->
  <div v-else class="space-y-3">
    <AgentCardSkeleton v-for="i in 3" :key="`skeleton-${i}`" />
  </div>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { watchDebounced } from '@vueuse/core';

import AgentCardSkeleton from '~/components/agent/AgentCardSkeleton.vue';
import AgentCompatibilityIcon from '~/components/agent/AgentCompatibilityIcon.vue';
import Badge from '~/components/atomic/Badge.vue';
import Icon from '~/components/atomic/Icon.vue';
import Pagination from '~/components/atomic/Pagination.vue';
import { Badge as UiBadge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import Card from '~/components/ui/card/Card.vue';
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import SearchInput from '~/components/ui/input/SearchInput.vue';
import { Separator } from '~/components/ui/separator';
import { useDate } from '~/compositions/useDate';
import type { Agent } from '~/lib/api/types';
import {
  formatAgentCPU,
  formatAgentMemory,
  getAgentDisplayName,
  getAgentStatusAnimation,
  getAgentStatusColor,
  getAgentStatusTitle,
  hasAgentResources,
} from '~/lib/agent-utils';
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

const date = useDate();
const { t } = useI18n();

const getStatusColor = getAgentStatusColor;
const getStatusAnimation = getAgentStatusAnimation;
const getStatusTitle = (agent: Agent) => getAgentStatusTitle(agent, t);

// Local search state for immediate UI updates without losing focus
const localSearchQuery = ref(props.agentFilters.search);

// Sync local search with prop when it changes externally (e.g., clear filters)
watch(
  () => props.agentFilters.search,
  (newValue) => {
    if (newValue !== localSearchQuery.value) {
      localSearchQuery.value = newValue;
    }
  },
);

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

// Multi-select platform filter (client-side only, not synced to parent)
const selectedPlatforms = ref<string[]>([]);

// Multi-select backend filter (client-side only, not synced to parent)
const selectedBackends = ref<string[]>([]);

function togglePlatform(value: string, checked: boolean) {
  if (checked) {
    selectedPlatforms.value = [...selectedPlatforms.value, value];
  } else {
    selectedPlatforms.value = selectedPlatforms.value.filter((p) => p !== value);
  }
}

function toggleBackend(value: string, checked: boolean) {
  if (checked) {
    selectedBackends.value = [...selectedBackends.value, value];
  } else {
    selectedBackends.value = selectedBackends.value.filter((b) => b !== value);
  }
}

function clearPlatforms() {
  selectedPlatforms.value = [];
}

function clearBackends() {
  selectedBackends.value = [];
}

const sortBy = computed(() => props.agentFilters.sort);
const sortReverse = computed(() => props.agentFilters.order === 'desc');

function updateSort(value: string | number | boolean | bigint | Record<string, any> | null) {
  if (typeof value === 'string') {
    emit('updateFilters', { sort: value });
    emit('filtersChanged');
  }
}

function updateSortOrder(descending: boolean) {
  emit('updateFilters', { order: descending ? 'desc' : 'asc' });
  emit('filtersChanged');
}

// Compute available options from agents
const platformOptions = computed<SelectOption[]>(() => {
  const platforms = new Set<string>();
  props.agents?.forEach((agent) => {
    if (agent.platform) {
      platforms.add(agent.platform);
    }
  });

  const options: SelectOption[] = [];
  [...platforms].toSorted().forEach((platform: string) => {
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

  const options: SelectOption[] = [];
  [...backends].toSorted().forEach((backend: string) => {
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

// Display agents - sorting is server-side, multi-select filters are client-side
const displayedAgents = computed(() => {
  if (!props.agents) return [];

  let filtered = props.agents;

  // Apply platform filter (client-side multi-select)
  if (selectedPlatforms.value.length > 0) {
    filtered = filtered.filter((agent) => agent.platform && selectedPlatforms.value.includes(agent.platform));
  }

  // Apply backend filter (client-side multi-select)
  if (selectedBackends.value.length > 0) {
    filtered = filtered.filter((agent) => agent.backend && selectedBackends.value.includes(agent.backend));
  }

  return filtered;
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

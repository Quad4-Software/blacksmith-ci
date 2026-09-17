<template>
  <!-- Filter controls -->
  <div v-if="!props.loading && props.autoscalers" class="mb-4">
    <div class="flex flex-wrap items-center gap-2">
      <!-- Search input (left) -->
      <div class="w-[200px]">
        <SearchInput
          v-model="searchQuery"
          :placeholder="$t('admin.settings.autoscalers.search')"
          type="text"
          class="[&_input]:h-8"
        />
      </div>

      <!-- Spacer -->
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
            {{ $t('admin.settings.autoscalers.sort_descending') }}
          </DropdownMenuCheckboxItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  </div>

  <!-- Autoscaler Cards Grid -->
  <div v-if="!props.loading && props.autoscalers" class="space-y-3">
    <div
      v-for="autoscaler in displayedAutoscalers"
      :key="autoscaler.id"
      class="rounded-lg border border-border bg-card shadow-sm hover:shadow-md transition-shadow duration-200"
    >
      <div class="p-4">
        <!-- Header Row: Name, Status Indicator, and Actions -->
        <div class="flex items-center justify-between mb-3">
          <!-- Autoscaler Name and Status -->
          <div class="flex items-center gap-3 flex-1 min-w-0">
            <!-- Status Indicator -->
            <div
              class="shrink-0 w-3 h-3 rounded-full"
              :class="[getStatusColor(autoscaler), getStatusAnimation(autoscaler)]"
              :title="getStatusTitle(autoscaler)"
            ></div>

            <!-- Autoscaler Name -->
            <div class="min-w-0 flex-1">
              <h3 class="text-base font-semibold truncate">
                <router-link
                  :to="{ name: 'admin-autoscaler-detail', params: { id: autoscaler.id } }"
                  class="text-primary hover:underline"
                >
                  {{ autoscaler.name }}
                </router-link>
              </h3>
              <div class="flex items-center gap-2 mt-1 flex-wrap">
                <span class="text-xs text-muted-foreground"
                  >{{ $t('admin.settings.autoscalers.id') }} {{ autoscaler.id }}</span
                >
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span class="text-xs text-muted-foreground">•</span>
                <span class="text-xs text-muted-foreground">
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  {{ $t('admin.settings.autoscalers.min_agents.name') }}: {{ autoscaler.min_agents }}</span
                >
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span class="text-xs text-muted-foreground">•</span>
                <span class="text-xs text-muted-foreground">
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  {{ $t('admin.settings.autoscalers.max_agents.name') }}: {{ autoscaler.max_agents }}</span
                >
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span class="text-xs text-muted-foreground">•</span>
                <span class="text-xs text-muted-foreground"
                  >{{ autoscaler.workflows_per_agent }}
                  {{ $t('admin.settings.autoscalers.workflows_per_agent_short') }}</span
                >
                <template v-if="autoscaler.provider">
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  <span class="text-xs text-muted-foreground">•</span>
                  <span class="text-xs text-muted-foreground">{{ autoscaler.provider }}</span>
                </template>
                <template v-if="autoscaler.region">
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  <span class="text-xs text-muted-foreground">•</span>
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  <span class="text-xs text-muted-foreground">Region: {{ autoscaler.region }}</span>
                </template>
                <template v-if="autoscaler.instance_type">
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  <span class="text-xs text-muted-foreground">•</span>
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  <span class="text-xs text-muted-foreground">Type: {{ autoscaler.instance_type }}</span>
                </template>
              </div>
            </div>

            <!-- Disabled Badge (if applicable) -->
            <Badge v-if="!autoscaler.enabled" :label="$t('disabled')" class="shrink-0" />
          </div>

          <!-- Action Buttons -->
          <div class="hidden md:flex items-center gap-1 shrink-0 ml-3">
            <IconButton
              :icon="autoscaler.enabled ? 'turn-off' : 'play'"
              :title="
                autoscaler.enabled ? $t('admin.settings.autoscalers.disable') : $t('admin.settings.autoscalers.enable')
              "
              class="h-8 w-8"
              @click="$emit('toggleEnabled', autoscaler)"
            />
            <IconButton
              icon="trash"
              :title="$t('admin.settings.autoscalers.delete_autoscaler')"
              class="h-8 w-8 hover:text-destructive"
              :is-loading="props.isDeleting"
              @click="$emit('delete', autoscaler)"
            />
          </div>
        </div>

        <!-- Info Grid -->
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 mb-3">
          <!-- Version -->
          <div class="flex flex-col">
            <span class="text-xs font-medium text-muted-foreground mb-1">{{
              $t('admin.settings.autoscalers.version.label')
            }}</span>
            <span v-if="autoscaler.version" class="text-sm font-mono text-foreground">
              {{ autoscaler.version }}
            </span>
            <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
            <span v-else class="text-sm text-muted-foreground" aria-label="not available">—</span>
          </div>

          <!-- Last Seen (desktop: separate, mobile: combined with agents) -->
          <div class="hidden sm:flex flex-col">
            <span class="text-xs font-medium text-muted-foreground mb-1">{{
              $t('admin.settings.autoscalers.last_contact.label')
            }}</span>
            <span class="text-sm text-foreground">
              {{
                autoscaler.last_contact
                  ? date.timeAgo(autoscaler.last_contact * 1000)
                  : $t('admin.settings.autoscalers.never')
              }}
            </span>
          </div>

          <!-- Agents Status (desktop: separate, mobile: combined with last seen) -->
          <div class="hidden sm:flex flex-col">
            <span class="text-xs font-medium text-muted-foreground mb-1">{{
              $t('admin.settings.autoscalers.agents.label')
            }}</span>
            <div class="flex items-center gap-2">
              <span class="text-sm text-foreground">
                <span class="font-semibold">{{ autoscaler.active_agents || 0 }}</span>
                <span v-if="autoscaler.max_agents" class="text-muted-foreground">
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  <span aria-hidden="true"> / </span>
                  {{ autoscaler.max_agents }}
                </span>
              </span>
              <span
                v-if="autoscaler.pending_agents && autoscaler.pending_agents > 0"
                class="inline-flex items-center justify-center px-2 py-0.5 text-xs font-medium rounded-full"
                :class="
                  isPendingStale(autoscaler)
                    ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                    : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
                "
                :title="
                  isPendingStale(autoscaler)
                    ? `${autoscaler.pending_agents} agent${autoscaler.pending_agents > 1 ? 's' : ''} stuck — provisioning has not completed`
                    : `${autoscaler.pending_agents} pending agent${autoscaler.pending_agents > 1 ? 's' : ''}`
                "
              >
                {{ autoscaler.pending_agents }}
                {{
                  isPendingStale(autoscaler)
                    ? $t('admin.settings.autoscalers.agents.pending_stale')
                    : $t('admin.settings.autoscalers.agents.pending')
                }}
              </span>
            </div>
          </div>

          <!-- Mobile: Last Seen + Agents combined in one row -->
          <div class="flex sm:hidden flex-col">
            <div class="grid grid-cols-2 gap-3 mb-1">
              <span class="text-xs font-medium text-muted-foreground">{{
                $t('admin.settings.autoscalers.last_contact.label')
              }}</span>
              <span class="text-xs font-medium text-muted-foreground">{{
                $t('admin.settings.autoscalers.agents.label')
              }}</span>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <span class="text-sm text-foreground">
                {{
                  autoscaler.last_contact
                    ? date.timeAgo(autoscaler.last_contact * 1000)
                    : $t('admin.settings.autoscalers.never')
                }}
              </span>
              <div class="flex items-center gap-2">
                <span class="text-sm text-foreground">
                  <span class="font-semibold">{{ autoscaler.active_agents || 0 }}</span>
                  <span v-if="autoscaler.max_agents" class="text-muted-foreground">
                    <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                    <span aria-hidden="true"> / </span>
                    {{ autoscaler.max_agents }}
                  </span>
                </span>
                <span
                  v-if="autoscaler.pending_agents && autoscaler.pending_agents > 0"
                  class="inline-flex items-center justify-center px-2 py-0.5 text-xs font-medium rounded-full"
                  :class="
                    isPendingStale(autoscaler)
                      ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                      : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
                  "
                  :title="
                    isPendingStale(autoscaler)
                      ? `${autoscaler.pending_agents} agent${autoscaler.pending_agents > 1 ? 's' : ''} stuck — provisioning has not completed`
                      : `${autoscaler.pending_agents} pending agent${autoscaler.pending_agents > 1 ? 's' : ''}`
                  "
                >
                  {{ autoscaler.pending_agents }}
                  {{
                    isPendingStale(autoscaler)
                      ? $t('admin.settings.autoscalers.agents.pending_stale')
                      : $t('admin.settings.autoscalers.agents.pending')
                  }}
                </span>
              </div>
            </div>
          </div>
        </div>

        <!-- Technical Details Section (if configured) -->
        <div v-if="hasMetadata(autoscaler)" class="pt-3 border-t border-border">
          <div class="flex flex-wrap items-center gap-2">
            <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
            <span class="text-xs font-medium text-muted-foreground mr-1">Details:</span>

            <!-- Provider Badge -->
            <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
            <Badge v-if="autoscaler.provider" label="provider" :value="autoscaler.provider" />

            <!-- CPU Limit -->
            <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
            <Badge v-if="autoscaler.limit_cpu_quota" label="CPU" :value="autoscaler.limit_cpu_quota" />

            <!-- Memory Limit -->
            <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
            <Badge v-if="autoscaler.limit_mem" label="Memory" :value="autoscaler.limit_mem" />

            <!-- Agent Labels -->
            <template v-if="autoscaler.agent_labels && Object.keys(autoscaler.agent_labels).length > 0">
              <span
                v-for="(value, key) in autoscaler.agent_labels"
                :key="`${key}-${value}`"
                class="inline-flex items-center px-2 py-1 text-xs font-medium rounded bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 border border-blue-200 dark:border-blue-800"
                :title="`Agent label: ${key} = ${value}`"
              >
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span class="font-semibold">{{ key }}<span aria-hidden="true">:</span></span>
                <span class="ml-1">{{ value }}</span>
              </span>
            </template>
          </div>
        </div>

        <!-- Action Buttons (mobile only) -->
        <div class="flex md:hidden justify-end gap-1 pt-3 border-t border-border mt-3">
          <IconButton
            :icon="autoscaler.enabled ? 'turn-off' : 'play'"
            :title="
              autoscaler.enabled ? $t('admin.settings.autoscalers.disable') : $t('admin.settings.autoscalers.enable')
            "
            class="h-8 w-8"
            @click="$emit('toggleEnabled', autoscaler)"
          />
          <IconButton
            icon="trash"
            :title="$t('admin.settings.autoscalers.delete_autoscaler')"
            class="h-8 w-8 hover:text-destructive"
            :is-loading="props.isDeleting"
            @click="$emit('delete', autoscaler)"
          />
        </div>
      </div>
    </div>

    <!-- Pagination -->
    <Pagination
      v-if="displayedAutoscalers.length > 0"
      :current-page="props.currentPage"
      :total-pages="totalPages"
      :has-more="props.hasMore"
      :is-jumping-to-last="props.isJumpingToLast"
      @update:current-page="$emit('update:current-page', $event)"
      @jump-to-last="$emit('jumpToLast')"
    />

    <!-- Empty States -->
    <div
      v-if="displayedAutoscalers.length === 0 && props.autoscalers?.length > 0"
      class="text-center py-12 text-muted-foreground"
    >
      <Icon name="question" class="h-12 w-12 mx-auto mb-3 opacity-50" />
      <p class="text-base">{{ $t('admin.settings.autoscalers.no_match') }}</p>
      <Button v-if="hasActiveFilters" variant="outline" class="mt-4" @click="clearFilters">
        {{ $t('admin.settings.autoscalers.clear_filters') }}
      </Button>
    </div>

    <div v-if="props.autoscalers?.length === 0" class="text-center py-12 text-muted-foreground">
      <Icon name="list" class="h-12 w-12 mx-auto mb-3 opacity-50" />
      <p class="text-base">
        {{ $t('admin.settings.autoscalers.none') }}
        <a
          href="https://crowci.dev/dev/configuration/autoscaler/"
          target="_blank"
          rel="noopener noreferrer"
          class="text-primary hover:text-primary/80 underline"
        >
          {{ $t('admin.settings.autoscalers.none_learn_more') }}
        </a>
      </p>
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
import Icon from '~/components/atomic/Icon.vue';
import IconButton from '~/components/atomic/IconButton.vue';
import Pagination from '~/components/atomic/Pagination.vue';
import { Button } from '~/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import SearchInput from '~/components/ui/input/SearchInput.vue';
import { useDate } from '~/compositions/useDate';

import type { Autoscaler } from '~/lib/api/types';
import type { SelectOption } from '~/components/form/form.types';

const props = defineProps<{
  autoscalers: Autoscaler[];
  isDeleting: boolean;
  loading: boolean;
  autoscalerFilters: {
    search: string;
    enabled?: string;
    sort: string;
    order: string;
  };
  currentPage: number;
  hasMore?: boolean;
  isJumpingToLast?: boolean;
}>();

const emit = defineEmits<{
  toggleEnabled: [autoscaler: Autoscaler];
  delete: [autoscaler: Autoscaler];
  filtersChanged: [];
  'update:current-page': [page: number];
  jumpToLast: [];
  updateFilters: [filters: { search?: string; enabled?: string; sort?: string; order?: string }];
}>();

const date = useDate();
const { t } = useI18n();

function hasMetadata(autoscaler: Autoscaler): boolean {
  return !!(
    autoscaler.provider ||
    autoscaler.instance_type ||
    autoscaler.region ||
    autoscaler.min_agents !== undefined ||
    autoscaler.max_agents ||
    autoscaler.workflows_per_agent ||
    autoscaler.limit_cpu_quota ||
    autoscaler.limit_mem ||
    (autoscaler.agent_labels && Object.keys(autoscaler.agent_labels).length > 0)
  );
}

const PROVISION_TIMEOUT_SECONDS = 300; // 5 minutes

// Check if pending agents have been pending for longer than the timeout
function isPendingStale(autoscaler: Autoscaler): boolean {
  if (!autoscaler.pending_agents || autoscaler.pending_agents <= 0 || !autoscaler.pending_since) {
    return false;
  }
  const now = Date.now() / 1000;
  return now - autoscaler.pending_since >= PROVISION_TIMEOUT_SECONDS;
}

// Get status color based on autoscaler health
function getStatusColor(autoscaler: Autoscaler): string {
  if (!autoscaler.enabled) {
    return 'bg-muted-foreground';
  }

  if (!autoscaler.last_contact || autoscaler.last_contact === 0) {
    return 'bg-red-500';
  }

  const now = Date.now() / 1000;
  const timeSinceContact = now - autoscaler.last_contact;

  if (timeSinceContact < 120) {
    return 'bg-green-500';
  } else if (timeSinceContact < 300) {
    return 'bg-yellow-500';
  } else {
    return 'bg-red-500';
  }
}

function getStatusTitle(autoscaler: Autoscaler): string {
  if (!autoscaler.enabled) {
    return 'Autoscaler is disabled';
  }

  if (!autoscaler.last_contact || autoscaler.last_contact === 0) {
    return 'Autoscaler has never connected';
  }

  const now = Date.now() / 1000;
  const timeSinceContact = now - autoscaler.last_contact;

  if (timeSinceContact < 120) {
    return 'Autoscaler is healthy and active';
  } else if (timeSinceContact < 300) {
    return 'Autoscaler connection may be unstable';
  } else {
    return 'Autoscaler is offline or unreachable';
  }
}

function getStatusAnimation(autoscaler: Autoscaler): string {
  if (!autoscaler.enabled || !autoscaler.last_contact || autoscaler.last_contact === 0) {
    return '';
  }

  const now = Date.now() / 1000;
  const timeSinceContact = now - autoscaler.last_contact;

  if (timeSinceContact < 120) {
    return 'animate-pulse-slow';
  }

  return '';
}

const localSearchQuery = ref(props.autoscalerFilters.search);

watchDebounced(
  localSearchQuery,
  (newValue) => {
    emit('updateFilters', { search: newValue });
    emit('filtersChanged');
  },
  { debounce: 300 },
);

const searchQuery = computed({
  get: () => localSearchQuery.value,
  set: (value: string) => {
    localSearchQuery.value = value;
  },
});

const sortBy = computed(() => props.autoscalerFilters.sort);
const sortReverse = computed(() => props.autoscalerFilters.order === 'desc');

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

const sortOptions = computed<SelectOption[]>(() => [
  { value: 'status', text: t('admin.settings.autoscalers.sort.status') },
  { value: 'id', text: t('admin.settings.autoscalers.sort.id') },
  { value: 'name', text: t('admin.settings.autoscalers.sort.name') },
  { value: 'last_contact', text: t('admin.settings.autoscalers.sort.last_contact') },
]);

const hasActiveFilters = computed(() => {
  return searchQuery.value !== '';
});

function clearFilters() {
  searchQuery.value = '';
}

const displayedAutoscalers = computed(() => {
  if (!props.autoscalers) return [];
  return props.autoscalers;
});

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

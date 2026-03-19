<template>
  <Settings :title="$t('metrics.title')" :description="$t('metrics.desc')">
    <template #headerActions>
      <div class="flex items-center gap-2 flex-wrap">
        <!-- Owner filter -->
        <DropdownMenu v-if="!showAllInstance && uniqueOwners.length > 1" :modal="false">
          <DropdownMenuTrigger as-child>
            <Button variant="outline" size="sm" class="h-9 border-dashed">
              <Icon name="user" class="mr-2 h-4 w-4" />
              {{ $t('owner') }}
              <template v-if="selectedOwners.length > 0">
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span class="mx-2" aria-hidden="true">|</span>
                <BadgeUI variant="secondary" class="rounded-sm px-1 font-normal">
                  {{ selectedOwners.length }}
                </BadgeUI>
              </template>
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent class="w-[200px]" align="start">
            <div class="max-h-[300px] overflow-y-auto">
              <DropdownMenuCheckboxItem
                v-for="owner in uniqueOwners"
                :key="owner"
                :model-value="selectedOwners.includes(owner)"
                @update:model-value="(value) => toggleOwnerFilter(owner, !!value)"
              >
                {{ owner }}
              </DropdownMenuCheckboxItem>
            </div>
            <template v-if="selectedOwners.length > 0">
              <DropdownMenuSeparator />
              <DropdownMenuItem @click="clearOwnerFilters">
                {{ $t('clear_filters') }}
              </DropdownMenuItem>
            </template>
          </DropdownMenuContent>
        </DropdownMenu>

        <!-- Repo filter -->
        <RepoFilter
          v-if="!showAllInstance && filteredRepos.length > 0"
          :repos="filteredRepos"
          :selected-repo-ids="selectedRepoIds"
          @update:selected-repo-ids="updateSelectedRepos"
        />

        <!-- Date range filter -->
        <DateRangeFilter v-model="dateRange" />

        <!-- Admin scope toggle -->
        <div v-if="user?.admin" class="flex items-center gap-2 border-l pl-2 ml-1">
          <Switch id="admin-scope" :checked="showAllInstance" @update:checked="toggleAdminScope" />
          <label for="admin-scope" class="text-sm cursor-pointer whitespace-nowrap">
            {{ $t('metrics.filter.show_all_instance') }}
          </label>
        </div>
      </div>
    </template>

    <div v-if="loading" class="flex justify-center py-8">
      <Icon name="spinner" class="h-8 w-8 animate-spin" />
    </div>

    <Tabs v-else-if="metrics" v-model="activeTab" class="w-full">
      <TabsList>
        <TabsTrigger value="overview">
          {{ $t('metrics.tabs.overview') }}
        </TabsTrigger>
        <TabsTrigger value="charts">
          {{ $t('metrics.tabs.charts') }}
        </TabsTrigger>
      </TabsList>

      <!-- Overview Tab Content -->
      <TabsContent value="overview" class="space-y-4">
        <!-- Overview Cards -->
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          <Card>
            <CardContent>
              <div class="text-sm text-muted-foreground">{{ $t('metrics.total_repos') }}</div>
              <div class="text-3xl font-bold text-foreground">{{ metrics.total_repos || 0 }}</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent>
              <div class="text-sm text-muted-foreground">{{ $t('metrics.total_orgs') }}</div>
              <div class="text-3xl font-bold text-foreground">{{ metrics.total_orgs || 0 }}</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent>
              <div class="text-sm text-muted-foreground">{{ $t('metrics.total_pipelines') }}</div>
              <div class="text-3xl font-bold text-foreground">{{ metrics.total_pipelines || 0 }}</div>
            </CardContent>
          </Card>
        </div>

        <!-- Average Metrics -->
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <Card>
            <CardContent>
              <div class="text-sm text-muted-foreground">{{ $t('metrics.avg_build_time') }}</div>
              <div class="text-3xl font-bold text-foreground">{{ formatDuration(metrics.avg_build_time || 0) }}</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent>
              <div class="text-sm text-muted-foreground">{{ $t('metrics.avg_steps_per_workflow') }}</div>
              <div class="text-3xl font-bold text-foreground">{{ (metrics.avg_steps_per_flow || 0).toFixed(1) }}</div>
            </CardContent>
          </Card>
        </div>

        <!-- Top Agents by Workflows -->
        <Card v-if="topAgents.length > 0">
          <CardHeader>
            <CardTitle>{{ $t('metrics.top_agents') }}</CardTitle>
          </CardHeader>
          <CardContent>
            <div class="space-y-2">
              <div v-for="agent in topAgents" :key="agent.agent_id" class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <Icon name="agent" class="h-5 w-5 text-muted-foreground" />
                  <span class="font-medium">{{ agent.agent_name }}</span>
                  <Badge v-if="agent.platform" :label="$t('metrics.platform')" :value="agent.platform" />
                  <Badge v-if="agent.backend" :label="$t('metrics.backend')" :value="agent.backend" />
                </div>
                <span class="text-muted-foreground">{{ agent.workflow_count }} {{ $t('metrics.workflows') }}</span>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- Top Repos by Pipelines -->
        <Card v-if="topRepos.length > 0">
          <CardHeader>
            <CardTitle>{{ $t('metrics.top_repos') }}</CardTitle>
          </CardHeader>
          <CardContent>
            <div class="space-y-3">
              <div v-for="repo in topRepos" :key="repo.repo_id" class="flex flex-col gap-1">
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <Icon name="repo" class="h-5 w-5 text-muted-foreground" />
                    <span class="font-medium">{{ repo.repo_full_name }}</span>
                  </div>
                  <span class="text-muted-foreground">{{ repo.pipeline_count }} {{ $t('metrics.pipelines') }}</span>
                </div>
                <div class="ml-7 flex gap-4 text-sm text-muted-foreground">
                  <span>{{ $t('metrics.avg_time') }} {{ formatDuration(repo.avg_build_time) }}</span>
                  <span>{{ $t('metrics.total_time') }} {{ formatDuration(repo.total_build_time) }}</span>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- Top Orgs by Pipelines -->
        <Card v-if="topOrgs.length > 0">
          <CardHeader>
            <CardTitle>{{ $t('metrics.top_orgs') }}</CardTitle>
          </CardHeader>
          <CardContent>
            <div class="space-y-3">
              <div v-for="org in topOrgs" :key="org.org_id" class="flex flex-col gap-1">
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <Icon name="org" class="h-5 w-5 text-muted-foreground" />
                    <span class="font-medium">{{ org.org_name }}</span>
                    <Badge :label="$t('metrics.repos')" :value="org.repo_count.toString()" />
                  </div>
                  <span class="text-muted-foreground">{{ org.pipeline_count }} {{ $t('metrics.pipelines') }}</span>
                </div>
                <div class="ml-7 flex gap-4 text-sm text-muted-foreground">
                  <span>{{ $t('metrics.avg_time') }} {{ formatDuration(org.avg_build_time) }}</span>
                  <span>{{ $t('metrics.total_time') }} {{ formatDuration(org.total_build_time) }}</span>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </TabsContent>

      <!-- Charts Tab Content -->
      <TabsContent value="charts">
        <AdminMetricsCharts :metrics="metrics" />
      </TabsContent>
    </Tabs>
  </Settings>
</template>

<script lang="ts" setup>
import { useStorage } from '@vueuse/core';
import { computed, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';

import AdminMetricsCharts from '~/components/admin/AdminMetricsCharts.vue';
import Badge from '~/components/atomic/Badge.vue';
import Icon from '~/components/atomic/Icon.vue';
import Settings from '~/components/layout/Settings.vue';
import DateRangeFilter from '~/components/metrics/DateRangeFilter.vue';
import RepoFilter from '~/components/metrics/RepoFilter.vue';
import { Badge as BadgeUI } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '~/components/ui/card';
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import { Switch } from '~/components/ui/switch';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '~/components/ui/tabs';
import useApiClient from '~/compositions/useApiClient';
import useAuthentication from '~/compositions/useAuthentication';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { Metrics, Repo } from '~/lib/api/types';

const apiClient = useApiClient();
const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const { user } = useAuthentication();

const metrics = ref<Metrics>();
const loading = ref(false);
const userRepos = ref<Repo[]>([]);

// Persisted filter state
const showAllInstance = useStorage('crow:metrics-show-all', false);
const selectedRepoIds = useStorage<number[]>('crow:metrics-selected-repos', []);
const selectedOwners = useStorage<string[]>('crow:metrics-selected-owners', []);
const dateRange = ref<{ after?: number; before?: number }>({});

// Get unique owners from user repos
const uniqueOwners = computed(() => {
  const owners = new Set(userRepos.value.map((r) => r.owner));
  return [...owners].toSorted();
});

// Filter repos by selected owners
const filteredRepos = computed(() => {
  if (selectedOwners.value.length === 0) {
    return userRepos.value;
  }
  return userRepos.value.filter((r) => selectedOwners.value.includes(r.owner));
});

function toggleOwnerFilter(owner: string, selected: boolean) {
  if (selected) {
    selectedOwners.value = [...selectedOwners.value, owner];
  } else {
    selectedOwners.value = selectedOwners.value.filter((o) => o !== owner);
  }
  // Clear repo selection if none of the selected repos belong to the remaining owners
  if (selectedOwners.value.length > 0) {
    const filteredRepoIds = filteredRepos.value.map((r) => r.id);
    selectedRepoIds.value = selectedRepoIds.value.filter((id) => filteredRepoIds.includes(id));
  }
  loadMetrics();
}

function clearOwnerFilters() {
  selectedOwners.value = [];
  loadMetrics();
}

const activeTab = computed({
  get: () => (route.query.tab as string) || 'overview',
  set: (value: string) => {
    router.push({ query: { ...route.query, tab: value } });
  },
});

const topAgents = computed(() => {
  if (!metrics.value?.agent_details || !Array.isArray(metrics.value.agent_details)) return [];
  return metrics.value.agent_details.toSorted((a, b) => (b.workflow_count || 0) - (a.workflow_count || 0)).slice(0, 10);
});

const topRepos = computed(() => {
  if (!metrics.value?.repo_details || !Array.isArray(metrics.value.repo_details)) return [];
  return metrics.value.repo_details.toSorted((a, b) => (b.pipeline_count || 0) - (a.pipeline_count || 0)).slice(0, 10);
});

const topOrgs = computed(() => {
  if (!metrics.value?.org_details || !Array.isArray(metrics.value.org_details)) return [];
  return metrics.value.org_details.toSorted((a, b) => (b.pipeline_count || 0) - (a.pipeline_count || 0)).slice(0, 10);
});

function formatDuration(seconds: number): string {
  if (!seconds || seconds === 0) return '0s';

  const absSecs = Math.abs(seconds);

  // For very large durations, use days
  if (absSecs >= 86400) {
    const days = Math.floor(absSecs / 86400);
    const hours = Math.floor((absSecs % 86400) / 3600);
    if (hours > 0) {
      return `${days}d ${hours}h`;
    }
    return `${days}d`;
  }

  // For medium durations, use hours and minutes
  if (absSecs >= 3600) {
    const hours = Math.floor(absSecs / 3600);
    const minutes = Math.floor((absSecs % 3600) / 60);
    if (minutes > 0) {
      return `${hours}h ${minutes}m`;
    }
    return `${hours}h`;
  }

  // For small durations, use minutes and seconds
  if (absSecs >= 60) {
    const minutes = Math.floor(absSecs / 60);
    const secs = Math.floor(absSecs % 60);
    if (secs > 0) {
      return `${minutes}m ${secs}s`;
    }
    return `${minutes}m`;
  }

  // For very small durations, just show seconds
  return `${Math.floor(absSecs)}s`;
}

function toggleAdminScope() {
  showAllInstance.value = !showAllInstance.value;
  loadMetrics();
}

function updateSelectedRepos(repoIds: number[]) {
  selectedRepoIds.value = repoIds;
  loadMetrics();
}

async function loadUserRepos() {
  try {
    const repos = await apiClient.getRepoList();
    // Filter to only active repos
    userRepos.value = repos.filter((r) => r.active);
  } catch {
    userRepos.value = [];
  }
}

async function loadMetrics() {
  loading.value = true;
  try {
    const filter: { after?: number; before?: number; all?: boolean; repos?: string } = {
      ...dateRange.value,
    };

    if (showAllInstance.value && user?.admin) {
      filter.all = true;
    } else if (selectedRepoIds.value.length > 0) {
      filter.repos = selectedRepoIds.value.join(',');
    }

    metrics.value = await apiClient.getMetrics(filter);
  } finally {
    loading.value = false;
  }
}

// Watch for date range changes
watch(dateRange, () => {
  loadMetrics();
});

onMounted(async () => {
  await loadUserRepos();
  await loadMetrics();
});

useCrowTitle(computed(() => [t('metrics.title')]));
</script>

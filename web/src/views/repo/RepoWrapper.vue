<!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
<template>
  <Scaffold v-if="repo && repoPermissions && route.meta.repoHeader" enable-tabs fluid-content>
    <template #title>
      <span class="flex min-w-0 whitespace-nowrap overflow-hidden text-ellipsis items-center gap-1">
        <router-link
          v-if="isAuthenticated"
          :to="{ name: 'org', params: { orgId: repo.org_id } }"
          class="hover:underline hover:decoration-primary shrink-0"
          >{{ repo.owner }}</router-link
        >
        <span v-else class="shrink-0">{{ repo.owner }}</span>
        <span class="shrink-0">&nbsp;/&nbsp;</span>
        <span class="truncate">{{ repo.name }}</span>
        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger as-child>
              <Button variant="ghost" size="icon-sm" class="h-6 w-6 shrink-0" @click.stop="copyRepoSlug">
                <Icon :name="copiedRepoSlug ? 'lucide:check' : 'lucide:copy'" class="h-3 w-3" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              <p>{{ copiedRepoSlug ? $t('copied') : $t('copy') }}</p>
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </span>
    </template>
    <template #headerActions>
      <IconButton :href="repo.forge_url" :title="$t('repo.open_in_forge')" :icon="forgeIcon" class="forge h-8 w-8" />
      <IconButton
        v-if="repoPermissions.admin"
        :to="{ name: 'repo-settings-secrets' }"
        :title="$t('secrets.secrets')"
        icon="secret"
      />
      <IconButton
        v-if="repoPermissions.admin"
        :to="{ name: 'repo-settings' }"
        :title="$t('settings')"
        icon="settings"
      />
    </template>
    <template #tabActions>
      <div class="flex flex-col gap-2 w-full pb-2 md:flex-row md:items-center md:justify-between">
        <!-- Action buttons -->
        <div class="flex items-center gap-2">
          <Button
            v-if="repoPermissions.push && (route.name === 'repo' || route.name === 'repo-pipeline')"
            variant="outline"
            size="icon-sm"
            class="h-8 w-8 shrink-0"
            :title="isMultiselectMode ? $t('repo.pipeline.exit_multiselect') : $t('repo.pipeline.multiselect')"
            @click="toggleMultiselectMode"
          >
            <Icon :name="isMultiselectMode ? 'close' : 'list'" class="h-4 w-4" />
          </Button>
          <Button v-if="repoPermissions.push" size="sm" class="h-8" @click="showManualPipelinePopup = true">
            <Icon name="play" class="mr-1 h-4 w-4" />
            {{ $t('repo.manual_pipeline.trigger') }}
          </Button>
        </div>

        <!-- Search (drops below the filters on mobile, sits between actions and filters on desktop) -->
        <SearchInput
          v-model="searchQuery"
          :placeholder="$t('search')"
          class="order-last mt-2 w-full md:order-none md:mt-0 md:w-64 lg:w-80"
        />

        <!-- Filters -->
        <div class="flex items-center gap-2 flex-wrap">
          <!-- Event type filter (multi-select) -->
          <DropdownMenu :modal="false">
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" class="h-8 border-dashed">
                <Icon name="filter" class="mr-2 h-4 w-4" />
                {{ $t('repo.pipeline.filter.event') }}
                <template v-if="selectedEvents.length > 0">
                  <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                  <span class="mx-2" aria-hidden="true">|</span>
                  <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                  <Badge variant="secondary" class="rounded-sm px-1 font-normal">
                    {{ selectedEvents.length }}
                  </Badge>
                </template>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent class="w-[200px]" align="end">
              <DropdownMenuCheckboxItem
                v-for="event in eventTypes"
                :key="event"
                :model-value="selectedEvents.includes(event)"
                @update:model-value="(checked) => toggleEventFilter(event, checked)"
              >
                {{ event }}
              </DropdownMenuCheckboxItem>
              <template v-if="selectedEvents.length > 0">
                <DropdownMenuSeparator />
                <DropdownMenuItem @click="clearEventFilter">
                  {{ $t('clear_filters') }}
                </DropdownMenuItem>
              </template>
            </DropdownMenuContent>
          </DropdownMenu>

          <!-- Branch filter -->
          <DropdownMenu :modal="false">
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" class="h-8 border-dashed">
                <Icon name="branch" class="mr-2 h-4 w-4" />
                {{ $t('repo.pipeline.filter.branch') }}
                <template v-if="branchFilter">
                  <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                  <span class="mx-2" aria-hidden="true">|</span>
                  <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                  <Badge variant="secondary" class="rounded-sm px-1 font-normal max-w-[100px] truncate">
                    {{ branchFilterLabel }}
                  </Badge>
                </template>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent class="w-[220px] max-h-[300px] overflow-y-auto" align="end">
              <DropdownMenuRadioGroup v-model="branchFilter">
                <DropdownMenuRadioItem value="">
                  {{ $t('repo.pipeline.filter.all_branches') }}
                </DropdownMenuRadioItem>
                <DropdownMenuSeparator v-if="branches.length > 0" />
                <DropdownMenuRadioItem v-for="branch in branches" :key="branch" :value="branch">
                  {{ branch }}
                </DropdownMenuRadioItem>
                <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                <template v-if="pullRequests.length > 0">
                  <DropdownMenuSeparator />
                  <DropdownMenuRadioItem
                    v-for="pr in pullRequests"
                    :key="`pr-${pr.index}`"
                    :value="`refs/pull/${pr.index}/head`"
                  >
                    PR #{{ pr.index }}
                  </DropdownMenuRadioItem>
                </template>
                <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>

          <!-- Date range filter -->
          <Popover>
            <PopoverTrigger as-child>
              <Button variant="outline" size="sm" class="h-8 border-dashed">
                <Icon name="lucide:calendar" class="mr-2 h-4 w-4" />
                {{ $t('repo.pipeline.filter.date_range') }}
                <template v-if="hasDateFilter">
                  <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                  <span class="mx-2" aria-hidden="true">|</span>
                  <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                  <Badge variant="secondary" class="rounded-sm px-1 font-normal">
                    {{ dateRangeLabel }}
                  </Badge>
                </template>
              </Button>
            </PopoverTrigger>
            <PopoverContent class="w-auto p-3" align="end">
              <div class="space-y-2.5">
                <div class="space-y-1.5">
                  <Label class="text-xs">{{ $t('repo.pipeline.filter.date_from') }}</Label>
                  <DatePicker
                    v-model="dateFrom"
                    :placeholder="$t('repo.pipeline.filter.date_from')"
                    class="h-9 text-sm"
                  />
                </div>
                <div class="space-y-1.5">
                  <Label class="text-xs">{{ $t('repo.pipeline.filter.date_to') }}</Label>
                  <DatePicker v-model="dateTo" :placeholder="$t('repo.pipeline.filter.date_to')" class="h-9 text-sm" />
                </div>
                <div class="flex gap-2 pt-1">
                  <Button variant="outline" size="sm" class="flex-1 h-8" @click="clearDateFilter">
                    {{ $t('clear') }}
                  </Button>
                </div>
              </div>
            </PopoverContent>
          </Popover>

          <!-- Clear all filters (kept mounted to reserve its space and avoid a row shift) -->
          <Button
            variant="ghost"
            size="sm"
            class="h-8"
            :class="{ 'invisible pointer-events-none': !hasActiveFilters }"
            :aria-hidden="!hasActiveFilters"
            :tabindex="hasActiveFilters ? undefined : -1"
            @click="clearAllFilters"
          >
            {{ $t('clear_filters') }}
          </Button>
        </div>
      </div>
    </template>
    <ManualPipelinePopup :open="showManualPipelinePopup" @close="showManualPipelinePopup = false" />
    <router-view />
  </Scaffold>
  <router-view v-else-if="repo && repoPermissions" />
</template>

<script lang="ts" setup>
import type { Ref } from 'vue';
import { computed, onMounted, provide as vueProvide, ref, toRef, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { DatePicker } from '~/components/ui/date-picker';
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
import { Label } from '~/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import SearchInput from '~/components/ui/input/SearchInput.vue';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import Icon from '~/components/atomic/Icon.vue';
import type { IconNames } from '~/components/atomic/Icon.types';
import IconButton from '~/components/atomic/IconButton.vue';
import ManualPipelinePopup from '~/components/layout/popups/ManualPipelinePopup.vue';
import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import useApiClient from '~/compositions/useApiClient';
import useAuthentication from '~/compositions/useAuthentication';
import { useForgeStore } from '~/compositions/useForgeStore';
import { provide } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import useRepos from '~/compositions/useRepos';
import type { Forge, Repo, RepoPermissions } from '~/lib/api/types';
import { usePipelineStore } from '~/store/pipelines';
import { useRepoStore } from '~/store/repos';

const props = defineProps<{
  repoId: string;
}>();

const PR_REF_REGEX = /^refs\/pull\/(\d+)\/head$/;

const _repoId = toRef(props, 'repoId');
const repositoryId = computed(() => Number.parseInt(_repoId.value, 10));
const repoStore = useRepoStore();
const pipelineStore = usePipelineStore();
const apiClient = useApiClient();
const notifications = useNotifications();
const { isAuthenticated } = useAuthentication();
const route = useRoute();
const router = useRouter();
const i18n = useI18n();
const forgeStore = useForgeStore();
const { updateLastAccess } = useRepos();

const repo = repoStore.getRepo(repositoryId);
const repoPermissions = ref<RepoPermissions>();
const copiedRepoSlug = ref(false);

async function copyRepoSlug() {
  if (!repo.value) return;
  try {
    await navigator.clipboard.writeText(repo.value.full_name);
    copiedRepoSlug.value = true;
    setTimeout(() => {
      copiedRepoSlug.value = false;
    }, 2000);
  } catch {
    // Failed to copy
  }
}
const pipelines = pipelineStore.getRepoPipelines(repositoryId);
const isMultiselectMode = ref(false);
const showManualPipelinePopup = ref(false);

// Search and filter state
const searchQuery = ref('');
const dateFrom = ref('');
const dateTo = ref('');
const selectedEvents = ref<string[]>([]);
const branchFilter = ref('');
const workflowFilter = ref<string[]>([]);
const hasDateFilter = ref(false); // Track if user has actually set a filter

// Branches and PRs for filters
const branches = ref<string[]>([]);
const pullRequests = ref<{ index: string; title: string }[]>([]);

// Set default "to" date to today (for the input field, not the filter)
const today = new Date().toISOString().split('T')[0];

// Event types for filter
const eventTypes = ['push', 'pull_request', 'pull_request_closed', 'tag', 'cron', 'manual', 'release', 'deploy'];

const dateRangeLabel = computed(() => {
  if (dateFrom.value && dateTo.value) {
    return `${dateFrom.value} - ${dateTo.value}`;
  } else if (dateFrom.value) {
    return i18n.t('repo.pipeline.filter.from_date', { date: dateFrom.value });
  } else if (dateTo.value) {
    return i18n.t('repo.pipeline.filter.until_date', { date: dateTo.value });
  }
  return '';
});

const branchFilterLabel = computed(() => {
  if (!branchFilter.value) return '';
  // Check if it's a PR ref
  const prMatch = branchFilter.value.match(PR_REF_REGEX);
  if (prMatch) {
    return `PR #${prMatch[1]}`;
  }
  return branchFilter.value;
});

const hasActiveFilters = computed(
  () =>
    selectedEvents.value.length > 0 || !!branchFilter.value || hasDateFilter.value || workflowFilter.value.length > 0,
);

// Watch for changes to activate the filter
watch([dateFrom, dateTo], ([newFrom, newTo]) => {
  hasDateFilter.value = !!(newFrom || (newTo && newTo !== today));
});

function toggleEventFilter(event: string, checked: boolean) {
  if (checked) {
    if (!selectedEvents.value.includes(event)) {
      selectedEvents.value = [...selectedEvents.value, event];
    }
  } else {
    selectedEvents.value = selectedEvents.value.filter((e) => e !== event);
  }
}

function clearEventFilter() {
  selectedEvents.value = [];
}

function clearDateFilter() {
  dateFrom.value = '';
  dateTo.value = '';
  hasDateFilter.value = false;
}

function clearAllFilters() {
  selectedEvents.value = [];
  branchFilter.value = '';
  workflowFilter.value = [];
  clearDateFilter();
}

provide('repo', repo as Ref<Repo>); // can't be undefined because of v-if in template
provide('repo-permissions', repoPermissions as Ref<RepoPermissions>); // can't be undefined because of v-if in template
provide('pipelines', pipelines);
vueProvide('multiselect-mode', isMultiselectMode);
vueProvide('search-query', searchQuery);
vueProvide('date-from', dateFrom);
vueProvide('date-to', dateTo);
vueProvide('selected-events', selectedEvents);
vueProvide('branch-filter', branchFilter);
vueProvide('workflow-filter', workflowFilter);

function toggleMultiselectMode() {
  isMultiselectMode.value = !isMultiselectMode.value;
}

const forge = ref<Forge>();

// Map forge types to local icon names, with special handling for known instances
function getForgeIconName(f: Forge): IconNames {
  // Check for specific forge instances by URL
  if (f.url.toLowerCase().includes('codeberg')) {
    return 'codeberg';
  }
  if (f.url.toLowerCase().includes('codefloe')) {
    return 'codefloe';
  }
  // Fall back to forge type
  const typeToIcon: Record<string, IconNames> = {
    github: 'github',
    gitlab: 'gitlab',
    gitea: 'gitea',
    forgejo: 'forgejo',
    bitbucket: 'bitbucket',
    'bitbucket-dc': 'bitbucket',
    addon: 'repo',
  };
  return typeToIcon[f.type] || 'repo';
}

const forgeIcon = computed<IconNames>(() => {
  if (forge.value) {
    return getForgeIconName(forge.value);
  }
  return 'repo';
});

// Load forge as soon as repo.forge_id is available (works for cached repos too)
watch(
  () => repo.value?.forge_id,
  async (forgeId) => {
    if (forgeId && !forge.value) {
      forge.value = (await forgeStore.getForge(forgeId)).value;
    }
  },
  { immediate: true },
);

// Load branches/PRs as soon as repo.id is available
watch(
  () => repo.value?.id,
  async (repoId) => {
    if (repoId && branches.value.length === 0) {
      // Load branches and PRs in parallel
      const loadBranches = async () => {
        try {
          branches.value = await apiClient.getRepoBranches(repoId, { page: 1 });
        } catch {
          branches.value = [];
        }
      };

      const loadPRs = async () => {
        if (repo.value?.pr_enabled && repo.value?.allow_pr) {
          try {
            const prs = await apiClient.getRepoPullRequests(repoId, { page: 1 });
            pullRequests.value = prs.map((pr) => ({ index: pr.index, title: pr.title }));
          } catch {
            pullRequests.value = [];
          }
        }
      };

      void Promise.all([loadBranches(), loadPRs()]);
    }
  },
  { immediate: true },
);

async function loadRepo() {
  repoPermissions.value = await apiClient.getRepoPermissions(repositoryId.value);
  if (!repoPermissions.value.pull) {
    notifications.notify({
      type: 'error',
      duration: 9000,
      title: i18n.t('notify_title_error'),
      text: i18n.t('repo.notify_text_not_allowed'),
    });
    // no access and not authenticated, redirect to login
    if (!isAuthenticated) {
      await router.replace({ name: 'login', query: { url: route.fullPath } });
      return;
    }
    await router.replace({ name: 'home' });
    return;
  }

  // Load repo and pipelines (watchers will handle forge/branches/PRs)
  await repoStore.loadRepo(repositoryId.value);
  await pipelineStore.loadRepoPipelines(repositoryId.value);

  updateLastAccess(repositoryId.value);
}

onMounted(() => {
  loadRepo();
});

watch([repositoryId], () => {
  loadRepo();
});
</script>

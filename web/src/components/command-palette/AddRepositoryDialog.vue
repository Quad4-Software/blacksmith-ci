<template>
  <Dialog v-model:open="repositoryDialog.isOpen.value">
    <DialogContent class="sm:max-w-[700px] max-h-[85vh] flex flex-col">
      <!-- Main repository selection view -->
      <template v-if="!showImportDialog">
        <DialogHeader>
          <DialogTitle>{{ $t('repo.add') }}</DialogTitle>
          <DialogDescription>
            {{ $t('repo.add_desc') }}
          </DialogDescription>
        </DialogHeader>

        <!-- Expired forge warning banner -->
        <div v-if="expiredForges.length > 0" class="mt-4 rounded-md border border-destructive/50 bg-destructive/5 p-3">
          <div class="flex items-start gap-2">
            <Icon name="alert" class="h-4 w-4 shrink-0 text-destructive mt-0.5" />
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium text-destructive">
                {{ $t('repo.enable.forge_connection_issue') }}
              </p>
              <p class="text-xs text-muted-foreground mt-0.5">
                {{ $t('repo.enable.forge_connection_issue_desc') }}
              </p>
              <div class="flex flex-wrap gap-2 mt-2">
                <Button
                  v-for="forge in expiredForges"
                  :key="forge.id"
                  variant="outline"
                  size="sm"
                  class="h-7 text-xs"
                  as="a"
                  :href="getReconnectUrl(forge.forge_id)"
                >
                  <img
                    v-if="getUserForgeCustomIconUrl(forge)"
                    :src="getUserForgeCustomIconUrl(forge)!"
                    class="mr-1.5 h-3 w-3"
                    alt=""
                  />
                  <Icon v-else :name="getUserForgeIconName(forge)" class="mr-1.5 h-3 w-3" />
                  <span class="font-medium">{{ forge.forge_login }}</span>
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  <span class="text-muted-foreground mx-1">@</span>
                  <span class="text-muted-foreground">{{ getForgeDisplayName(forge) }}</span>
                  <Icon name="lucide:refresh-cw" class="ml-1.5 h-3 w-3" />
                </Button>
              </div>
            </div>
          </div>
        </div>

        <div class="flex-1 flex flex-col min-h-0 mt-4">
          <!-- Search and filter controls -->
          <div class="flex flex-col sm:flex-row gap-2 mb-4">
            <SearchInput v-model="search" :placeholder="$t('search')" class="flex-1" />
            <!-- Forge filter (only show if multiple forges) -->
            <Select v-if="repoForges.length > 1" v-model="forgeFilter">
              <SelectTrigger class="w-full sm:w-[180px]">
                <SelectValue :placeholder="$t('repo.enable.filter.all_forges')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{{ $t('repo.enable.filter.all_forges') }}</SelectItem>
                <SelectItem v-for="forge in repoForges" :key="forge.id" :value="String(forge.id)">
                  <div class="flex items-center gap-2">
                    <Icon :name="getForgeIconName(forge)" class="h-4 w-4" />
                    <span>{{ forge.url.replace(PROTOCOL_PREFIX_REGEX, '').replace(TRAILING_SLASH_REGEX, '') }}</span>
                  </div>
                </SelectItem>
              </SelectContent>
            </Select>
            <Select v-model="ownerFilter">
              <SelectTrigger class="w-full sm:w-[180px]">
                <SelectValue :placeholder="$t('repo.enable.filter.all_owners')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{{ $t('repo.enable.filter.all_owners') }}</SelectItem>
                <SelectItem v-for="owner in owners" :key="owner" :value="owner">{{ owner }}</SelectItem>
              </SelectContent>
            </Select>
            <Select v-model="statusFilter">
              <SelectTrigger class="w-full sm:w-[160px]">
                <SelectValue :placeholder="$t('repo.enable.filter.all')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{{ $t('repo.enable.filter.all') }}</SelectItem>
                <SelectItem value="disabled">{{ $t('repo.enable.filter.not_enabled') }}</SelectItem>
                <SelectItem value="enabled">{{ $t('repo.enable.filter.enabled') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <!-- Selection actions -->
          <div v-if="notEnabledFilteredRepos.length > 0" class="flex items-center justify-between mb-3 px-1">
            <Button v-if="!isAllSelected" variant="ghost" size="sm" class="h-7 px-2 text-xs" @click="selectAll">
              {{ $t('repo.enable.select_all', { count: notEnabledFilteredRepos.length }) }}
            </Button>
            <Button v-else variant="ghost" size="sm" class="h-7 px-2 text-xs" @click="clearSelection">
              {{ $t('repo.enable.clear_selection') }}
            </Button>
            <span v-if="selectedRepos.size > 0" class="text-sm text-muted-foreground">
              {{ $t('repo.enable.selected_count', { count: selectedRepos.size }) }}
            </span>
          </div>

          <!-- Repository list -->
          <div class="flex-1 overflow-y-auto space-y-1 min-h-0">
            <template v-if="repos !== undefined && filteredRepos.length > 0">
              <div
                v-for="repo in filteredRepos"
                :key="repo.forge_remote_id"
                class="flex items-center gap-3 p-3 rounded-md border border-border hover:bg-accent/50 transition-colors"
                :class="{ 'bg-accent/30': selectedRepos.has(repo.forge_remote_id) }"
              >
                <!-- Checkbox for disabled repos -->
                <Checkbox
                  v-if="!repo.active"
                  :checked="selectedRepos.has(repo.forge_remote_id)"
                  @update:checked="toggleRepoSelection(repo)"
                />
                <div v-else class="w-4" />

                <!-- Forge source column (show when multiple forges are configured) -->
                <div
                  v-if="forges && forges.length > 1 && getForgeById(repo.forge_id)"
                  class="flex items-center gap-1.5 shrink-0 min-w-[100px] max-w-[140px]"
                  :title="getForgeById(repo.forge_id)!.url"
                >
                  <Icon
                    :name="getForgeIconName(getForgeById(repo.forge_id)!)"
                    class="h-4 w-4 shrink-0 text-muted-foreground"
                  />
                  <span class="text-xs text-muted-foreground truncate">
                    {{ getForgeHostname(getForgeById(repo.forge_id)!) }}
                  </span>
                </div>

                <!-- Repo name -->
                <div class="flex-1 min-w-0">
                  <span class="text-sm text-foreground truncate block">{{ repo.full_name }}</span>
                </div>

                <!-- Status / Action -->
                <div class="flex items-center gap-2 shrink-0">
                  <Badge v-if="repo.active" variant="secondary" class="text-xs">
                    {{ $t('repo.enable.enabled') }}
                  </Badge>
                  <DropdownMenu v-else>
                    <DropdownMenuTrigger as-child>
                      <Button size="sm" variant="outline" :disabled="isActivatingRepo">
                        <Icon
                          v-if="isActivatingRepo && activatingRepoIds.has(repo.forge_remote_id)"
                          name="spinner"
                          class="mr-1.5 h-3 w-3 animate-spin"
                        />
                        {{ $t('repo.enable.enable') }}
                        <Icon name="chevron-down" class="ml-1.5 h-3 w-3" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem @click.stop="activateSingleRepo(repo)">
                        {{ $t('repo.enable.enable') }}
                      </DropdownMenuItem>
                      <DropdownMenuItem @click.stop="activateSingleRepoWithImport(repo)">
                        {{ $t('repo.enable.import.migrate') }}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </div>
            </template>
            <div v-else-if="loading" class="flex justify-center p-8">
              <Icon name="spinner" class="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
            <div v-else-if="repos !== undefined && repos.length === 0" class="text-center p-8 text-muted-foreground">
              {{ $t('no_repos') }}
            </div>
            <div v-else-if="filteredRepos.length === 0" class="text-center p-8 text-muted-foreground">
              {{ $t('no_search_results') }}
            </div>
          </div>
        </div>

        <!-- Footer actions -->
        <div class="flex items-center justify-between gap-2 mt-4 pt-4 border-t border-border">
          <div class="text-sm text-muted-foreground">
            <template v-if="repos">
              {{
                $t('repo.enable.total_repos', {
                  total: filteredRepos.length,
                  not_enabled: notEnabledFilteredRepos.length,
                })
              }}
            </template>
          </div>
          <div class="flex gap-2">
            <Button variant="outline" @click="handleClose">
              {{ $t('close') }}
            </Button>
            <Button v-if="selectedRepos.size > 0" :disabled="isActivatingRepo" @click="activateSelectedRepos">
              <Icon v-if="isActivatingRepo" name="spinner" class="mr-2 h-4 w-4 animate-spin" />
              {{ $t('repo.enable.enable_selected', { count: selectedRepos.size }) }}
            </Button>
          </div>
        </div>
      </template>

      <!-- Import settings view -->
      <template v-else>
        <DialogHeader>
          <DialogTitle>{{ $t('repo.enable.import.title') }}</DialogTitle>
          <DialogDescription>
            {{ $t('repo.enable.import.desc') }}
          </DialogDescription>
        </DialogHeader>

        <div class="flex-1 flex flex-col min-h-0 mt-4 space-y-4">
          <!-- Source repository selection with search -->
          <div class="space-y-2">
            <label class="text-sm font-medium">{{ $t('repo.enable.import.select_source') }}</label>
            <Combobox
              v-model="selectedSourceRepoId"
              v-model:search-term="sourceRepoSearch"
              :filter-function="filterSourceRepos"
            >
              <ComboboxAnchor class="w-full">
                <ComboboxInput
                  :placeholder="$t('repo.enable.import.source_placeholder')"
                  :display-value="getSelectedSourceRepoDisplayName"
                />
              </ComboboxAnchor>
              <ComboboxList class="w-[var(--reka-combobox-trigger-width)]">
                <ComboboxViewport>
                  <ComboboxEmpty>{{ $t('no_search_results') }}</ComboboxEmpty>
                  <ComboboxItem
                    v-for="repo in importableRepos"
                    :key="repo.id"
                    :value="String(repo.id)"
                    class="cursor-pointer"
                  >
                    <div class="flex items-center gap-2">
                      <Icon
                        v-if="forges && forges.length > 1 && getForgeById(repo.forge_id)"
                        :name="getForgeIconName(getForgeById(repo.forge_id)!)"
                        class="h-4 w-4 text-muted-foreground"
                      />
                      <span>{{ repo.full_name }}</span>
                    </div>
                    <ComboboxItemIndicator class="ml-auto">
                      <Icon name="lucide:check" class="h-4 w-4" />
                    </ComboboxItemIndicator>
                  </ComboboxItem>
                </ComboboxViewport>
              </ComboboxList>
            </Combobox>
            <p v-if="importableRepos.length === 0" class="text-sm text-muted-foreground">
              {{ $t('repo.enable.import.no_importable_repos') }}
            </p>
          </div>

          <!-- Import preview -->
          <div v-if="importPreview" class="rounded-md border border-border p-4 space-y-3">
            <h4 class="text-sm font-medium">{{ $t('repo.enable.import.preview.title') }}</h4>
            <p class="text-xs text-muted-foreground">{{ $t('repo.enable.import.preview.from_repo') }}</p>
            <div class="flex flex-wrap gap-2 text-sm text-muted-foreground">
              <Badge variant="secondary">
                {{ $t('repo.enable.import.preview.secrets', { count: importPreview.secret_count }) }}
              </Badge>
              <Badge variant="secondary">
                {{ $t('repo.enable.import.preview.registries', { count: importPreview.registry_count }) }}
              </Badge>
              <Badge variant="secondary">
                {{ $t('repo.enable.import.preview.crons', { count: importPreview.cron_count }) }}
              </Badge>
              <Badge variant="secondary">
                {{ $t('repo.enable.import.preview.pipelines', { count: importPreview.pipeline_count }) }}
              </Badge>
            </div>
            <!-- Org-level assets from other forges -->
            <div
              v-if="importPreview.org_secret_count > 0 || importPreview.org_registry_count > 0"
              class="mt-3 pt-3 border-t border-border"
            >
              <p class="text-xs text-muted-foreground mb-2">
                {{
                  $t('repo.enable.import.preview.from_org', {
                    name: importPreview.org_name,
                    forge: importPreview.org_source_forge,
                  })
                }}
              </p>
              <div class="flex flex-wrap gap-2 text-sm text-muted-foreground">
                <Badge v-if="importPreview.org_secret_count > 0" variant="outline">
                  {{ $t('repo.enable.import.preview.org_secrets', { count: importPreview.org_secret_count }) }}
                </Badge>
                <Badge v-if="importPreview.org_registry_count > 0" variant="outline">
                  {{ $t('repo.enable.import.preview.org_registries', { count: importPreview.org_registry_count }) }}
                </Badge>
              </div>
            </div>
          </div>

          <!-- Import options -->
          <div v-if="selectedSourceRepoId" class="space-y-3">
            <div class="flex items-center gap-3 p-3 rounded-md border border-border">
              <Checkbox id="import-settings" v-model:checked="importOptions.settings" />
              <div class="flex-1">
                <label for="import-settings" class="text-sm font-medium cursor-pointer">
                  {{ $t('repo.enable.import.options.settings') }}
                </label>
                <p class="text-xs text-muted-foreground">
                  {{ $t('repo.enable.import.options.settings_desc') }}
                </p>
              </div>
            </div>

            <div class="flex items-center gap-3 p-3 rounded-md border border-border">
              <Checkbox id="import-secrets" v-model:checked="importOptions.secrets" />
              <div class="flex-1">
                <label for="import-secrets" class="text-sm font-medium cursor-pointer">
                  {{ $t('repo.enable.import.options.secrets') }}
                </label>
                <p class="text-xs text-muted-foreground">
                  {{ $t('repo.enable.import.options.secrets_desc') }}
                </p>
              </div>
            </div>

            <div class="flex items-center gap-3 p-3 rounded-md border border-border">
              <Checkbox id="import-registries" v-model:checked="importOptions.registries" />
              <div class="flex-1">
                <label for="import-registries" class="text-sm font-medium cursor-pointer">
                  {{ $t('repo.enable.import.options.registries') }}
                </label>
                <p class="text-xs text-muted-foreground">
                  {{ $t('repo.enable.import.options.registries_desc') }}
                </p>
              </div>
            </div>

            <div class="flex items-center gap-3 p-3 rounded-md border border-border">
              <Checkbox id="import-crons" v-model:checked="importOptions.crons" />
              <div class="flex-1">
                <label for="import-crons" class="text-sm font-medium cursor-pointer">
                  {{ $t('repo.enable.import.options.crons') }}
                </label>
                <p class="text-xs text-muted-foreground">
                  {{ $t('repo.enable.import.options.crons_desc') }}
                </p>
              </div>
            </div>

            <div class="flex items-center gap-3 p-3 rounded-md border border-border">
              <Checkbox id="import-pipelines" v-model:checked="importOptions.pipelines" />
              <div class="flex-1">
                <label for="import-pipelines" class="text-sm font-medium cursor-pointer">
                  {{ $t('repo.enable.import.options.pipelines') }}
                </label>
                <p class="text-xs text-muted-foreground">
                  {{ $t('repo.enable.import.options.pipelines_desc') }}
                </p>
              </div>
            </div>

            <div class="flex items-center gap-3 p-3 rounded-md border border-destructive/50 bg-destructive/5">
              <Checkbox id="import-delete" v-model:checked="importOptions.deleteSource" />
              <div class="flex-1">
                <label for="import-delete" class="text-sm font-medium cursor-pointer text-destructive">
                  {{ $t('repo.enable.import.options.delete_source') }}
                </label>
                <p class="text-xs text-muted-foreground">
                  {{ $t('repo.enable.import.options.delete_source_desc') }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- Import dialog footer -->
        <div class="flex items-center justify-end gap-2 mt-4 pt-4 border-t border-border">
          <Button variant="outline" @click="skipImport">
            {{ $t('repo.enable.import.skip') }}
          </Button>
          <Button
            v-if="selectedSourceRepoId && hasSelectedImportOptions"
            :disabled="isImporting"
            @click="performImport"
          >
            <Icon v-if="isImporting" name="spinner" class="mr-2 h-4 w-4 animate-spin" />
            {{ $t('repo.enable.import.import_button') }}
          </Button>
        </div>
      </template>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
import type { IconNames } from '~/components/atomic/Icon.types';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import {
  Combobox,
  ComboboxAnchor,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxItemIndicator,
  ComboboxList,
  ComboboxViewport,
} from '~/components/ui/combobox';
import SearchInput from '~/components/ui/input/SearchInput.vue';
import useApiClient from '~/compositions/useApiClient';
import useConfig from '~/compositions/useConfig';
import { useForges } from '~/compositions/useForges';
import { useGlobalDialogs } from '~/compositions/useGlobalDialogs';
import useNotifications from '~/compositions/useNotifications';
import type { Forge, ImportPreviewResponse, Repo, RepoResponse, UserForge } from '~/lib/api/types';

const PROTOCOL_PREFIX_REGEX = /^https?:\/\//;
const TRAILING_SLASH_REGEX = /\/$/;

const { t } = useI18n();
const router = useRouter();
const apiClient = useApiClient();
const config = useConfig();
const notifications = useNotifications();
const { repositoryDialog } = useGlobalDialogs();
const { forges, getForgeById } = useForges();

const repos = ref<Repo[]>();
const userForges = ref<UserForge[]>([]);
const search = ref('');
const ownerFilter = ref('all');
const forgeFilter = ref('all');
const statusFilter = ref('all');
const loading = ref(false);
const selectedRepos = ref<Set<string>>(new Set());
const activatingRepoIds = ref<Set<string>>(new Set());
const isActivatingRepo = ref(false);

// Computed: expired forges that need attention
const expiredForges = computed(() => userForges.value.filter((uf) => uf.status === 'expired'));

// Get reconnect URL for a forge
function getReconnectUrl(forgeId: number): string {
  const rootPath = config.rootPath || '';
  return `${rootPath}/authorize?forge_id=${forgeId}&reconnect=true`;
}

// Get icon name for a UserForge
function getUserForgeIconName(userForge: UserForge): IconNames {
  // Check for custom icon first
  if (userForge.forge_icon) {
    // If it's not a URL, treat it as a predefined icon name
    if (!isValidUrl(userForge.forge_icon)) {
      const customIcon = userForge.forge_icon.toLowerCase();
      const validIcons: Record<string, IconNames> = {
        codeberg: 'codeberg',
        codefloe: 'codefloe',
        github: 'github',
        gitlab: 'gitlab',
        gitea: 'gitea',
        forgejo: 'forgejo',
        bitbucket: 'bitbucket',
        repo: 'repo',
      };
      if (validIcons[customIcon]) {
        return validIcons[customIcon];
      }
    }
  }

  const forgeType = userForge.forge_type?.toLowerCase() || '';
  const forgeName = userForge.forge_name?.toLowerCase() || '';

  if (forgeName.includes('codeberg')) return 'codeberg';
  if (forgeName.includes('codefloe')) return 'codefloe';

  const typeToIcon: Record<string, IconNames> = {
    github: 'github',
    gitlab: 'gitlab',
    gitea: 'gitea',
    forgejo: 'forgejo',
    bitbucket: 'bitbucket',
    'bitbucket-dc': 'bitbucket',
    addon: 'repo',
  };
  return typeToIcon[forgeType] || 'repo';
}

// Get custom icon URL for a UserForge if the icon is a URL
function getUserForgeCustomIconUrl(userForge: UserForge): string | null {
  if (userForge.forge_icon && isValidUrl(userForge.forge_icon)) {
    return userForge.forge_icon;
  }
  return null;
}

// Get forge display name (hostname)
function getForgeDisplayName(userForge: UserForge): string {
  if (!userForge.forge_name) return userForge.forge_type || '';
  try {
    const url = new URL(userForge.forge_name);
    return url.hostname;
  } catch {
    return userForge.forge_name.replace(PROTOCOL_PREFIX_REGEX, '').replace(TRAILING_SLASH_REGEX, '');
  }
}

// Import dialog state
const showImportDialog = ref(false);
const activatedRepoId = ref<number | null>(null);
const importableRepos = ref<Repo[]>([]);
const selectedSourceRepoId = ref<string>('');
const sourceRepoSearch = ref('');
const importPreview = ref<ImportPreviewResponse | null>(null);
const isImporting = ref(false);
const importOptions = ref({
  settings: true,
  secrets: true,
  registries: true,
  crons: true,
  pipelines: false,
  deleteSource: false,
});

// Filter function for source repo combobox
function filterSourceRepos(repos: string[], searchTerm: string): string[] {
  if (!searchTerm) return repos;
  const term = searchTerm.toLowerCase();
  return repos.filter((repoId) => {
    const repo = importableRepos.value.find((r) => String(r.id) === repoId);
    return repo?.full_name.toLowerCase().includes(term);
  });
}

// Get display name for selected source repo
function getSelectedSourceRepoDisplayName(repoId: string): string {
  if (!repoId) return '';
  const repo = importableRepos.value.find((r) => String(r.id) === repoId);
  return repo?.full_name || '';
}

// Check if any import options are selected
const hasSelectedImportOptions = computed(
  () =>
    importOptions.value.settings ||
    importOptions.value.secrets ||
    importOptions.value.registries ||
    importOptions.value.crons ||
    importOptions.value.pipelines,
);

// Check if a string is a valid URL
function isValidUrl(str: string): boolean {
  try {
    return Boolean(new URL(str));
  } catch {
    return false;
  }
}

// Get forge icon - returns either an IconNames or null if it's a custom URL icon
function getForgeIconName(forge: Forge): IconNames {
  // Check for custom icon first
  if (forge.icon) {
    // If it's not a URL, treat it as a predefined icon name
    if (!isValidUrl(forge.icon)) {
      const customIcon = forge.icon.toLowerCase();
      const validIcons: Record<string, IconNames> = {
        codeberg: 'codeberg',
        codefloe: 'codefloe',
        github: 'github',
        gitlab: 'gitlab',
        gitea: 'gitea',
        forgejo: 'forgejo',
        bitbucket: 'bitbucket',
        repo: 'repo',
      };
      if (validIcons[customIcon]) {
        return validIcons[customIcon];
      }
    }
  }

  // Fall back to URL-based detection
  if (forge.url.toLowerCase().includes('codeberg')) {
    return 'codeberg';
  }
  if (forge.url.toLowerCase().includes('codefloe')) {
    return 'codefloe';
  }

  const typeToIcon: Record<string, IconNames> = {
    github: 'github',
    gitlab: 'gitlab',
    gitea: 'gitea',
    forgejo: 'forgejo',
    bitbucket: 'bitbucket',
    'bitbucket-dc': 'bitbucket',
    addon: 'repo',
  };
  return typeToIcon[forge.type] || 'repo';
}

// Get forge hostname for display
function getForgeHostname(forge: Forge): string {
  try {
    const url = new URL(forge.url);
    return url.hostname;
  } catch {
    return forge.url.replace(PROTOCOL_PREFIX_REGEX, '').replace(TRAILING_SLASH_REGEX, '');
  }
}

// Get unique forges from repos
const repoForges = computed(() => {
  if (!repos.value || !forges.value) return [];
  const forgeIds = new Set(repos.value.map((r) => r.forge_id));
  return forges.value.filter((f) => forgeIds.has(f.id));
});

// Computed: unique owners from repos
const owners = computed(() => {
  if (!repos.value) return [];
  const ownerSet = new Set<string>();
  for (const repo of repos.value) {
    ownerSet.add(repo.owner);
  }
  return [...ownerSet].toSorted();
});

// Computed: filtered repos based on search, owner, forge and status
const filteredRepos = computed(() => {
  if (!repos.value) return [];

  let filtered = repos.value;

  // Apply owner filter
  if (ownerFilter.value !== 'all') {
    filtered = filtered.filter((r) => r.owner === ownerFilter.value);
  }

  // Apply forge filter
  if (forgeFilter.value !== 'all') {
    const forgeId = Number.parseInt(forgeFilter.value, 10);
    filtered = filtered.filter((r) => r.forge_id === forgeId);
  }

  // Apply status filter
  if (statusFilter.value === 'disabled') {
    filtered = filtered.filter((r) => !r.active);
  } else if (statusFilter.value === 'enabled') {
    filtered = filtered.filter((r) => r.active);
  }

  // Apply search filter
  if (search.value.trim()) {
    const query = search.value.toLowerCase();
    filtered = filtered.filter((r) => r.full_name.toLowerCase().includes(query));
  }

  return filtered;
});

// Computed: not-enabled repos in current filtered view
const notEnabledFilteredRepos = computed(() => filteredRepos.value.filter((r) => !r.active));

// Computed: check if all not-enabled filtered repos are selected
const isAllSelected = computed(
  () =>
    notEnabledFilteredRepos.value.length > 0 &&
    notEnabledFilteredRepos.value.every((r) => selectedRepos.value.has(r.forge_remote_id)),
);

// Load repos when dialog opens
watch(
  () => repositoryDialog.isOpen.value,
  async (isOpen) => {
    if (isOpen) {
      // Reset state
      search.value = '';
      ownerFilter.value = 'all';
      forgeFilter.value = 'all';
      statusFilter.value = 'all';
      selectedRepos.value = new Set();

      // Load user forges to check for expired tokens
      try {
        userForges.value = await apiClient.getUserForges();
      } catch {
        // Non-fatal, just won't show expired forge warnings
      }

      if (!repos.value) {
        loading.value = true;
        try {
          repos.value = await apiClient.getRepoList({ all: true });
        } catch {
          notifications.notify({
            title: t('notify_title_error'),
            text: t('unknown_error'),
            type: 'error',
          });
        } finally {
          loading.value = false;
        }
      }
    }
  },
);

// Watch for source repo selection changes to load preview
watch(selectedSourceRepoId, () => {
  loadImportPreview();
});

function toggleRepoSelection(repo: Repo) {
  const newSet = new Set(selectedRepos.value);
  if (newSet.has(repo.forge_remote_id)) {
    newSet.delete(repo.forge_remote_id);
  } else {
    newSet.add(repo.forge_remote_id);
  }
  selectedRepos.value = newSet;
}

function selectAll() {
  const newSet = new Set(selectedRepos.value);
  for (const repo of notEnabledFilteredRepos.value) {
    newSet.add(repo.forge_remote_id);
  }
  selectedRepos.value = newSet;
}

function clearSelection() {
  selectedRepos.value = new Set();
}

async function activateSingleRepo(repo: Repo) {
  isActivatingRepo.value = true;
  activatingRepoIds.value.add(repo.forge_remote_id);

  try {
    const activatedRepo = await apiClient.activateRepo(repo.forge_remote_id, repo.forge_id);

    // Update local state
    if (repos.value) {
      const index = repos.value.findIndex((r) => r.forge_remote_id === repo.forge_remote_id);
      if (index !== -1) {
        repos.value[index] = { ...repos.value[index], active: true, id: activatedRepo.id };
      }
    }

    notifications.notify({
      title: t('notify_title_success'),
      text: t('repo.enable.notify_text_enabled'),
      type: 'success',
    });

    // Navigate to the repo
    repositoryDialog.close();
    await router.push({ name: 'repo', params: { repoId: activatedRepo.id } });
  } catch (error) {
    const message = (error as { message?: string })?.message || t('repo.enable.notify_text_error');
    notifications.notify({
      title: t('notify_title_error'),
      text: message,
      type: 'error',
    });
  } finally {
    activatingRepoIds.value.delete(repo.forge_remote_id);
    isActivatingRepo.value = false;
  }
}

async function activateSingleRepoWithImport(repo: Repo) {
  isActivatingRepo.value = true;
  activatingRepoIds.value.add(repo.forge_remote_id);

  try {
    const activatedRepo = await apiClient.activateRepo(repo.forge_remote_id, repo.forge_id);

    // Update local state
    if (repos.value) {
      const index = repos.value.findIndex((r) => r.forge_remote_id === repo.forge_remote_id);
      if (index !== -1) {
        repos.value[index] = { ...repos.value[index], active: true, id: activatedRepo.id };
      }
    }

    notifications.notify({
      title: t('notify_title_success'),
      text: t('repo.enable.notify_text_enabled'),
      type: 'success',
    });

    // Fetch importable repos and show import dialog
    try {
      const importable = await apiClient.getImportableRepos(activatedRepo.id);
      if (importable.length > 0) {
        activatedRepoId.value = activatedRepo.id;
        importableRepos.value = importable;
        showImportDialog.value = true;
        return; // Stay in dialog for import
      }
    } catch {
      // If we can't get importable repos, show a notification and navigate
      notifications.notify({
        title: t('notify_title_error'),
        text: t('repo.enable.import.no_importable_repos'),
        type: 'error',
      });
    }

    // Navigate to the repo if no importable repos
    repositoryDialog.close();
    await router.push({ name: 'repo', params: { repoId: activatedRepo.id } });
  } catch (error) {
    const message = (error as { message?: string })?.message || t('repo.enable.notify_text_error');
    notifications.notify({
      title: t('notify_title_error'),
      text: message,
      type: 'error',
    });
  } finally {
    activatingRepoIds.value.delete(repo.forge_remote_id);
    isActivatingRepo.value = false;
  }
}

async function activateSelectedRepos() {
  if (selectedRepos.value.size === 0) return;

  isActivatingRepo.value = true;
  const reposToActivate = repos.value?.filter((r) => selectedRepos.value.has(r.forge_remote_id)) ?? [];

  let successCount = 0;
  let failedCount = 0;
  let lastActivatedRepo: RepoResponse | null = null;
  const allMigrationMessages: string[] = [];
  const errorMessages: Set<string> = new Set();

  for (const repo of reposToActivate) {
    activatingRepoIds.value.add(repo.forge_remote_id);

    try {
      const activatedRepo = await apiClient.activateRepo(repo.forge_remote_id, repo.forge_id);

      // Update local state
      if (repos.value) {
        const index = repos.value.findIndex((r) => r.forge_remote_id === repo.forge_remote_id);
        if (index !== -1) {
          repos.value[index] = { ...repos.value[index], active: true, id: activatedRepo.id };
        }
      }

      // Collect any migration messages
      if (activatedRepo.messages && activatedRepo.messages.length > 0) {
        allMigrationMessages.push(...activatedRepo.messages);
      }

      successCount++;
      lastActivatedRepo = activatedRepo;
    } catch (error) {
      failedCount++;
      const message = (error as { message?: string })?.message;
      if (message) {
        errorMessages.add(message);
      }
    } finally {
      activatingRepoIds.value.delete(repo.forge_remote_id);
    }
  }

  // Clear selection
  selectedRepos.value = new Set();
  isActivatingRepo.value = false;

  // Show notification
  if (successCount > 0) {
    notifications.notify({
      title: t('notify_title_success'),
      text: t('repo.enable.notify_text_enabled_multiple', { count: successCount }),
      type: 'success',
    });
  }

  // Show migration messages as info notifications
  for (const message of allMigrationMessages) {
    notifications.notify({
      title: t('repo.enable.migration_info'),
      text: message,
      type: 'info',
    });
  }

  if (failedCount > 0) {
    // Show collected error messages, or fall back to generic message
    const errorText =
      errorMessages.size > 0
        ? [...errorMessages].join('; ')
        : t('repo.enable.notify_text_error_multiple', { count: failedCount });
    notifications.notify({
      title: t('notify_title_error'),
      text: errorText,
      type: 'error',
    });
  }

  // Navigate to the last activated repo if only one was selected, otherwise stay
  if (successCount === 1 && lastActivatedRepo) {
    repositoryDialog.close();
    await router.push({ name: 'repo', params: { repoId: lastActivatedRepo.id } });
  }
}

// Import-related functions
async function loadImportPreview() {
  if (!selectedSourceRepoId.value || !activatedRepoId.value) {
    importPreview.value = null;
    return;
  }

  try {
    const sourceRepoId = Number.parseInt(selectedSourceRepoId.value, 10);
    importPreview.value = await apiClient.getImportPreview(activatedRepoId.value, sourceRepoId);
  } catch {
    importPreview.value = null;
  }
}

function resetImportState() {
  showImportDialog.value = false;
  activatedRepoId.value = null;
  importableRepos.value = [];
  selectedSourceRepoId.value = '';
  sourceRepoSearch.value = '';
  importPreview.value = null;
  isImporting.value = false;
  importOptions.value = {
    settings: true,
    secrets: true,
    registries: true,
    crons: true,
    pipelines: false,
    deleteSource: false,
  };
}

async function skipImport() {
  const repoId = activatedRepoId.value;
  resetImportState();
  repositoryDialog.close();
  if (repoId) {
    await router.push({ name: 'repo', params: { repoId } });
  }
}

async function performImport() {
  if (!activatedRepoId.value || !selectedSourceRepoId.value) return;

  isImporting.value = true;
  try {
    const sourceRepoId = Number.parseInt(selectedSourceRepoId.value, 10);
    const result = await apiClient.importRepoSettings(activatedRepoId.value, {
      source_repo_id: sourceRepoId,
      settings: importOptions.value.settings,
      secrets: importOptions.value.secrets,
      registries: importOptions.value.registries,
      crons: importOptions.value.crons,
      pipelines: importOptions.value.pipelines,
      delete_source: importOptions.value.deleteSource,
    });

    notifications.notify({
      title: t('notify_title_success'),
      text: t('repo.enable.import.success_details', {
        secrets: result.secrets_imported,
        registries: result.registries_imported,
        crons: result.crons_imported,
        pipelines: result.pipelines_imported,
      }),
      type: 'success',
    });

    // If source was deleted, update the local repos list
    if (result.source_deleted && repos.value) {
      repos.value = repos.value.filter((r) => r.id !== sourceRepoId);
    }

    const repoId = activatedRepoId.value;
    resetImportState();
    repositoryDialog.close();
    await router.push({ name: 'repo', params: { repoId } });
  } catch {
    notifications.notify({
      title: t('notify_title_error'),
      text: t('repo.enable.import.error'),
      type: 'error',
    });
  } finally {
    isImporting.value = false;
  }
}

function handleClose() {
  search.value = '';
  ownerFilter.value = 'all';
  forgeFilter.value = 'all';
  statusFilter.value = 'all';
  selectedRepos.value = new Set();
  resetImportState();
  repositoryDialog.close();
}
</script>

<template>
  <div class="space-y-3 text-foreground">
    <div class="space-y-4">
      <!-- Toolbar with Search, Filters and Bulk Actions -->
      <div class="flex items-center justify-center gap-2 flex-wrap">
        <div class="flex items-center gap-2 flex-wrap">
          <!-- Search -->
          <SearchInput
            :model-value="searchTerm"
            :placeholder="$t('search')"
            class="w-[300px] min-w-[80px]"
            @update:model-value="(value) => $emit('update:searchTerm', value as string)"
          />

          <!-- Scope Filter -->
          <DropdownMenu :modal="false">
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" class="h-8 border-dashed">
                <Icon name="filter" class="mr-2 h-4 w-4" />
                {{ $t('scope_type') }}
                <template v-if="selectedScopeLabelsCount > 0">
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  <span class="mx-2" aria-hidden="true">|</span>
                  <Badge variant="secondary" class="rounded-sm px-1 font-normal">
                    {{ selectedScopeLabelsCount }}
                  </Badge>
                </template>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent class="w-[200px]" align="start">
              <div class="max-h-[300px] overflow-y-auto">
                <div v-if="uniqueScopes.length === 0" class="px-2 py-4 text-sm text-muted-foreground">
                  {{ $t('no_search_results') }}
                </div>
                <DropdownMenuCheckboxItem
                  v-for="label in uniqueScopes"
                  :key="label"
                  :model-value="selectedScopeLabels.includes(label)"
                  @update:model-value="(value) => toggleScopeLabel(label, !!value)"
                >
                  {{ label }}
                </DropdownMenuCheckboxItem>
              </div>
              <template v-if="selectedScopes.length > 0">
                <DropdownMenuSeparator />
                <DropdownMenuItem @click="clearScopeFilters">
                  {{ $t('clear_filters') }}
                </DropdownMenuItem>
              </template>
            </DropdownMenuContent>
          </DropdownMenu>

          <!-- Owner Filter -->
          <DropdownMenu :modal="false">
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" class="h-8 border-dashed">
                <Icon name="filter" class="mr-2 h-4 w-4" />
                {{ $t('owner') }}
                <template v-if="selectedOwners.length > 0">
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  <span class="mx-2" aria-hidden="true">|</span>
                  <Badge variant="secondary" class="rounded-sm px-1 font-normal">
                    {{ selectedOwners.length }}
                  </Badge>
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

          <!-- Forge Filter (only shown when multiple forges configured) -->
          <DropdownMenu v-if="showForgeColumn" :modal="false">
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" class="h-8 border-dashed">
                <Icon name="filter" class="mr-2 h-4 w-4" />
                {{ $t('forge') }}
                <template v-if="selectedForges.length > 0">
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  <span class="mx-2" aria-hidden="true">|</span>
                  <Badge variant="secondary" class="rounded-sm px-1 font-normal">
                    {{ selectedForges.length }}
                  </Badge>
                </template>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent class="w-[220px]" align="start">
              <div class="max-h-[300px] overflow-y-auto">
                <DropdownMenuCheckboxItem
                  v-for="forge in uniqueForgeNames"
                  :key="forge"
                  :model-value="selectedForges.includes(forge)"
                  @update:model-value="(value) => toggleForgeFilter(forge, !!value)"
                >
                  <div class="flex items-center gap-2">
                    <Icon :name="getForgeIconForName(forge)" class="h-4 w-4 text-muted-foreground" />
                    <span>{{ forge }}</span>
                  </div>
                </DropdownMenuCheckboxItem>
              </div>
              <template v-if="selectedForges.length > 0">
                <DropdownMenuSeparator />
                <DropdownMenuItem @click="clearForgeFilters">
                  {{ $t('clear_filters') }}
                </DropdownMenuItem>
              </template>
            </DropdownMenuContent>
          </DropdownMenu>

          <!-- Column Visibility -->
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" class="h-8">
                <Icon name="columns" class="mr-2 h-4 w-4" />
                {{ $t('columns') }}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              <DropdownMenuCheckboxItem
                v-for="column in table.getAllColumns().filter((col) => col.getCanHide())"
                :key="column.id"
                class="capitalize"
                :model-value="column.getIsVisible()"
                @update:model-value="(value) => column.toggleVisibility(!!value)"
              >
                {{ column.id }}
              </DropdownMenuCheckboxItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <!-- Bulk Actions -->
        <div v-if="table.getFilteredSelectedRowModel().rows.length > 0" class="flex items-center gap-2">
          <span class="text-sm text-muted-foreground">
            {{ table.getFilteredSelectedRowModel().rows.length }} {{ $t('selected') }}
          </span>
          <Button variant="destructive" size="sm" @click="handleBulkDelete">
            <Icon name="trash" class="h-4 w-4 mr-2" />
            {{ $t('delete') }}
          </Button>
        </div>
      </div>

      <!-- Table -->
      <div v-if="secrets && secrets.length > 0" class="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow v-for="headerGroup in table.getHeaderGroups()" :key="headerGroup.id">
              <TableHead
                v-for="header in headerGroup.headers"
                :key="header.id"
                :style="{
                  width: `${header.getSize()}px`,
                  minWidth: `${header.column.columnDef.minSize || header.getSize()}px`,
                  position: 'relative',
                }"
                :class="{ 'pr-3': header.column.getCanResize() }"
              >
                <div :class="{ 'overflow-hidden pr-2': header.column.getCanResize() }">
                  <FlexRender
                    v-if="!header.isPlaceholder"
                    :render="header.column.columnDef.header"
                    :props="header.getContext()"
                  />
                </div>
                <div
                  v-if="header.column.getCanResize()"
                  class="absolute right-0 top-0 h-full w-1 cursor-col-resize select-none touch-none bg-border hover:bg-primary"
                  :class="{
                    'bg-primary': header.column.getIsResizing(),
                  }"
                  :onMousedown="header.getResizeHandler()"
                  :onTouchstart="header.getResizeHandler()"
                />
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <template v-if="table.getRowModel().rows?.length">
              <TableRow
                v-for="row in table.getRowModel().rows"
                :key="row.id"
                :data-state="row.getIsSelected() && 'selected'"
              >
                <TableCell
                  v-for="cell in row.getVisibleCells()"
                  :key="cell.id"
                  :style="{
                    width: `${cell.column.getSize()}px`,
                    minWidth: `${cell.column.columnDef.minSize || cell.column.getSize()}px`,
                  }"
                >
                  <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
                </TableCell>
              </TableRow>
            </template>
            <TableRow v-else>
              <TableCell :colspan="columns.length" class="h-24 text-center">
                {{ $t('secrets.none') }}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <!-- Empty State -->
      <div v-else-if="secrets && secrets.length === 0" class="text-center py-12 text-muted-foreground">
        <Icon name="secret" class="h-12 w-12 mx-auto mb-3 opacity-50" />
        <p class="text-base">{{ $t('secrets.none') }}</p>
      </div>

      <!-- Loading State -->
      <div v-else class="text-center py-12 text-muted-foreground">
        <Icon name="spinner" class="h-12 w-12 mx-auto mb-3" />
        <p class="text-base">{{ $t('loading') }}</p>
      </div>
    </div>

    <!-- Bulk Delete Confirmation Dialog -->
    <AlertDialog v-model:open="showBulkDeleteDialog">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('secrets.delete_confirm_title') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('secrets.delete_bulk_confirm', { count: table.getFilteredSelectedRowModel().rows.length }) }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="confirmBulkDelete">
            {{ $t('delete') }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <!-- Single Delete Confirmation Dialog -->
    <AlertDialog v-model:open="showSingleDeleteDialog">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('secrets.delete_confirm_title') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('secrets.delete_confirm', { name: secretToDelete?.name || '' }) }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="confirmSingleDelete">
            {{ $t('delete') }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>

<script lang="ts" setup>
import type {
  ColumnDef,
  ColumnFiltersState,
  ColumnSizingState,
  SortingState,
  VisibilityState,
} from '@tanstack/vue-table';
import {
  FlexRender,
  getCoreRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  getSortedRowModel,
  useVueTable,
} from '@tanstack/vue-table';
import { useStorage } from '@vueuse/core';
import { ArrowUpDown, Check, Copy, MoreVertical } from 'lucide-vue-next';
import { computed, defineComponent, h, onMounted, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '~/components/ui/alert-dialog';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import SearchInput from '~/components/ui/input/SearchInput.vue';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import type { IconNames } from '~/components/atomic/Icon.types';
import useApiClient from '~/compositions/useApiClient';
import useConfig from '~/compositions/useConfig';
import { useForges } from '~/compositions/useForges';
import useNotifications from '~/compositions/useNotifications';
import type { SecretWithOrg } from '~/lib/api/types';
import { WebhookEvents } from '~/lib/api/types';

const props = defineProps<{
  secrets: SecretWithOrg[] | null;
  isDeleting: boolean;
  searchTerm: string;
}>();

const emit = defineEmits<{
  (event: 'edit', secret: SecretWithOrg): void;
  (event: 'delete', secret: SecretWithOrg): void;
  (event: 'bulkDelete', secrets: SecretWithOrg[]): void;
  (event: 'update:searchTerm', value: string): void;
  (event: 'updateEvents', secret: SecretWithOrg, events: WebhookEvents[]): void;
}>();

const { t } = useI18n();
const { user } = useConfig();
const apiClient = useApiClient();
const notifications = useNotifications();
const { forges } = useForges();

// Show forge column only when multiple forges are configured
const showForgeColumn = computed(() => forges.value.length > 1);

// Helper to check if a string is a valid URL
function isValidUrl(str: string): boolean {
  try {
    return Boolean(new URL(str));
  } catch {
    return false;
  }
}

// Helper to get forge icon name from secret's forge info
function getForgeIconName(forgeIcon?: string, forgeType?: string, forgeName?: string): IconNames {
  // Check for custom icon first
  if (forgeIcon && !isValidUrl(forgeIcon)) {
    const customIcon = forgeIcon.toLowerCase();
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

  // Check forgeName for special hosts
  if (forgeName) {
    const lowerName = forgeName.toLowerCase();
    if (lowerName.includes('codeberg')) return 'codeberg';
    if (lowerName.includes('codefloe')) return 'codefloe';
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
  return typeToIcon[forgeType || ''] || 'repo';
}

// Helper to get custom icon URL if forge_icon is a URL
function getForgeCustomIconUrl(forgeIcon?: string): string | null {
  if (forgeIcon && isValidUrl(forgeIcon)) {
    return forgeIcon;
  }
  return null;
}

// Track copied state for secret names
const copiedSecrets = reactive<Record<string, boolean>>({});

async function copySecretName(name: string) {
  try {
    await navigator.clipboard.writeText(name);
    copiedSecrets[name] = true;
    setTimeout(() => {
      copiedSecrets[name] = false;
    }, 2000);
  } catch {
    // Failed to copy
  }
}

const sorting = ref<SortingState>([
  { id: 'owner', desc: false },
  { id: 'name', desc: false },
]);
const columnFilters = ref<ColumnFiltersState>([]);

// Load column sizing from localStorage
const COLUMN_SIZING_STORAGE_KEY = 'crow-secrets-table-column-sizing';
const loadColumnSizing = (): ColumnSizingState => {
  try {
    const saved = localStorage.getItem(COLUMN_SIZING_STORAGE_KEY);
    return saved ? JSON.parse(saved) : {};
  } catch {
    return {};
  }
};

const columnSizing = ref<ColumnSizingState>(loadColumnSizing());

// Save column sizing to localStorage whenever it changes
watch(
  columnSizing,
  (newSizing) => {
    try {
      localStorage.setItem(COLUMN_SIZING_STORAGE_KEY, JSON.stringify(newSizing));
    } catch (error) {
      console.error('Failed to save column sizing to localStorage:', error);
    }
  },
  { deep: true },
);

const rowSelection = ref({});
const selectedScopes = useStorage<string[]>('crow:secrets-filter-scopes', []);
const selectedOwners = useStorage<string[]>('crow:secrets-filter-owners', []);
const selectedForges = useStorage<string[]>('crow:secrets-filter-forges', []);
const columnVisibility = ref<VisibilityState>({
  source: false, // Hide source column by default
  created_at: false, // Hide added column by default
  updated_at: false, // Hide last edited column by default
  forge: false, // Forge column visibility controlled by showForgeColumn
});

// Update forge column visibility when forges are loaded
watch(
  showForgeColumn,
  (show) => {
    columnVisibility.value = { ...columnVisibility.value, forge: show };
  },
  { immediate: true },
);

// Helper to get forge icon name from forge hostname
function getForgeIconForName(forgeName: string): IconNames {
  // Check if any forge in the list matches this name and has icon info
  if (props.secrets) {
    const secret = props.secrets.find((s) => s.forge_name === forgeName);
    if (secret) {
      return getForgeIconName(secret.forge_icon, secret.forge_type, secret.forge_name);
    }
  }
  return 'repo';
}

// Alert dialog state
const showBulkDeleteDialog = ref(false);
const showSingleDeleteDialog = ref(false);
const secretToDelete = ref<SecretWithOrg | null>(null);

function scopeLabel(scope: string) {
  const scopeMap: Record<string, string> = {
    user: t('scope.user'),
    org: t('scope.org'),
    repo: t('scope.repo'),
    global: t('scope.global'),
  };
  return scopeMap[scope] || scope;
}

// Available webhook events
const allWebhookEvents = [
  { value: WebhookEvents.Push, label: 'push' },
  { value: WebhookEvents.Tag, label: 'tag' },
  { value: WebhookEvents.Release, label: 'release' },
  { value: WebhookEvents.PullRequest, label: 'pull_request' },
  { value: WebhookEvents.Deploy, label: 'deploy' },
  { value: WebhookEvents.Cron, label: 'cron' },
  { value: WebhookEvents.Manual, label: 'manual' },
];

function canEdit(secret: SecretWithOrg): boolean {
  if (secret.scope_type === 'user') return true;
  if (secret.scope_type === 'global') return user?.admin === true;
  if (secret.scope_type === 'repo') return true; // Assume user has access if they can see it
  return secret.scope_type === 'org';
}

function formatTimestamp(timestamp: number): string {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
}

function handleBulkDelete() {
  showBulkDeleteDialog.value = true;
}

function confirmBulkDelete() {
  // eslint-disable-next-line ts/no-use-before-define
  const selected = table.getFilteredSelectedRowModel().rows.map((row) => row.original);
  const deletable = selected.filter((secret) => canEdit(secret));
  emit('bulkDelete', deletable);
  // eslint-disable-next-line ts/no-use-before-define
  table.resetRowSelection();
  showBulkDeleteDialog.value = false;
}

function handleSingleDelete(secret: SecretWithOrg) {
  secretToDelete.value = secret;
  showSingleDeleteDialog.value = true;
}

function confirmSingleDelete() {
  if (secretToDelete.value) {
    emit('delete', secretToDelete.value);
  }
  secretToDelete.value = null;
  showSingleDeleteDialog.value = false;
}

// Inline events editor component (tightly coupled to this view)
// eslint-disable-next-line vue/one-component-per-file
const EventsEditor = defineComponent({
  name: 'EventsEditor',
  props: {
    secret: {
      type: Object as () => SecretWithOrg,
      required: true,
    },
  },
  emits: ['update'],
  setup(props, { emit: emitLocal }) {
    const open = ref(false);
    const selectedEvents = ref<string[]>([]);

    // Current events from the secret
    const currentEvents = computed(() => (props.secret.events || []).map((e) => e as string));

    // Show only events not already assigned
    const availableEvents = computed(() => {
      return allWebhookEvents.filter((e) => !currentEvents.value.includes(e.value as string));
    });

    // Apply selected events
    function applySelection() {
      if (selectedEvents.value.length > 0) {
        const newEvents = [...currentEvents.value, ...selectedEvents.value];
        emitLocal('update', props.secret, newEvents as WebhookEvents[]);
        selectedEvents.value = [];
      }
      open.value = false;
    }

    // Remove an event tag
    function removeEvent(eventValue: string) {
      const newEvents = currentEvents.value.filter((e) => e !== eventValue);
      emitLocal('update', props.secret, newEvents as WebhookEvents[]);
    }

    return () =>
      h('div', { class: 'flex flex-wrap gap-1 items-center' }, [
        // Existing event tags
        ...currentEvents.value.map((event: string) =>
          h(
            'span',
            {
              key: event,
              class:
                'inline-flex items-center gap-1 h-5 px-2 rounded-md border border-muted-foreground/25 bg-muted text-xs',
            },
            [
              event,
              h(
                'button',
                {
                  type: 'button',
                  class:
                    'ml-1 hover:bg-muted-foreground/20 rounded-full w-3 h-3 inline-flex items-center justify-center',
                  onClick: (e: Event) => {
                    e.stopPropagation();
                    removeEvent(event);
                  },
                },
                '×',
              ),
            ],
          ),
        ),
        // Add button with popover
        availableEvents.value.length > 0
          ? h(
              Popover,
              {
                open: open.value,
                'onUpdate:open': (value: boolean) => {
                  if (!value) {
                    selectedEvents.value = [];
                  }
                  open.value = value;
                },
              },
              {
                default: () => [
                  h(
                    PopoverTrigger,
                    { asChild: true },
                    {
                      default: () =>
                        h(
                          'button',
                          {
                            type: 'button',
                            class:
                              'inline-flex items-center justify-center h-5 w-5 rounded-md border border-dashed border-input hover:border-ring hover:bg-muted/50 text-xs text-muted-foreground',
                          },
                          '+',
                        ),
                    },
                  ),
                  h(
                    PopoverContent,
                    { class: 'w-auto p-3', align: 'start', side: 'bottom' },
                    {
                      default: () =>
                        h('div', { class: 'space-y-2' }, [
                          h(
                            'div',
                            { class: 'space-y-2' },
                            availableEvents.value.map((event) =>
                              h('div', { key: event.value, class: 'flex items-center gap-2' }, [
                                h(Checkbox, {
                                  id: `event-${event.value}-${props.secret.id}`,
                                  checked: selectedEvents.value.includes(event.value as string),
                                  'onUpdate:checked': (checked: boolean | 'indeterminate') => {
                                    const isChecked = checked === true;
                                    if (isChecked) {
                                      selectedEvents.value.push(event.value as string);
                                    } else {
                                      const index = selectedEvents.value.indexOf(event.value as string);
                                      if (index > -1) {
                                        selectedEvents.value.splice(index, 1);
                                      }
                                    }
                                  },
                                }),
                                h(
                                  'label',
                                  {
                                    for: `event-${event.value}-${props.secret.id}`,
                                    class: 'text-sm cursor-pointer',
                                  },
                                  event.label,
                                ),
                              ]),
                            ),
                          ),
                          selectedEvents.value.length > 0
                            ? h(
                                'div',
                                { class: 'pt-2 border-t' },
                                h(
                                  Button,
                                  {
                                    size: 'sm',
                                    class: 'w-full',
                                    onClick: applySelection,
                                  },
                                  () =>
                                    `Add ${selectedEvents.value.length} event${selectedEvents.value.length > 1 ? 's' : ''}`,
                                ),
                              )
                            : null,
                        ]),
                    },
                  ),
                ],
              },
            )
          : currentEvents.value.length === 0
            ? h('span', { class: 'text-xs text-muted-foreground' }, 'No events')
            : null,
      ]);
  },
});

// Inline secret value viewer component (tightly coupled to this view)
// eslint-disable-next-line vue/one-component-per-file
const SecretValueViewer = defineComponent({
  name: 'SecretValueViewer',
  props: {
    secret: {
      type: Object as () => SecretWithOrg,
      required: true,
    },
    canView: {
      type: Boolean,
      required: true,
    },
  },
  setup(props) {
    const showValue = ref(false);
    const isLoading = ref(false);
    const abbreviatedValue = ref<string | null>(null);
    const hasError = ref(false);

    async function toggleValue() {
      if (!props.canView) return;

      if (!showValue.value) {
        // Fetch abbreviated value
        isLoading.value = true;
        hasError.value = false;
        try {
          let result;
          if (props.secret.scope_type === 'repo' && props.secret.repo_id) {
            result = await apiClient.getSecretAbbreviated(props.secret.repo_id, props.secret.name);
          } else if ((props.secret.scope_type === 'org' || props.secret.scope_type === 'user') && props.secret.org_id) {
            result = await apiClient.getOrgSecretAbbreviated(props.secret.org_id, props.secret.name);
          } else if (props.secret.scope_type === 'global') {
            result = await apiClient.getGlobalSecretAbbreviated(props.secret.name);
          } else {
            hasError.value = true;
            return;
          }

          if (result?.value) {
            abbreviatedValue.value = result.value;
            showValue.value = true;
          }
        } catch {
          hasError.value = true;
          notifications.notify({
            title: t('notify_title_error'),
            text: t('secrets.error_viewing_value'),
            type: 'error',
          });
        } finally {
          isLoading.value = false;
        }
      } else {
        showValue.value = false;
      }
    }

    return () =>
      h('div', { class: 'flex items-center gap-2' }, [
        showValue.value && abbreviatedValue.value
          ? h('code', { class: 'text-xs font-mono bg-muted px-1 py-0.5 rounded' }, abbreviatedValue.value)
          : h('span', { class: 'text-muted-foreground text-xs' }, '••••••••'),
        props.canView
          ? h(
              'button',
              {
                type: 'button',
                class: `hover:text-primary transition-colors ${hasError.value ? 'text-destructive' : ''}`,
                onClick: toggleValue,
                disabled: isLoading.value,
                title: showValue.value ? t('secrets.hide_value') : t('secrets.show_value'),
              },
              isLoading.value
                ? h(Icon, { name: 'spinner', class: 'h-4 w-4 animate-spin' })
                : h(Icon, { name: showValue.value ? 'lucide:eye-off' : 'lucide:eye', class: 'h-4 w-4' }),
            )
          : null,
      ]);
  },
});

const columns: ColumnDef<SecretWithOrg>[] = [
  {
    id: 'select',
    header: ({ table }) =>
      h(
        'div',
        { class: 'ml-3' },
        h(Checkbox, {
          checked: table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate'),
          'onUpdate:checked': (value: boolean | 'indeterminate') => table.toggleAllPageRowsSelected(!!value),
          ariaLabel: 'Select all',
        }),
      ),
    cell: ({ row }) =>
      h(Checkbox, {
        checked: row.getIsSelected(),
        'onUpdate:checked': (value: boolean | 'indeterminate') => row.toggleSelected(!!value),
        ariaLabel: 'Select row',
        disabled: !canEdit(row.original),
      }),
    enableSorting: false,
    enableHiding: false,
    enableResizing: false,
    size: 20,
    minSize: 20,
    maxSize: 20,
  },
  {
    accessorKey: 'name',
    header: ({ column }) => {
      return h(
        Button,
        {
          variant: 'ghost',
          class: ' pr-6',
          onClick: () => column.toggleSorting(column.getIsSorted() === 'asc'),
        },
        () => ['Name', h(ArrowUpDown, { class: 'ml-2 h-4 w-4' })],
      );
    },
    cell: ({ row }) => {
      const secret = row.original;
      const name = row.getValue('name') as string;
      return h('div', { class: 'flex items-center gap-1' }, [
        h(
          'button',
          {
            class: 'font-medium text-link hover:underline cursor-pointer',
            onClick: () => emit('edit', secret),
          },
          name,
        ),
        h(
          'button',
          {
            type: 'button',
            class: 'p-1 hover:bg-muted rounded transition-colors',
            title: copiedSecrets[name] ? t('copied') : t('copy'),
            onClick: (e: Event) => {
              e.stopPropagation();
              copySecretName(name);
            },
          },
          h(copiedSecrets[name] ? Check : Copy, { class: 'h-3 w-3 text-muted-foreground' }),
        ),
      ]);
    },
    size: 200,
    minSize: 60,
  },
  {
    id: 'value',
    header: () => h('div', { class: 'ml-3' }, t('secrets.value')),
    cell: ({ row }) => {
      const secret = row.original;
      // Determine if user can view the abbreviated value
      // User secrets: always viewable by owner
      // Org secrets: viewable by org admins
      // Repo secrets: viewable by users with push access (handled by API)
      // Global secrets: viewable by instance admins
      const canViewValue = canEdit(secret);

      return h(SecretValueViewer, {
        secret,
        canView: canViewValue,
      });
    },
    enableSorting: false,
    enableHiding: true,
    size: 180,
    minSize: 120,
  },
  {
    id: 'owner',
    accessorFn: (row) => {
      return row.scope_type === 'repo' && row.repo_name ? row.repo_name : row.org_name;
    },
    header: ({ column }) => {
      return h(
        Button,
        {
          variant: 'ghost',
          class: ' pr-6',
          onClick: () => column.toggleSorting(column.getIsSorted() === 'asc'),
        },
        () => ['Owner', h(ArrowUpDown, { class: 'ml-2 h-4 w-4' })],
      );
    },
    cell: ({ row }) => {
      const secret = row.original;
      const ownerName = secret.scope_type === 'repo' && secret.repo_name ? secret.repo_name : secret.org_name;

      // For repo-level secrets, link to the repo page
      if (secret.scope_type === 'repo' && secret.repo_name) {
        return h(
          'a',
          {
            href: `/${secret.repo_name}`,
            class: 'text-link hover:underline',
          },
          ownerName,
        );
      }

      // For org-level secrets, link to the org page
      if (secret.scope_type === 'org' && secret.org_id) {
        return h(
          'a',
          {
            href: `/orgs/${secret.org_id}`,
            class: 'text-link hover:underline',
          },
          ownerName,
        );
      }

      // For user and global secrets, just display the text
      return h('div', {}, ownerName);
    },
    filterFn: (row, id, value) => {
      return value.includes(row.getValue(id));
    },
    size: 180,
    minSize: 80,
  },
  {
    id: 'forge',
    accessorKey: 'forge_name',
    header: () => h('div', { class: 'ml-3' }, t('forge')),
    cell: ({ row }) => {
      const secret = row.original;
      if (!secret.forge_name) {
        return h('span', { class: 'text-muted-foreground' }, '-');
      }

      const customIconUrl = getForgeCustomIconUrl(secret.forge_icon);
      const iconName = getForgeIconName(secret.forge_icon, secret.forge_type, secret.forge_name);

      return h('div', { class: 'flex items-center gap-1.5', title: secret.forge_name }, [
        customIconUrl
          ? h('img', { src: customIconUrl, class: 'h-4 w-4', alt: '' })
          : h(Icon, { name: iconName, class: 'h-4 w-4 shrink-0 text-muted-foreground' }),
        h('span', { class: 'text-sm text-muted-foreground truncate' }, secret.forge_name),
      ]);
    },
    filterFn: (row, id, value) => {
      return value.includes(row.getValue(id));
    },
    enableSorting: false,
    size: 150,
    minSize: 100,
  },
  {
    accessorKey: 'scope_type',
    id: 'scope',
    header: ({ column }) => {
      return h(
        Button,
        {
          variant: 'ghost',
          class: ' pr-6',
          onClick: () => column.toggleSorting(column.getIsSorted() === 'asc'),
        },
        () => [t('scope_type'), h(ArrowUpDown, { class: 'ml-2 h-4 w-4' })],
      );
    },
    cell: ({ row }) => {
      const secret = row.original;
      return h('div', { class: 'flex items-center gap-2' }, [
        h('span', {}, scopeLabel(secret.scope_type)),
        canEdit(secret) ? null : h(Badge, { variant: 'secondary' as const }, () => t('readonly')),
      ]);
    },
    filterFn: (row, id, value) => {
      return value.includes(row.getValue(id));
    },
    size: 60,
    minSize: 60,
    maxSize: 60,
    enableResizing: false,
  },
  {
    accessorKey: 'events',
    header: () => h('div', { class: 'ml-3' }, 'Events'),
    cell: ({ row }) => {
      const secret = row.original;
      if (!canEdit(secret)) {
        return h(
          'div',
          { class: 'flex flex-wrap gap-1' },
          (secret.events || []).map((event: WebhookEvents) =>
            h(Badge, { variant: 'outline' as const, class: 'text-xs' }, () => event),
          ),
        );
      }

      return h(EventsEditor, {
        secret,
        onUpdate: (secret: SecretWithOrg, events: WebhookEvents[]) => {
          emit('updateEvents', secret, events);
        },
      });
    },
    enableSorting: false,
    size: 300,
    minSize: 300,
  },
  {
    accessorKey: 'source',
    header: () => 'Source',
    cell: ({ row }) => {
      const secret = row.original;
      return h('div', { class: 'capitalize' }, secret.source || 'internal');
    },
    size: 100,
    minSize: 80,
  },
  {
    accessorKey: 'created_at',
    header: ({ column }) => {
      return h(
        Button,
        {
          variant: 'ghost' as const,
          class: ' pr-6',
          onClick: () => column.toggleSorting(column.getIsSorted() === 'asc'),
        },
        () => [t('secrets.added'), h(ArrowUpDown, { class: 'ml-2 h-4 w-4' })],
      );
    },
    cell: ({ row }) => {
      const secret = row.original;
      return h('div', {}, formatTimestamp(secret.created_at));
    },
    size: 170,
    minSize: 140,
  },
  {
    accessorKey: 'updated_at',
    header: ({ column }) => {
      return h(
        Button,
        {
          variant: 'ghost' as const,
          class: ' pr-6',
          onClick: () => column.toggleSorting(column.getIsSorted() === 'asc'),
        },
        () => [t('secrets.last_edited'), h(ArrowUpDown, { class: 'ml-2 h-4 w-4' })],
      );
    },
    cell: ({ row }) => {
      const secret = row.original;
      return h('div', {}, formatTimestamp(secret.updated_at));
    },
    size: 170,
    minSize: 140,
  },
  {
    id: 'actions',
    enableHiding: false,
    enableResizing: false,
    size: 10,
    cell: ({ row }) => {
      const secret = row.original;
      if (!canEdit(secret)) {
        return null;
      }

      return h(
        'div',
        { class: 'flex items-center justify-end' },
        h(
          DropdownMenu,
          {},
          {
            default: () => [
              h(
                DropdownMenuTrigger,
                { asChild: true },
                {
                  default: () =>
                    h(
                      Button,
                      {
                        variant: 'ghost' as const,
                        class: 'h-8 w-8 p-0',
                      },
                      () => [h('span', { class: 'sr-only' }, t('actions')), h(MoreVertical, { class: 'h-4 w-4' })],
                    ),
                },
              ),
              h(
                DropdownMenuContent,
                { align: 'end' },
                {
                  default: () => [
                    h(
                      DropdownMenuItem,
                      {
                        onClick: () => emit('edit', secret),
                      },
                      () => [h(Icon, { name: 'edit', class: 'mr-2 h-4 w-4' }), t('secrets.edit')],
                    ),
                    h(
                      DropdownMenuItem,
                      {
                        class: 'text-destructive',
                        onClick: () => handleSingleDelete(secret),
                      },
                      () => [h(Icon, { name: 'trash', class: 'mr-2 h-4 w-4' }), t('secrets.delete')],
                    ),
                  ],
                },
              ),
            ],
          },
        ),
      );
    },
  },
];

const table = useVueTable({
  get data() {
    return props.secrets || [];
  },
  columns,
  getCoreRowModel: getCoreRowModel(),
  getSortedRowModel: getSortedRowModel(),
  getFilteredRowModel: getFilteredRowModel(),
  getFacetedUniqueValues: getFacetedUniqueValues(),
  enableColumnResizing: true,
  columnResizeMode: 'onChange',
  onSortingChange: (updaterOrValue) => {
    sorting.value = typeof updaterOrValue === 'function' ? updaterOrValue(sorting.value) : updaterOrValue;
  },
  onColumnFiltersChange: (updaterOrValue) => {
    columnFilters.value = typeof updaterOrValue === 'function' ? updaterOrValue(columnFilters.value) : updaterOrValue;
  },
  onColumnSizingChange: (updaterOrValue) => {
    columnSizing.value = typeof updaterOrValue === 'function' ? updaterOrValue(columnSizing.value) : updaterOrValue;
  },
  onRowSelectionChange: (updaterOrValue) => {
    rowSelection.value = typeof updaterOrValue === 'function' ? updaterOrValue(rowSelection.value) : updaterOrValue;
  },
  onColumnVisibilityChange: (updaterOrValue) => {
    columnVisibility.value =
      typeof updaterOrValue === 'function' ? updaterOrValue(columnVisibility.value) : updaterOrValue;
  },
  state: {
    get sorting() {
      return sorting.value;
    },
    get columnFilters() {
      return columnFilters.value;
    },
    get columnSizing() {
      return columnSizing.value;
    },
    get rowSelection() {
      return rowSelection.value;
    },
    get columnVisibility() {
      return columnVisibility.value;
    },
  },
});

// Map scope types to display labels
const scopeLabelMap: Record<string, string[]> = {
  Organization: ['user', 'org'],
  Repository: ['repo'],
  Global: ['global'],
};

// Array of selected labels (persisted to localStorage)
const selectedScopeLabels = useStorage<string[]>('crow:secrets-filter-scope-labels', []);

// Reverse mapping: scope type to label
const scopeToLabel: Record<string, string> = {
  user: 'Organization',
  org: 'Organization',
  repo: 'Repository',
  global: 'Global',
};

const uniqueScopes = computed(() => {
  const scopeColumn = table.getColumn('scope');
  if (!scopeColumn) return [];

  const facetedValues = scopeColumn.getFacetedUniqueValues();
  const scopeTypes = [...facetedValues.keys()];

  // Get unique labels
  const labels = new Set<string>();
  scopeTypes.forEach((scopeType) => {
    const label = scopeToLabel[scopeType];
    if (label) labels.add(label);
  });

  return [...labels].toSorted();
});

const uniqueOwners = computed(() => {
  const ownerColumn = table.getColumn('owner');
  if (!ownerColumn) return [];
  const facetedValues = ownerColumn.getFacetedUniqueValues();
  return [...facetedValues.keys()].toSorted();
});

const uniqueForgeNames = computed(() => {
  const forgeColumn = table.getColumn('forge');
  if (!forgeColumn) return [];
  const facetedValues = forgeColumn.getFacetedUniqueValues();
  // Filter out empty/undefined forge names
  return [...facetedValues.keys()].filter((name) => name && name.trim() !== '').sort();
});

const selectedScopeLabelsCount = computed(() => {
  return selectedScopeLabels.value.length;
});

function toggleScopeLabel(label: string, checked: boolean) {
  if (checked) {
    if (!selectedScopeLabels.value.includes(label)) {
      selectedScopeLabels.value.push(label);
    }
  } else {
    const index = selectedScopeLabels.value.indexOf(label);
    if (index > -1) {
      selectedScopeLabels.value.splice(index, 1);
    }
  }

  // Convert labels to scope types
  const newSelectedScopes: string[] = [];
  selectedScopeLabels.value.forEach((label) => {
    const scopeTypes = scopeLabelMap[label] || [];
    newSelectedScopes.push(...scopeTypes);
  });

  selectedScopes.value = newSelectedScopes;

  if (newSelectedScopes.length > 0) {
    table.getColumn('scope')?.setFilterValue(newSelectedScopes);
  } else {
    table.getColumn('scope')?.setFilterValue(undefined);
  }
}

function clearScopeFilters() {
  selectedScopeLabels.value = [];
  selectedScopes.value = [];
  table.getColumn('scope')?.setFilterValue(undefined);
}

function toggleOwnerFilter(owner: string, checked: boolean) {
  if (checked) {
    if (!selectedOwners.value.includes(owner)) {
      selectedOwners.value.push(owner);
    }
  } else {
    const index = selectedOwners.value.indexOf(owner);
    if (index > -1) {
      selectedOwners.value.splice(index, 1);
    }
  }

  if (selectedOwners.value.length > 0) {
    table.getColumn('owner')?.setFilterValue(selectedOwners.value);
  } else {
    table.getColumn('owner')?.setFilterValue(undefined);
  }
}

function clearOwnerFilters() {
  selectedOwners.value = [];
  table.getColumn('owner')?.setFilterValue(undefined);
}

function toggleForgeFilter(forge: string, checked: boolean) {
  if (checked) {
    if (!selectedForges.value.includes(forge)) {
      selectedForges.value.push(forge);
    }
  } else {
    const index = selectedForges.value.indexOf(forge);
    if (index > -1) {
      selectedForges.value.splice(index, 1);
    }
  }

  if (selectedForges.value.length > 0) {
    table.getColumn('forge')?.setFilterValue(selectedForges.value);
  } else {
    table.getColumn('forge')?.setFilterValue(undefined);
  }
}

function clearForgeFilters() {
  selectedForges.value = [];
  table.getColumn('forge')?.setFilterValue(undefined);
}

// Apply persisted filters on mount
onMounted(() => {
  // Apply scope filters
  if (selectedScopeLabels.value.length > 0) {
    const scopeTypes: string[] = [];
    selectedScopeLabels.value.forEach((label) => {
      const types = scopeLabelMap[label] || [];
      scopeTypes.push(...types);
    });
    if (scopeTypes.length > 0) {
      selectedScopes.value = scopeTypes;
      table.getColumn('scope')?.setFilterValue(scopeTypes);
    }
  }

  // Apply owner filters
  if (selectedOwners.value.length > 0) {
    table.getColumn('owner')?.setFilterValue(selectedOwners.value);
  }

  // Apply forge filters
  if (selectedForges.value.length > 0) {
    table.getColumn('forge')?.setFilterValue(selectedForges.value);
  }
});
</script>

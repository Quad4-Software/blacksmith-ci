<template>
  <div class="space-y-3">
    <div class="space-y-4">
      <!-- Toolbar with Search, Filters and Bulk Actions -->
      <div class="flex items-center justify-center gap-2 flex-wrap">
        <div class="flex items-center gap-2 flex-wrap">
          <!-- Search -->
          <SearchInput
            :model-value="props.searchTerm"
            :placeholder="$t('search')"
            class="w-[300px] min-w-[80px]"
            @update:model-value="(value) => $emit('update:searchTerm', value as string)"
          />

          <!-- Owner Filter -->
          <DropdownMenu v-if="owners.length > 1" :modal="false">
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" class="h-8 border-dashed">
                <Icon name="filter" class="mr-2 h-4 w-4" />
                {{ $t('owner') }}
                <template v-if="selectedOwners.length > 0">
                  <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                  <span class="mx-2" aria-hidden="true">|</span>
                  <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                  <Badge variant="secondary" class="rounded-sm px-1 font-normal">
                    {{ selectedOwners.length }}
                  </Badge>
                </template>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent class="w-[200px]" align="start">
              <div class="max-h-[300px] overflow-y-auto">
                <DropdownMenuCheckboxItem
                  v-for="owner in owners"
                  :key="owner"
                  :model-value="selectedOwners.includes(owner)"
                  @update:model-value="(value) => toggleOwner(owner, !!value)"
                >
                  {{ owner }}
                </DropdownMenuCheckboxItem>
              </div>
              <template v-if="selectedOwners.length > 0">
                <DropdownMenuSeparator />
                <DropdownMenuItem @click="clearOwnerFilter">
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
          <Button variant="outline" size="sm" @click="handleBulkDisable">
            <Icon name="lucide:power" class="h-4 w-4 mr-2" />
            {{ $t('admin.settings.repos.disable') }}
          </Button>
          <Button variant="destructive" size="sm" @click="handleBulkDelete">
            <Icon name="trash" class="h-4 w-4 mr-2" />
            {{ $t('delete') }}
          </Button>
        </div>
      </div>

      <!-- Table -->
      <div v-if="repos && repos.length > 0" class="rounded-md border">
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
                {{ $t('admin.settings.repos.none') }}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <!-- Empty State -->
      <div v-else-if="repos && repos.length === 0" class="text-center py-12 text-muted-foreground">
        <Icon name="repo" class="h-12 w-12 mx-auto mb-3 opacity-50" />
        <p class="text-base">{{ $t('admin.settings.repos.none') }}</p>
      </div>

      <!-- Loading State -->
      <div v-else class="text-center py-12 text-muted-foreground">
        <Icon name="spinner" class="h-12 w-12 mx-auto mb-3 animate-spin" />
        <p class="text-base">{{ $t('loading') }}</p>
      </div>
    </div>

    <!-- Bulk Disable Confirmation Dialog -->
    <AlertDialog v-model:open="showBulkDisableDialog">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('admin.settings.repos.disable_confirm_title') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{
              $t('admin.settings.repos.disable_bulk_confirm', {
                count: table.getFilteredSelectedRowModel().rows.length,
              })
            }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="confirmBulkDisable">
            {{ $t('admin.settings.repos.disable') }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <!-- Bulk Delete Confirmation Dialog -->
    <AlertDialog v-model:open="showBulkDeleteDialog">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('admin.settings.repos.delete_confirm_title') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{
              $t('admin.settings.repos.delete_bulk_confirm', {
                count: table.getFilteredSelectedRowModel().rows.length,
              })
            }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
          <AlertDialogAction variant="destructive" @click="confirmBulkDelete">
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
  RowSelectionState,
  SortingState,
  VisibilityState,
} from '@tanstack/vue-table';
import { FlexRender, getCoreRowModel, getFilteredRowModel, getSortedRowModel, useVueTable } from '@tanstack/vue-table';
import { useStorage } from '@vueuse/core';
import { ArrowUpDown, Check, Copy, MoreVertical } from 'lucide-vue-next';
import { computed, h, onMounted, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import { useForges } from '~/compositions/useForges';
import type { Forge, Repo } from '~/lib/api/types';
import type { IconNames } from '~/components/atomic/Icon.types';

const props = defineProps<{
  repos: Repo[] | null;
  searchTerm: string;
}>();

const emit = defineEmits<{
  (event: 'update:searchTerm', value: string): void;
  (event: 'bulkDisable', repos: Repo[]): void;
  (event: 'bulkDelete', repos: Repo[]): void;
}>();

const { t } = useI18n();
const router = useRouter();
const { getForgeById } = useForges();

// Track copied state for repo names
const copiedRepos = reactive<Record<string, boolean>>({});

async function copyRepoName(name: string) {
  try {
    await navigator.clipboard.writeText(name);
    copiedRepos[name] = true;
    setTimeout(() => {
      copiedRepos[name] = false;
    }, 2000);
  } catch {
    // Failed to copy
  }
}

const sorting = ref<SortingState>([{ id: 'full_name', desc: false }]);
const columnFilters = ref<ColumnFiltersState>([]);
const columnVisibility = ref<VisibilityState>({});
const rowSelection = ref<RowSelectionState>({});

// Extract unique owners from repos
const owners = computed(() => {
  if (!props.repos) return [];
  const ownerSet = new Set(props.repos.map((repo) => repo.owner));
  return Array.from(ownerSet).sort();
});

// Load column sizing from localStorage
const COLUMN_SIZING_STORAGE_KEY = 'crow-repos-table-column-sizing';
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

const selectedOwners = useStorage<string[]>('crow:repos-filter-owners', []);

// Alert dialog state
const showBulkDisableDialog = ref(false);
const showBulkDeleteDialog = ref(false);

function getForgeForRepo(repo: Repo): Forge | null {
  return getForgeById(repo.forge_id);
}

// Map forge types to local icon names
function getForgeIconName(forge: Forge): IconNames {
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

function getForgeHostname(forge: Forge | null): string {
  if (!forge?.url) return '';
  try {
    const url = new URL(forge.url);
    return url.hostname;
  } catch {
    return forge.url;
  }
}

function handleBulkDisable() {
  showBulkDisableDialog.value = true;
}

function handleBulkDelete() {
  showBulkDeleteDialog.value = true;
}

const columns: ColumnDef<Repo>[] = [
  {
    id: 'select',
    header: ({ table }) =>
      h(
        'div',
        { class: 'ml-3' },
        h(Checkbox, {
          modelValue: table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate'),
          'onUpdate:modelValue': (value: boolean | 'indeterminate') => table.toggleAllPageRowsSelected(!!value),
          ariaLabel: 'Select all',
        }),
      ),
    cell: ({ row }) =>
      h(Checkbox, {
        modelValue: row.getIsSelected(),
        'onUpdate:modelValue': (value: boolean | 'indeterminate') => row.toggleSelected(!!value),
        ariaLabel: 'Select row',
      }),
    enableSorting: false,
    enableHiding: false,
    size: 20,
    minSize: 20,
    maxSize: 20,
  },
  {
    accessorKey: 'full_name',
    header: ({ column }) => {
      return h(
        Button,
        {
          variant: 'ghost',
          class: 'pr-6',
          onClick: () => column.toggleSorting(column.getIsSorted() === 'asc'),
        },
        () => [t('admin.settings.repos.name'), h(ArrowUpDown, { class: 'ml-2 h-4 w-4' })],
      );
    },
    cell: ({ row }) => {
      const repo = row.original;
      const fullName = repo.full_name;
      return h('div', { class: 'flex items-center gap-1' }, [
        h(
          'a',
          {
            href: `/repos/${repo.id}`,
            class: 'font-medium text-link hover:underline cursor-pointer',
            onClick: (e: Event) => {
              e.preventDefault();
              router.push({ name: 'repo', params: { repoId: repo.id } });
            },
          },
          fullName,
        ),
        h(
          'button',
          {
            type: 'button',
            class: 'p-1 hover:bg-muted rounded transition-colors',
            title: copiedRepos[fullName] ? t('copied') : t('copy'),
            onClick: (e: Event) => {
              e.stopPropagation();
              copyRepoName(fullName);
            },
          },
          h(copiedRepos[fullName] ? Check : Copy, { class: 'h-3 w-3 text-muted-foreground' }),
        ),
      ]);
    },
    size: 300,
    minSize: 150,
  },
  {
    accessorKey: 'owner',
    header: ({ column }) => {
      return h(
        Button,
        {
          variant: 'ghost',
          class: 'pr-6',
          onClick: () => column.toggleSorting(column.getIsSorted() === 'asc'),
        },
        () => [t('owner'), h(ArrowUpDown, { class: 'ml-2 h-4 w-4' })],
      );
    },
    cell: ({ row }) => h('div', {}, row.getValue('owner')),
    filterFn: (row, _id, value: string[]) => {
      if (!value || value.length === 0) return true;
      return value.includes(row.original.owner);
    },
    size: 150,
    minSize: 100,
  },
  {
    id: 'forge',
    header: () => t('admin.settings.repos.forge'),
    cell: ({ row }) => {
      const repo = row.original;
      const forge = getForgeForRepo(repo);
      if (!forge || !repo.forge_url) return null;
      return h(
        'a',
        {
          href: repo.forge_url,
          target: '_blank',
          title: `View ${repo.full_name} on ${getForgeHostname(forge)}`,
          class: 'inline-flex items-center justify-center h-6 w-6 hover:opacity-80 transition-opacity',
        },
        h(Icon, { name: getForgeIconName(forge), class: 'h-4 w-4' }),
      );
    },
    enableSorting: false,
    size: 80,
    minSize: 60,
  },
  {
    accessorKey: 'id',
    header: ({ column }) => {
      return h(
        Button,
        {
          variant: 'ghost',
          class: 'pr-6',
          onClick: () => column.toggleSorting(column.getIsSorted() === 'asc'),
        },
        () => ['ID', h(ArrowUpDown, { class: 'ml-2 h-4 w-4' })],
      );
    },
    cell: ({ row }) => h('div', {}, row.getValue('id')),
    size: 80,
    minSize: 60,
  },
  {
    id: 'actions',
    enableHiding: false,
    size: 10,
    cell: ({ row }) => {
      const repo = row.original;

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
                        onClick: () => router.push({ name: 'repo', params: { repoId: repo.id } }),
                      },
                      () => [h(Icon, { name: 'chevron-right', class: 'mr-2 h-4 w-4' }), t('admin.settings.repos.view')],
                    ),
                    h(
                      DropdownMenuItem,
                      {
                        onClick: () => router.push({ name: 'repo-settings', params: { repoId: repo.id } }),
                      },
                      () => [
                        h(Icon, { name: 'settings-outline', class: 'mr-2 h-4 w-4' }),
                        t('admin.settings.repos.settings'),
                      ],
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
    return props.repos || [];
  },
  columns,
  getCoreRowModel: getCoreRowModel(),
  getSortedRowModel: getSortedRowModel(),
  getFilteredRowModel: getFilteredRowModel(),
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
  onColumnVisibilityChange: (updaterOrValue) => {
    columnVisibility.value =
      typeof updaterOrValue === 'function' ? updaterOrValue(columnVisibility.value) : updaterOrValue;
  },
  onRowSelectionChange: (updaterOrValue) => {
    rowSelection.value = typeof updaterOrValue === 'function' ? updaterOrValue(rowSelection.value) : updaterOrValue;
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
    get columnVisibility() {
      return columnVisibility.value;
    },
    get rowSelection() {
      return rowSelection.value;
    },
  },
});

function confirmBulkDisable() {
  const selected = table.getFilteredSelectedRowModel().rows.map((row) => row.original);
  emit('bulkDisable', selected);
  table.resetRowSelection();
  showBulkDisableDialog.value = false;
}

function confirmBulkDelete() {
  const selected = table.getFilteredSelectedRowModel().rows.map((row) => row.original);
  emit('bulkDelete', selected);
  table.resetRowSelection();
  showBulkDeleteDialog.value = false;
}

function toggleOwner(owner: string, checked: boolean) {
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

function clearOwnerFilter() {
  selectedOwners.value = [];
  table.getColumn('owner')?.setFilterValue(undefined);
}

// Initialize filters from localStorage on mount
onMounted(() => {
  if (selectedOwners.value.length > 0) {
    table.getColumn('owner')?.setFilterValue(selectedOwners.value);
  }
});
</script>

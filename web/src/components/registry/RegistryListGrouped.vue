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
            class="w-[300px] min-w-[100px]"
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
                  :checked="selectedOwners.includes(owner)"
                  @select="
                    (e) => {
                      e.preventDefault();
                      toggleOwnerFilter(owner, !selectedOwners.includes(owner));
                    }
                  "
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
      <div v-if="registries && registries.length > 0" class="rounded-md border">
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
                {{ $t('registries.none') }}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <!-- Empty State -->
      <div v-else-if="registries && registries.length === 0" class="text-center py-12 text-muted-foreground">
        <Icon name="docker" class="h-12 w-12 mx-auto mb-3 opacity-50" />
        <p class="text-base">{{ $t('registries.none') }}</p>
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
          <AlertDialogTitle>{{ $t('registries.delete_confirm_title') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('registries.delete_bulk_confirm', { count: table.getFilteredSelectedRowModel().rows.length }) }}
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
          <AlertDialogTitle>{{ $t('registries.delete_confirm_title') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('registries.delete_confirm', { name: registryToDelete?.address || '' }) }}
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
import { ArrowUpDown, MoreVertical } from 'lucide-vue-next';
import { computed, h, ref, watch } from 'vue';
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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import useConfig from '~/compositions/useConfig';
import type { RegistryWithOrg } from '~/lib/api/types';

const props = defineProps<{
  registries: RegistryWithOrg[] | null;
  isDeleting: boolean;
  searchTerm: string;
}>();

const emit = defineEmits<{
  (event: 'edit', registry: RegistryWithOrg): void;
  (event: 'delete', registry: RegistryWithOrg): void;
  (event: 'update:searchTerm', value: string): void;
}>();

const { t } = useI18n();
const { user } = useConfig();

const sorting = ref<SortingState>([
  { id: 'owner', desc: false },
  { id: 'address', desc: false },
]);
const columnFilters = ref<ColumnFiltersState>([]);

// Load column sizing from localStorage
const COLUMN_SIZING_STORAGE_KEY = 'crow-registries-table-column-sizing';
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
const selectedScopes = ref<string[]>([]);
const selectedOwners = ref<string[]>([]);
const columnVisibility = ref<VisibilityState>({
  created_at: false, // Hide added column by default
  updated_at: false, // Hide last edited column by default
});

// Alert dialog state
const showBulkDeleteDialog = ref(false);
const showSingleDeleteDialog = ref(false);
const registryToDelete = ref<RegistryWithOrg | null>(null);

function scopeLabel(scope: string) {
  const scopeMap: Record<string, string> = {
    user: t('scope.user'),
    org: t('scope.org'),
    repo: t('scope.repo'),
    global: t('scope.global'),
  };
  return scopeMap[scope] || scope;
}

function canEdit(registry: RegistryWithOrg): boolean {
  if (registry.scope_type === 'user') return true;
  if (registry.scope_type === 'global') return user?.admin === true;
  return registry.scope_type === 'org';
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
  selected.forEach((registry) => {
    if (canEdit(registry)) {
      emit('delete', registry);
    }
  });
  // eslint-disable-next-line ts/no-use-before-define
  table.resetRowSelection();
  showBulkDeleteDialog.value = false;
}

function handleSingleDelete(registry: RegistryWithOrg) {
  registryToDelete.value = registry;
  showSingleDeleteDialog.value = true;
}

function confirmSingleDelete() {
  if (registryToDelete.value) {
    emit('delete', registryToDelete.value);
  }
  registryToDelete.value = null;
  showSingleDeleteDialog.value = false;
}

const columns: ColumnDef<RegistryWithOrg>[] = [
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
    accessorKey: 'address',
    header: ({ column }) => {
      return h(
        Button,
        {
          variant: 'ghost',
          class: ' pr-6',
          onClick: () => column.toggleSorting(column.getIsSorted() === 'asc'),
        },
        () => ['Address', h(ArrowUpDown, { class: 'ml-2 h-4 w-4' })],
      );
    },
    cell: ({ row }) => {
      const registry = row.original;
      return h(
        'button',
        {
          class: 'font-medium text-link hover:underline cursor-pointer',
          onClick: () => emit('edit', registry),
        },
        row.getValue('address'),
      );
    },
    size: 250,
    minSize: 60,
  },
  {
    accessorKey: 'username',
    header: ({ column }) => {
      return h(
        Button,
        {
          variant: 'ghost',
          class: ' pr-6',
          onClick: () => column.toggleSorting(column.getIsSorted() === 'asc'),
        },
        () => ['Username', h(ArrowUpDown, { class: 'ml-2 h-4 w-4' })],
      );
    },
    cell: ({ row }) => h('div', {}, row.getValue('username')),
    size: 150,
    minSize: 100,
  },
  {
    id: 'owner',
    accessorFn: (row) => {
      return row.org_name;
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
    cell: ({ row }) => h('div', {}, row.getValue('owner')),
    filterFn: (row, id, value) => {
      return value.includes(row.getValue(id));
    },
    size: 180,
    minSize: 80,
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
      const registry = row.original;
      return h('div', { class: 'flex items-center gap-2' }, [
        h('span', {}, scopeLabel(registry.scope_type)),
        canEdit(registry) ? null : h(Badge, { variant: 'secondary' as const }, () => t('readonly')),
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
    accessorKey: 'created_at',
    header: ({ column }) => {
      return h(
        Button,
        {
          variant: 'ghost' as const,
          class: ' pr-6',
          onClick: () => column.toggleSorting(column.getIsSorted() === 'asc'),
        },
        () => [t('registries.added'), h(ArrowUpDown, { class: 'ml-2 h-4 w-4' })],
      );
    },
    cell: ({ row }) => {
      const registry = row.original;
      return h('div', {}, formatTimestamp(registry.created_at));
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
        () => [t('registries.last_edited'), h(ArrowUpDown, { class: 'ml-2 h-4 w-4' })],
      );
    },
    cell: ({ row }) => {
      const registry = row.original;
      return h('div', {}, formatTimestamp(registry.updated_at));
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
      const registry = row.original;
      if (!canEdit(registry)) {
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
                        onClick: () => emit('edit', registry),
                      },
                      () => [h(Icon, { name: 'edit', class: 'mr-2 h-4 w-4' }), t('registries.edit')],
                    ),
                    h(
                      DropdownMenuItem,
                      {
                        class: 'text-destructive',
                        onClick: () => handleSingleDelete(registry),
                      },
                      () => [h(Icon, { name: 'trash', class: 'mr-2 h-4 w-4' }), t('registries.delete')],
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
    return props.registries || [];
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

// Array of selected labels
const selectedScopeLabels = ref<string[]>([]);

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
  const scopeTypes = Array.from(facetedValues.keys());

  // Get unique labels
  const labels = new Set<string>();
  scopeTypes.forEach((scopeType) => {
    const label = scopeToLabel[scopeType];
    if (label) labels.add(label);
  });

  return Array.from(labels).sort();
});

const uniqueOwners = computed(() => {
  const ownerColumn = table.getColumn('owner');
  if (!ownerColumn) return [];
  const facetedValues = ownerColumn.getFacetedUniqueValues();
  return Array.from(facetedValues.keys()).sort();
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
</script>

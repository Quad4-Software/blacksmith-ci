<template>
  <div class="space-y-3 text-foreground">
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

          <!-- Entity Type Filter -->
          <DropdownMenu :modal="false">
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="sm" class="h-8 border-dashed">
                <Icon name="filter" class="mr-2 h-4 w-4" />
                {{ $t('admin.settings.entities.entity_type') }}
                <template v-if="selectedEntityTypes.length > 0">
                  <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
                  <span class="mx-2" aria-hidden="true">|</span>
                  <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
                  <Badge variant="secondary" class="rounded-sm px-1 font-normal">
                    {{ selectedEntityTypes.length }}
                  </Badge>
                </template>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent class="w-[200px]" align="start">
              <div class="max-h-[300px] overflow-y-auto">
                <DropdownMenuCheckboxItem
                  :model-value="selectedEntityTypes.includes('user')"
                  @update:model-value="(value) => toggleEntityType('user', !!value)"
                >
                  {{ $t('admin.settings.entities.type_user') }}
                </DropdownMenuCheckboxItem>
                <DropdownMenuCheckboxItem
                  :model-value="selectedEntityTypes.includes('org')"
                  @update:model-value="(value) => toggleEntityType('org', !!value)"
                >
                  {{ $t('admin.settings.entities.type_org') }}
                </DropdownMenuCheckboxItem>
              </div>
              <template v-if="selectedEntityTypes.length > 0">
                <DropdownMenuSeparator />
                <DropdownMenuItem @click="clearEntityTypeFilter">
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

          <!-- Admin Filter Toggle -->
          <div class="flex items-center gap-2">
            <Switch :model-value="showAdminsOnly" @update:model-value="toggleAdminFilter" />
            <label class="text-sm text-muted-foreground">
              {{ $t('admin.settings.entities.show_admins_only') }}
            </label>
          </div>
        </div>

        <!-- Bulk Actions -->
        <div v-if="table.getFilteredSelectedRowModel().rows.length > 0" class="flex items-center gap-2">
          <span class="text-sm text-muted-foreground">
            {{ table.getFilteredSelectedRowModel().rows.length }} {{ $t('selected') }}
          </span>
          <Button v-if="hasSelectedUsers" variant="outline" size="sm" @click="handleBulkToggleAdmin">
            <Icon name="user" class="h-4 w-4 mr-2" />
            {{ $t('admin.settings.entities.toggle_admin') }}
          </Button>
          <Button variant="destructive" size="sm" @click="handleBulkDelete">
            <Icon name="trash" class="h-4 w-4 mr-2" />
            {{ $t('delete') }}
          </Button>
        </div>
      </div>

      <!-- Table -->
      <div v-if="entities && entities.length > 0" class="rounded-md border">
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
                {{ $t('admin.settings.entities.none') }}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <!-- Empty State -->
      <div v-else-if="entities && entities.length === 0" class="text-center py-12 text-muted-foreground">
        <Icon name="user" class="h-12 w-12 mx-auto mb-3 opacity-50" />
        <p class="text-base">{{ $t('admin.settings.entities.none') }}</p>
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
          <AlertDialogTitle>{{ $t('admin.settings.entities.delete_confirm_title') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{
              $t('admin.settings.entities.delete_bulk_confirm', {
                count: table.getFilteredSelectedRowModel().rows.length,
              })
            }}
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
          <AlertDialogTitle>{{ $t('admin.settings.entities.delete_confirm_title') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('admin.settings.entities.delete_confirm', { name: entityToDelete?.name || '' }) }}
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
import { ArrowUpDown, Check, Copy, MoreVertical } from '@lucide/vue';
import type {
  ColumnDef,
  ColumnFiltersState,
  ColumnSizingState,
  ColumnVisibilityState,
  SortingState,
} from '@tanstack/vue-table';
import { FlexRender, useTable } from '@tanstack/vue-table';
import { useStorage } from '@vueuse/core';
import type { IconNames } from '~/components/atomic/Icon.types';
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
import { Switch } from '~/components/ui/switch';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import { dataTableFeatures } from '~/components/ui/table/features';
import { useForges } from '~/compositions/useForges';
import type { Entity, Forge } from '~/lib/api/types';
import { isUserEntity } from '~/lib/api/types';
import { computed, h, onMounted, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

const props = defineProps<{
  entities: Entity[] | null;
  isDeleting: boolean;
  searchTerm: string;
}>();

const emit = defineEmits<{
  (event: 'editUser', entity: Entity): void;
  (event: 'delete', entity: Entity): void;
  (event: 'bulkDelete', entities: Entity[]): void;
  (event: 'bulkToggleAdmin', entities: Entity[]): void;
  (event: 'update:searchTerm', value: string): void;
}>();

const { t } = useI18n();
const router = useRouter();
const { primaryForge } = useForges();

// Track copied state for entity names
const copiedNames = reactive<Record<string, boolean>>({});

async function copyName(name: string) {
  try {
    await navigator.clipboard.writeText(name);
    copiedNames[name] = true;
    setTimeout(() => {
      copiedNames[name] = false;
    }, 2000);
  } catch {
    // Failed to copy
  }
}

const sorting = ref<SortingState>([
  { id: 'id', desc: false },
  { id: 'entity_type', desc: false },
]);
const columnFilters = ref<ColumnFiltersState>([]);
const rowSelection = ref({});
const columnVisibility = ref<ColumnVisibilityState>({});

// Load column sizing from localStorage
const COLUMN_SIZING_STORAGE_KEY = 'crow-entities-table-column-sizing';
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

const selectedEntityTypes = useStorage<string[]>('crow:entities-filter-types', []);
const showAdminsOnly = useStorage<boolean>('crow:entities-filter-admins-only', false);

// Alert dialog state
const showBulkDeleteDialog = ref(false);
const showSingleDeleteDialog = ref(false);
const entityToDelete = ref<Entity | null>(null);

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

function getEntityForgeUrl(entity: Entity): string {
  if (!primaryForge.value?.url || !entity.name) return '';
  try {
    const baseUrl = primaryForge.value.url.endsWith('/') ? primaryForge.value.url.slice(0, -1) : primaryForge.value.url;
    return `${baseUrl}/${entity.name}`;
  } catch {
    return '';
  }
}

function handleBulkDelete() {
  showBulkDeleteDialog.value = true;
}

function handleSingleDelete(entity: Entity) {
  entityToDelete.value = entity;
  showSingleDeleteDialog.value = true;
}

function confirmSingleDelete() {
  if (entityToDelete.value) {
    emit('delete', entityToDelete.value);
  }
  entityToDelete.value = null;
  showSingleDeleteDialog.value = false;
}

const columns: ColumnDef<typeof dataTableFeatures, Entity>[] = [
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
    accessorKey: 'name',
    header: ({ column }) => {
      return h(
        Button,
        {
          variant: 'ghost',
          class: 'pr-6',
          onClick: () => column.toggleSorting(column.getIsSorted() === 'asc'),
        },
        () => [t('admin.settings.entities.name'), h(ArrowUpDown, { class: 'ml-2 h-4 w-4' })],
      );
    },
    cell: ({ row }) => {
      const entity = row.original;
      const name = entity.name;
      if (entity.entity_type === 'org') {
        // Org name as clickable link with copy button
        return h('div', { class: 'flex items-center gap-1' }, [
          h(
            'a',
            {
              href: `/orgs/${entity.id}`,
              class: 'font-medium text-link hover:underline cursor-pointer',
              onClick: (e: Event) => {
                e.preventDefault();
                router.push({ name: 'org', params: { orgId: entity.id } });
              },
            },
            name,
          ),
          h(
            'button',
            {
              type: 'button',
              class: 'p-1 hover:bg-muted rounded transition-colors',
              title: copiedNames[name] ? t('copied') : t('copy'),
              onClick: (e: Event) => {
                e.stopPropagation();
                copyName(name);
              },
            },
            h(copiedNames[name] ? Check : Copy, { class: 'h-3 w-3 text-muted-foreground' }),
          ),
        ]);
      } else {
        // User name as editable button with copy button
        return h('div', { class: 'flex items-center gap-1' }, [
          h(
            'button',
            {
              class: 'font-medium text-link hover:underline cursor-pointer',
              onClick: () => emit('editUser', entity),
            },
            name,
          ),
          h(
            'button',
            {
              type: 'button',
              class: 'p-1 hover:bg-muted rounded transition-colors',
              title: copiedNames[name] ? t('copied') : t('copy'),
              onClick: (e: Event) => {
                e.stopPropagation();
                copyName(name);
              },
            },
            h(copiedNames[name] ? Check : Copy, { class: 'h-3 w-3 text-muted-foreground' }),
          ),
        ]);
      }
    },
    size: 200,
    minSize: 100,
  },
  {
    accessorKey: 'entity_type',
    header: ({ column }) => {
      return h(
        Button,
        {
          variant: 'ghost',
          class: 'pr-6',
          onClick: () => column.toggleSorting(column.getIsSorted() === 'asc'),
        },
        () => [t('admin.settings.entities.entity_type'), h(ArrowUpDown, { class: 'ml-2 h-4 w-4' })],
      );
    },
    cell: ({ row }) => {
      const entity = row.original;
      const label =
        entity.entity_type === 'user' ? t('admin.settings.entities.type_user') : t('admin.settings.entities.type_org');
      return h(Badge, { variant: entity.entity_type === 'user' ? 'default' : 'secondary' }, () => label);
    },
    filterFn: (row, id, value) => {
      return value.includes(row.getValue(id));
    },
    size: 120,
    minSize: 100,
  },
  {
    accessorKey: 'admin',
    header: () => t('admin.settings.entities.admin_column'),
    cell: ({ row }) => {
      const entity = row.original;
      if (entity.entity_type === 'user' && entity.admin) {
        return h(Icon, { name: 'lucide:check', class: 'h-4 w-4 text-green-600' });
      }
      return '';
    },
    filterFn: (row, _id, value: boolean) => {
      // If filter is off (false or undefined), show all
      if (!value) return true;
      // If filter is on (true), only show admin users
      const entity = row.original;
      return entity.entity_type === 'user' && entity.admin === true;
    },
    size: 100,
    minSize: 80,
  },
  {
    id: 'forge',
    header: () => t('admin.settings.entities.forge'),
    cell: ({ row }) => {
      const entity = row.original;
      if (!primaryForge.value) return null;
      const forgeUrl = getEntityForgeUrl(entity);
      if (!forgeUrl) return null;
      return h(
        'a',
        {
          href: forgeUrl,
          target: '_blank',
          class: 'inline-flex items-center justify-center h-6 w-6 hover:opacity-80 transition-opacity',
        },
        h(Icon, { name: getForgeIconName(primaryForge.value), class: 'h-4 w-4' }),
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
      const entity = row.original;

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
                    entity.entity_type === 'user'
                      ? h(
                          DropdownMenuItem,
                          {
                            onClick: () => emit('editUser', entity),
                          },
                          () => [h(Icon, { name: 'edit', class: 'mr-2 h-4 w-4' }), t('admin.settings.entities.edit')],
                        )
                      : h(
                          DropdownMenuItem,
                          {
                            onClick: () => router.push({ name: 'org', params: { orgId: entity.id } }),
                          },
                          () => [
                            h(Icon, { name: 'chevron-right', class: 'mr-2 h-4 w-4' }),
                            t('admin.settings.entities.view'),
                          ],
                        ),
                    h(
                      DropdownMenuItem,
                      {
                        class: 'text-destructive',
                        onClick: () => handleSingleDelete(entity),
                      },
                      () => [h(Icon, { name: 'trash', class: 'mr-2 h-4 w-4' }), t('delete')],
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

const table = useTable({
  features: dataTableFeatures,
  get data() {
    return props.entities || [];
  },
  columns,
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

function confirmBulkDelete() {
  const selected = table.getFilteredSelectedRowModel().rows.map((row) => row.original);
  emit('bulkDelete', selected);
  table.resetRowSelection();
  showBulkDeleteDialog.value = false;
}

function handleBulkToggleAdmin() {
  const selected = table.getFilteredSelectedRowModel().rows.map((row) => row.original);
  const users = selected.filter(isUserEntity);
  emit('bulkToggleAdmin', users);
  table.resetRowSelection();
}

const hasSelectedUsers = computed(() => {
  const selected = table.getFilteredSelectedRowModel().rows.map((row) => row.original);
  return selected.some(isUserEntity);
});

function toggleEntityType(type: string, checked: boolean) {
  if (checked) {
    if (!selectedEntityTypes.value.includes(type)) {
      selectedEntityTypes.value.push(type);
    }
  } else {
    const index = selectedEntityTypes.value.indexOf(type);
    if (index > -1) {
      selectedEntityTypes.value.splice(index, 1);
    }
  }

  if (selectedEntityTypes.value.length > 0) {
    table.getColumn('entity_type')?.setFilterValue(selectedEntityTypes.value);
  } else {
    table.getColumn('entity_type')?.setFilterValue(undefined);
  }
}

function clearEntityTypeFilter() {
  selectedEntityTypes.value = [];
  table.getColumn('entity_type')?.setFilterValue(undefined);
}

function toggleAdminFilter(value: boolean) {
  showAdminsOnly.value = value;
  table.getColumn('admin')?.setFilterValue(value ? true : undefined);
}

// Initialize filters from localStorage on mount
onMounted(() => {
  if (selectedEntityTypes.value.length > 0) {
    table.getColumn('entity_type')?.setFilterValue(selectedEntityTypes.value);
  }
  if (showAdminsOnly.value) {
    table.getColumn('admin')?.setFilterValue(true);
  }
});
</script>

<template>
  <div class="space-y-3 text-foreground">
    <div class="space-y-4">
      <!-- Toolbar with Bulk Actions -->
      <div v-if="table.getFilteredSelectedRowModel().rows.length > 0" class="flex items-center gap-2">
        <span class="text-sm text-muted-foreground">
          {{ table.getFilteredSelectedRowModel().rows.length }} {{ $t('selected') }}
        </span>
        <Button variant="destructive" size="sm" @click="handleBulkDelete">
          <Icon name="trash" class="h-4 w-4 mr-2" />
          {{ $t('delete') }}
        </Button>
      </div>

      <!-- Table -->
      <div v-if="tokens && tokens.length > 0" class="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow v-for="headerGroup in table.getHeaderGroups()" :key="headerGroup.id">
              <TableHead
                v-for="header in headerGroup.headers"
                :key="header.id"
                :style="{
                  width: `${header.getSize()}px`,
                  minWidth: `${header.column.columnDef.minSize || header.getSize()}px`,
                }"
              >
                <FlexRender
                  v-if="!header.isPlaceholder"
                  :render="header.column.columnDef.header"
                  :props="header.getContext()"
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
                {{ $t('access_tokens.empty') }}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <!-- Empty State -->
      <div v-else-if="tokens && tokens.length === 0" class="text-center py-12 text-muted-foreground">
        <Icon name="secret" class="h-12 w-12 mx-auto mb-4 opacity-50" />
        <p>{{ $t('access_tokens.empty') }}</p>
        <p class="text-sm mt-1">{{ $t('access_tokens.empty_description') }}</p>
      </div>

      <!-- Loading State -->
      <div v-else class="text-center py-12 text-muted-foreground">
        <Icon name="spinner" class="h-12 w-12 mx-auto mb-3 animate-spin" />
        <p class="text-base">{{ $t('loading') }}</p>
      </div>
    </div>

    <!-- Bulk Delete Confirmation Dialog -->
    <AlertDialog v-model:open="showBulkDeleteDialog">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('access_tokens.delete_confirm_title') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{
              $t('access_tokens.delete_bulk_confirm', {
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
          <AlertDialogTitle>{{ $t('access_tokens.delete_confirm_title') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('access_tokens.delete_confirm', { name: tokenToDelete?.name || '' }) }}
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

    <!-- Edit Scopes Dialog -->
    <AccessTokenEditDialog
      v-model:open="showEditDialog"
      :token="tokenToEdit"
      :is-saving="isUpdating"
      @save="handleEditSave"
    />
  </div>
</template>

<script lang="ts" setup>
import type { ColumnDef, SortingState, VisibilityState } from '@tanstack/vue-table';
import { FlexRender, getCoreRowModel, getSortedRowModel, useVueTable } from '@tanstack/vue-table';
import { MoreVertical } from 'lucide-vue-next';
import { h, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import AccessTokenEditDialog from '~/components/access-tokens/AccessTokenEditDialog.vue';
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
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import type { AccessToken, UpdateAccessTokenRequest } from '~/lib/api/types';

const props = defineProps<{
  tokens: AccessToken[] | null;
  isDeleting?: boolean;
  isUpdating?: boolean;
}>();

const emit = defineEmits<{
  (event: 'delete', tokenId: number): void;
  (event: 'bulkDelete', tokenIds: number[]): void;
  (event: 'update', tokenId: number, data: UpdateAccessTokenRequest): void;
}>();

const { t } = useI18n();

const sorting = ref<SortingState>([{ id: 'created_at', desc: true }]);
const rowSelection = ref({});
const columnVisibility = ref<VisibilityState>({});

// Dialog states
const showBulkDeleteDialog = ref(false);
const showSingleDeleteDialog = ref(false);
const showEditDialog = ref(false);
const tokenToDelete = ref<AccessToken | null>(null);
const tokenToEdit = ref<AccessToken | null>(null);

function formatDate(timestamp: number, includeTime = false): string {
  if (!timestamp) return '';
  const date = new Date(timestamp * 1000);
  if (includeTime) {
    return date.toLocaleString(undefined, {
      dateStyle: 'short',
      timeStyle: 'short',
    });
  }
  return date.toLocaleDateString();
}

function formatRelativeTime(timestamp: number): string {
  if (!timestamp) return '';

  const now = Math.floor(Date.now() / 1000);
  const diff = now - timestamp;

  if (diff < 60) {
    return t('time.just_now');
  } else if (diff < 3600) {
    const minutes = Math.floor(diff / 60);
    return t('time.minutes_ago', { count: minutes });
  } else if (diff < 86400) {
    const hours = Math.floor(diff / 3600);
    return t('time.hours_ago', { count: hours });
  } else if (diff < 604800) {
    const days = Math.floor(diff / 86400);
    return t('time.days_ago', { count: days });
  } else {
    return formatDate(timestamp, true);
  }
}

function isExpired(token: AccessToken): boolean {
  if (!token.expires_at) return false;
  return token.expires_at < Math.floor(Date.now() / 1000);
}

function isExpiringSoon(token: AccessToken): boolean {
  if (!token.expires_at) return false;
  const now = Math.floor(Date.now() / 1000);
  const sevenDays = 7 * 24 * 60 * 60;
  return token.expires_at > now && token.expires_at < now + sevenDays;
}

function handleBulkDelete() {
  showBulkDeleteDialog.value = true;
}

function handleSingleDelete(token: AccessToken) {
  tokenToDelete.value = token;
  showSingleDeleteDialog.value = true;
}

function confirmSingleDelete() {
  if (tokenToDelete.value) {
    emit('delete', tokenToDelete.value.id);
  }
  tokenToDelete.value = null;
  showSingleDeleteDialog.value = false;
}

function handleEdit(token: AccessToken) {
  tokenToEdit.value = token;
  showEditDialog.value = true;
}

function handleEditSave(data: UpdateAccessTokenRequest) {
  if (tokenToEdit.value) {
    emit('update', tokenToEdit.value.id, data);
  }
}

const columns: ColumnDef<AccessToken>[] = [
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
    size: 40,
    minSize: 40,
    maxSize: 40,
  },
  {
    accessorKey: 'name',
    header: () => t('access_tokens.name'),
    cell: ({ row }) => {
      const token = row.original;
      const name = token.name || t('access_tokens.unnamed');
      const expired = isExpired(token);
      const expiringSoon = isExpiringSoon(token);

      return h('div', { class: 'flex items-center gap-2 flex-wrap' }, [
        h('span', { class: expired ? 'text-muted-foreground line-through' : 'font-medium' }, name),
        expired
          ? h(Badge, { variant: 'destructive' }, () => t('access_tokens.expired'))
          : expiringSoon
            ? h(Badge, { variant: 'outline', class: 'border-amber-500/50 text-amber-700 dark:text-amber-400' }, () =>
                t('access_tokens.expiring_soon'),
              )
            : null,
      ]);
    },
    size: 200,
    minSize: 150,
  },
  {
    accessorKey: 'scopes',
    header: () => t('access_tokens.scopes'),
    cell: ({ row }) => {
      const token = row.original;
      return h(
        'div',
        { class: 'flex flex-wrap gap-1' },
        token.scopes.map((scope) => h(Badge, { key: scope, variant: 'secondary', class: 'text-xs' }, () => scope)),
      );
    },
    size: 250,
    minSize: 150,
  },
  {
    accessorKey: 'last_used',
    header: () => t('access_tokens.last_used'),
    cell: ({ row }) => {
      const token = row.original;
      if (!token.last_used) {
        return h('span', { class: 'text-muted-foreground text-sm' }, t('access_tokens.never_used'));
      }
      return h('span', { class: 'text-sm' }, formatRelativeTime(token.last_used));
    },
    size: 120,
    minSize: 100,
  },
  {
    accessorKey: 'created_at',
    header: () => t('access_tokens.created'),
    cell: ({ row }) => {
      const token = row.original;
      const dateStr = token.created_at ? formatDate(token.created_at, true) : '-';
      return h('span', { class: 'text-sm text-muted-foreground' }, dateStr || '-');
    },
    size: 140,
    minSize: 120,
  },
  {
    accessorKey: 'expires_at',
    header: () => t('access_tokens.expires'),
    cell: ({ row }) => {
      const token = row.original;
      if (!token.expires_at) {
        return h('span', { class: 'text-muted-foreground text-sm' }, t('access_tokens.no_expiration'));
      }
      return h('span', { class: 'text-sm' }, formatDate(token.expires_at));
    },
    size: 100,
    minSize: 80,
  },
  {
    id: 'actions',
    enableHiding: false,
    size: 50,
    cell: ({ row }) => {
      const token = row.original;

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
                        onClick: () => handleEdit(token),
                      },
                      () => [h(Icon, { name: 'edit', class: 'mr-2 h-4 w-4' }), t('access_tokens.edit')],
                    ),
                    h(
                      DropdownMenuItem,
                      {
                        class: 'text-destructive',
                        onClick: () => handleSingleDelete(token),
                      },
                      () => [h(Icon, { name: 'trash', class: 'mr-2 h-4 w-4' }), t('access_tokens.delete')],
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
    return props.tokens || [];
  },
  columns,
  getCoreRowModel: getCoreRowModel(),
  getSortedRowModel: getSortedRowModel(),
  onSortingChange: (updaterOrValue) => {
    sorting.value = typeof updaterOrValue === 'function' ? updaterOrValue(sorting.value) : updaterOrValue;
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
    get rowSelection() {
      return rowSelection.value;
    },
    get columnVisibility() {
      return columnVisibility.value;
    },
  },
});

// Must be defined after table is created
function confirmBulkDelete() {
  const selected = table.getFilteredSelectedRowModel().rows.map((row) => row.original.id);
  emit('bulkDelete', selected);
  table.resetRowSelection();
  showBulkDeleteDialog.value = false;
}
</script>

<template>
  <DropdownMenu :modal="false">
    <DropdownMenuTrigger as-child>
      <Button variant="outline" size="sm" class="h-8 border-dashed">
        <Icon name="filter" class="mr-2 h-4 w-4" />
        {{ $t('filter').charAt(0).toUpperCase() + $t('filter').slice(1) }}
        <template v-if="activeFilterCount > 0">
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          <span class="mx-2 select-none" aria-hidden="true">|</span>
          <Badge variant="secondary" class="rounded-sm px-1 font-normal">
            {{ activeFilterCount }}
          </Badge>
        </template>
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent class="w-[200px]" align="start">
      <!-- Show All Repos Toggle -->
      <div class="px-2 py-1.5">
        <DropdownMenuCheckboxItem :model-value="showAllRepos" @update:model-value="$emit('toggleShowAll')">
          {{ $t('repositories.filter.show_all') }}
        </DropdownMenuCheckboxItem>
      </div>

      <!-- Owner Filter Section -->
      <template v-if="owners.length > 0">
        <DropdownMenuSeparator />
        <div class="px-2 py-1">
          <div class="text-xs font-semibold text-muted-foreground px-2 py-1.5">
            {{ $t('owner') }}
          </div>
          <div class="max-h-[300px] overflow-y-auto">
            <DropdownMenuCheckboxItem
              v-for="owner in owners"
              :key="owner"
              :model-value="selectedOwners.includes(owner)"
              @update:model-value="$emit('toggleOwner', owner)"
            >
              {{ owner }}
            </DropdownMenuCheckboxItem>
          </div>
        </div>
      </template>

      <!-- Clear Filters -->
      <template v-if="activeFilterCount > 0">
        <DropdownMenuSeparator />
        <DropdownMenuItem @click="$emit('clearFilters')">
          {{ $t('clear_filters') }}
        </DropdownMenuItem>
      </template>
    </DropdownMenuContent>
  </DropdownMenu>
</template>

<script lang="ts" setup>
import Icon from '~/components/atomic/Icon.vue';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';

interface Props {
  owners: string[];
  selectedOwners: string[];
  showAllRepos: boolean;
  activeFilterCount: number;
}

interface Emits {
  (event: 'toggleOwner', owner: string): void;
  (event: 'toggleShowAll'): void;
  (event: 'clearFilters'): void;
}

defineProps<Props>();
defineEmits<Emits>();
</script>

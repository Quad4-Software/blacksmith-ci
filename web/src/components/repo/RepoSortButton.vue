<template>
  <DropdownMenu :modal="false">
    <DropdownMenuTrigger as-child>
      <Button variant="outline" size="sm" class="h-8">
        <Icon name="lucide:arrow-up-down" class="mr-2 h-4 w-4" />
        {{ $t('sort') }}
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent class="w-[200px]" align="end">
      <DropdownMenuRadioGroup :model-value="sortBy" @update:model-value="$emit('update:sortBy', $event)">
        <DropdownMenuRadioItem value="alphabetical">
          {{ $t('repositories.sort.alphabetical') }}
        </DropdownMenuRadioItem>
        <DropdownMenuRadioItem value="last_pipeline">
          {{ $t('repositories.sort.last_pipeline') }}
        </DropdownMenuRadioItem>
        <DropdownMenuRadioItem value="status">
          {{ $t('repositories.sort.status') }}
        </DropdownMenuRadioItem>
      </DropdownMenuRadioGroup>
    </DropdownMenuContent>
  </DropdownMenu>
</template>

<script lang="ts" setup>
import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';

interface Props {
  sortBy: string;
}

interface Emits {
  (event: 'update:sortBy', value: string): void;
}

defineProps<Props>();
defineEmits<Emits>();
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <Button variant="outline" size="sm" class="h-9 border-dashed">
        <Icon name="repo" class="mr-2 h-4 w-4" />
        {{ $t('metrics.filter.repos') }}
        <template v-if="selectedCount > 0 && selectedCount < repos.length">
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          <span class="mx-2" aria-hidden="true">|</span>
          <Badge variant="secondary" class="rounded-sm px-1 font-normal">
            {{ selectedCount }}
          </Badge>
        </template>
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end" class="max-h-[300px] overflow-y-auto w-[250px]">
      <div class="flex items-center justify-between px-2 py-1.5">
        <Button variant="ghost" size="sm" class="h-6 px-2 text-xs" @click="selectAll">
          {{ $t('metrics.filter.select_all') }}
        </Button>
        <Button variant="ghost" size="sm" class="h-6 px-2 text-xs" @click="clearSelection">
          {{ $t('metrics.filter.clear') }}
        </Button>
      </div>
      <DropdownMenuSeparator />
      <DropdownMenuCheckboxItem
        v-for="repo in repos"
        :key="repo.id"
        :checked="selectedRepoIds.includes(repo.id)"
        @click.prevent="toggleRepo(repo.id)"
      >
        <div class="flex items-center gap-2 truncate">
          <Icon name="repo" class="size-3.5 text-muted-foreground shrink-0" />
          <span class="truncate">{{ repo.full_name }}</span>
        </div>
      </DropdownMenuCheckboxItem>
      <div v-if="repos.length === 0" class="px-2 py-4 text-sm text-muted-foreground text-center">
        {{ $t('metrics.filter.no_repos') }}
      </div>
    </DropdownMenuContent>
  </DropdownMenu>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import type { Repo } from '~/lib/api/types';

const props = defineProps<{
  repos: Repo[];
  selectedRepoIds: number[];
}>();

const emit = defineEmits<{
  'update:selectedRepoIds': [value: number[]];
}>();

const selectedCount = computed(() => props.selectedRepoIds.length);

function toggleRepo(repoId: number) {
  const current = props.selectedRepoIds;
  if (current.includes(repoId)) {
    emit(
      'update:selectedRepoIds',
      current.filter((id) => id !== repoId),
    );
  } else {
    emit('update:selectedRepoIds', [...current, repoId]);
  }
}

function selectAll() {
  emit(
    'update:selectedRepoIds',
    props.repos.map((r) => r.id),
  );
}

function clearSelection() {
  emit('update:selectedRepoIds', []);
}
</script>

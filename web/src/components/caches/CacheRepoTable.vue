<template>
  <div v-if="repos.length > 0" class="rounded-md border" data-testid="cache-repo-table">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ $t('caches.repository') }}</TableHead>
          <TableHead class="text-right">{{ $t('caches.size') }}</TableHead>
          <TableHead class="text-right">{{ $t('caches.entries') }}</TableHead>
          <TableHead class="text-right">{{ $t('caches.hit_rate') }}</TableHead>
          <TableHead class="text-right">{{ $t('caches.egress') }}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="repo in repos" :key="repo.repo_id">
          <TableCell>
            <router-link
              v-if="repo.full_name"
              :to="{ name: 'repo-settings-caches', params: { repoId: repo.repo_id } }"
              class="hover:underline"
            >
              {{ repo.full_name }}
            </router-link>
            <span v-else class="text-muted-foreground">{{ $t('caches.deleted_repo', { id: repo.repo_id }) }}</span>
          </TableCell>
          <TableCell class="text-right whitespace-nowrap">{{ formatCacheSize(repo.size) }}</TableCell>
          <TableCell class="text-right">{{ repo.entries }}</TableCell>
          <TableCell class="text-right">{{ formatHitRate(cacheHitRate(repo.usage)) }}</TableCell>
          <TableCell class="text-right whitespace-nowrap">{{ formatCacheSize(repo.usage.egress) }}</TableCell>
        </TableRow>
      </TableBody>
    </Table>
  </div>
</template>

<script lang="ts" setup>
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import type { BuildCacheRepoSummary } from '~/lib/api/types';
import { cacheHitRate, formatCacheSize, formatHitRate } from '~/lib/build-cache';

defineProps<{
  repos: BuildCacheRepoSummary[];
}>();
</script>

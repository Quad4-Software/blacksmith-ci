<template>
  <div class="space-y-3">
    <div class="flex items-center justify-between">
      <label class="text-sm font-medium">{{ $t('secrets.shared_with') }}</label>
    </div>

    <!-- Current owner -->
    <div class="flex items-center gap-2 p-2 border rounded-md bg-muted/50">
      <Badge variant="default">{{ $t('secrets.owner') }}</Badge>
      <span class="text-sm">{{ ownerName }}</span>
    </div>

    <!-- Shared targets -->
    <div v-if="targets.length > 0" class="space-y-2">
      <div v-for="target in targets" :key="target.id" class="flex items-center justify-between p-2 border rounded-md">
        <div class="flex items-center gap-2">
          <Badge variant="outline">{{ $t('secrets.shared') }}</Badge>
          <span class="text-sm">{{ getTargetName(target) }}</span>
        </div>
        <div class="flex items-center gap-1">
          <Button variant="ghost" size="sm" @click="emit('makeOwner', target)">
            {{ $t('secrets.make_owner') }}
          </Button>
          <Button variant="ghost" size="sm" @click="emit('removeTarget', target)">
            <Icon name="close" class="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>

    <!-- Add target button -->
    <Popover v-model:open="addTargetOpen">
      <PopoverTrigger as-child>
        <Button variant="outline" size="sm" class="w-full border-dashed">
          <Icon name="plus" class="mr-2 h-4 w-4" />
          {{ $t('secrets.add_target') }}
        </Button>
      </PopoverTrigger>
      <PopoverContent class="w-[400px] p-0" align="start">
        <Command v-model:search-term="searchTerm">
          <CommandInput :placeholder="$t('secrets.scope.search_target')" />
          <CommandList>
            <CommandEmpty>{{ $t('no_search_results') }}</CommandEmpty>
            <CommandGroup v-if="orgs.length > 0" :heading="$t('secrets.scope.organizations')">
              <CommandItem
                v-for="org in orgs"
                :key="`org-${org.id}`"
                :value="org.name"
                @select="() => handleAddTarget('org', org.id)"
              >
                {{ org.name }}
              </CommandItem>
            </CommandGroup>
            <CommandGroup v-if="repos.length > 0" :heading="$t('secrets.scope.repositories')">
              <CommandItem
                v-for="repo in repos"
                :key="`repo-${repo.id}`"
                :value="`${repo.owner}/${repo.name}`"
                @select="() => handleAddTarget('repo', repo.id)"
              >
                {{ `${repo.owner}/${repo.name}` }}
              </CommandItem>
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  </div>
</template>

<script lang="ts" setup>
import { onMounted, ref } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '~/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import useApiClient from '~/compositions/useApiClient';
import type { Org, Repo, SecretTarget } from '~/lib/api/types';

const props = defineProps<{
  secretId: number;
  ownerName: string;
  targets: SecretTarget[];
  orgNames: Record<number, string>;
  repoNames: Record<number, string>;
}>();

const emit = defineEmits<{
  (event: 'addTarget', type: 'org' | 'repo', id: number): void;
  (event: 'removeTarget', target: SecretTarget): void;
  (event: 'makeOwner', target: SecretTarget): void;
}>();

const apiClient = useApiClient();

const addTargetOpen = ref(false);
const searchTerm = ref('');
const repos = ref<Repo[]>([]);
const orgs = ref<Org[]>([]);

function getTargetName(target: SecretTarget): string {
  if (target.repo_id) {
    return props.repoNames[target.repo_id] || `Repo #${target.repo_id}`;
  }
  if (target.org_id) {
    return props.orgNames[target.org_id] || `Org #${target.org_id}`;
  }
  return 'Unknown';
}

function handleAddTarget(type: 'org' | 'repo', id: number) {
  addTargetOpen.value = false;
  emit('addTarget', type, id);
}

onMounted(async () => {
  try {
    const [reposData, orgsData] = await Promise.all([
      apiClient.getActiveRepos({ silent: true }),
      apiClient.getUserOrgs({ silent: true }),
    ]);
    repos.value = reposData || [];
    orgs.value = orgsData || [];
  } catch {
    // Silent failure
  }
});
</script>

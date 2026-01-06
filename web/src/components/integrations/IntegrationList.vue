<template>
  <div class="space-y-4">
    <Card v-if="integrations.length === 0" class="p-6 text-center text-muted-foreground">
      {{ $t('integrations.empty') }}
    </Card>

    <Card v-for="integration in integrations" :key="integration.id" class="p-4">
      <div class="flex items-start justify-between gap-4">
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 mb-1">
            <h3 class="font-medium truncate">{{ integration.name }}</h3>
            <Badge variant="outline" class="shrink-0 font-mono">
              {{ $t('integrations.id_label', { id: integration.id }) }}
            </Badge>
            <Badge :variant="integration.is_active ? 'default' : 'secondary'" class="shrink-0">
              {{ integration.is_active ? $t('integrations.active') : $t('integrations.inactive') }}
            </Badge>
            <Badge variant="outline" class="shrink-0">
              {{ getTypeName(integration.type) }}
            </Badge>
          </div>
          <p v-if="integration.description" class="text-sm text-muted-foreground mb-2 line-clamp-2">
            {{ integration.description }}
          </p>
          <div class="flex flex-wrap gap-2 text-xs text-muted-foreground">
            <Tooltip v-if="integration.allowed_user_ids?.length">
              <TooltipTrigger as-child>
                <span class="flex items-center gap-1 cursor-help">
                  <Icon name="user" class="size-3" />
                  {{ $t('integrations.users_count', { count: integration.allowed_user_ids.length }) }}
                </span>
              </TooltipTrigger>
              <TooltipContent>
                <div class="text-sm">
                  <div v-for="id in integration.allowed_user_ids" :key="id">
                    {{ usersMap.get(id)?.login || `User #${id}` }}
                  </div>
                </div>
              </TooltipContent>
            </Tooltip>
            <Tooltip v-if="integration.allowed_org_ids?.length">
              <TooltipTrigger as-child>
                <span class="flex items-center gap-1 cursor-help">
                  <Icon name="repo" class="size-3" />
                  {{ $t('integrations.orgs_count', { count: integration.allowed_org_ids.length }) }}
                </span>
              </TooltipTrigger>
              <TooltipContent>
                <div class="text-sm">
                  <div v-for="id in integration.allowed_org_ids" :key="id">
                    {{ orgsMap.get(id)?.name || `Org #${id}` }}
                  </div>
                </div>
              </TooltipContent>
            </Tooltip>
            <Tooltip v-if="integration.allowed_repo_ids?.length">
              <TooltipTrigger as-child>
                <span class="flex items-center gap-1 cursor-help">
                  <Icon name="repo" class="size-3" />
                  {{ $t('integrations.repos_count', { count: integration.allowed_repo_ids.length }) }}
                </span>
              </TooltipTrigger>
              <TooltipContent>
                <div class="text-sm">
                  <div v-for="id in integration.allowed_repo_ids" :key="id">
                    {{ reposMap.get(id)?.full_name || `Repo #${id}` }}
                  </div>
                </div>
              </TooltipContent>
            </Tooltip>
            <span v-if="!hasAnyAllowlist(integration)" class="text-warning">
              {{ $t('integrations.no_allowlist') }}
            </span>
          </div>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <Button variant="outline" size="sm" :disabled="isTesting" @click="$emit('test', integration)">
            <Icon name="lucide:zap" class="mr-1 size-4" />
            {{ $t('integrations.test_connection') }}
          </Button>
          <Button variant="outline" size="sm" @click="$emit('edit', integration)">
            <Icon name="edit" class="mr-1 size-4" />
            {{ $t('edit') }}
          </Button>
          <Button variant="destructive" size="sm" :disabled="isDeleting" @click="$emit('delete', integration)">
            <Icon name="trash" class="mr-1 size-4" />
            {{ $t('delete') }}
          </Button>
        </div>
      </div>
    </Card>
  </div>
</template>

<script lang="ts" setup>
import Icon from '~/components/atomic/Icon.vue';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { Tooltip, TooltipContent, TooltipTrigger } from '~/components/ui/tooltip';
import type { Integration, IntegrationType, Org, Repo, User } from '~/lib/api/types';

defineProps<{
  integrations: Integration[];
  isDeleting: boolean;
  isTesting: boolean;
  usersMap: Map<number, User>;
  orgsMap: Map<number, Org>;
  reposMap: Map<number, Repo>;
}>();

defineEmits<{
  (e: 'edit', integration: Integration): void;
  (e: 'delete', integration: Integration): void;
  (e: 'test', integration: Integration): void;
}>();

const typeNames: Record<IntegrationType, string> = {
  vault: 'HashiCorp Vault / OpenBao',
};

function getTypeName(type: IntegrationType): string {
  return typeNames[type] || type;
}

function hasAnyAllowlist(integration: Integration): boolean {
  return (
    (integration.allowed_user_ids?.length ?? 0) > 0 ||
    (integration.allowed_org_ids?.length ?? 0) > 0 ||
    (integration.allowed_repo_ids?.length ?? 0) > 0
  );
}
</script>

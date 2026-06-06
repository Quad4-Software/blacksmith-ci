<template>
  <div class="space-y-3">
    <router-link
      v-for="workflow in workflows"
      :key="workflow.id"
      :to="{
        name: 'repo-pipeline',
        params: { repoId: workflow.repo_id, pipelineId: workflow.pipeline_number, stepId: workflow.pid },
      }"
      class="block"
    >
      <Card class="hover:shadow-md hover:border-primary/50 transition-all cursor-pointer">
        <div class="p-4">
          <!-- Header: Status, Name, Pipeline number -->
          <div class="flex items-center gap-3 mb-2">
            <PipelineStatusIcon :status="workflow.state" class="h-4 w-4 shrink-0" />

            <span class="font-medium truncate flex-1">{{ workflow.name || workflow.pipeline_name }}</span>

            <UiBadge variant="secondary" class="shrink-0">
              {{ `#${workflow.pipeline_number}` }}
            </UiBadge>

            <Icon name="chevron-right" class="h-4 w-4 text-muted-foreground shrink-0" />
          </div>

          <!-- Time info -->
          <div class="flex items-center gap-4 text-xs text-muted-foreground">
            <div v-if="workflow.finished" class="flex items-center gap-1">
              <Icon name="lucide:clock" class="h-3 w-3" />
              <span>{{ $t('admin.settings.agents.workflow_finished') }} {{ timeAgo(workflow.finished * 1000) }}</span>
            </div>
            <div v-if="workflow.started && workflow.finished" class="flex items-center gap-1">
              <Icon name="lucide:timer" class="h-3 w-3" />
              <span>{{ prettyDuration((workflow.finished - workflow.started) * 1000) }}</span>
            </div>
          </div>
        </div>
      </Card>
    </router-link>
  </div>
</template>

<script lang="ts" setup>
import Icon from '~/components/atomic/Icon.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import { Badge as UiBadge } from '~/components/ui/badge';
import Card from '~/components/ui/card/Card.vue';
import type { AgentWorkflow } from '~/lib/api/types';

defineProps<{
  workflows: AgentWorkflow[];
  timeAgo: (timestamp: number) => string;
  prettyDuration: (ms: number) => string;
}>();
</script>

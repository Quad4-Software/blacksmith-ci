<template>
  <div v-if="pipeline" class="flex items-center gap-2 min-w-0 w-full">
    <!-- Checkbox positioned outside the main item with fixed width spacer -->
    <div v-if="showCheckbox" class="w-7 shrink-0 flex items-center justify-center" @click.stop>
      <Checkbox :model-value="isSelected" @update:model-value="$emit('toggleSelect')" />
    </div>

    <!-- Main pipeline item -->
    <ListItem
      :to="showCheckbox ? undefined : to"
      class="flex-1 p-0! min-w-0"
      :class="{
        'ring-2 ring-primary/50 bg-primary/5': isSelected,
        'cursor-pointer': showCheckbox,
      }"
      @click="showCheckbox ? $emit('toggleSelect') : undefined"
    >
      <div class="flex w-11 shrink-0 items-center justify-center">
        <PipelineStatusIcon class="mx-2 md:mx-3" :status="pipeline.status" />
      </div>

      <div class="flex min-w-0 grow flex-wrap px-0 lg:px-4 py-0.5 pl-0.5! md:flex-nowrap">
        <div class="hidden shrink-0 items-center md:flex">
          <Icon v-if="pipeline.event === 'cron'" name="stopwatch" class="text-foreground" />
          <UserAvatar v-else :avatar-url="pipeline.author_avatar" :username="pipeline.author" size="md" />
        </div>

        <div class="flex min-w-0 flex-1 items-center md:mx-4">
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          <span class="md:display-unset hidden text-muted-foreground shrink-0">#{{ pipeline.number }}</span>
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          <span class="md:display-unset mx-2 hidden text-muted-foreground shrink-0">-</span>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger as-child>
                <span
                  class="overflow-hidden text-ellipsis whitespace-nowrap text-foreground underline md:no-underline min-w-0"
                >
                  {{ shortDisplayTitle }}
                </span>
              </TooltipTrigger>
              <TooltipContent side="bottom" class="max-w-md whitespace-pre-wrap break-words">
                <p>{{ displayTitle }}</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <!-- Restart indicator (hidden on mobile to prevent overflow) -->
          <span
            v-if="pipeline.parent"
            class="hidden md:inline ml-2 text-xs text-muted-foreground whitespace-nowrap shrink-0"
          >
            {{ restartIndicator }}
          </span>
        </div>

        <div
          class="grid w-full shrink-0 grid-flow-col grid-cols-1 grid-rows-2 gap-y-2 py-2 text-foreground md:ml-auto md:w-85"
        >
          <div class="flex min-w-0 items-center space-x-2">
            <span :title="pipelineEventTitle">
              <Icon v-if="pipeline.event === 'pull_request'" name="pull-request" size="1rem" />
              <Icon v-else-if="pipeline.event === 'pull_request_closed'" name="pull-request-closed" size="1rem" />
              <Icon v-else-if="pipeline.event === 'pull_request_edited'" name="pull-request-edited" size="1rem" />
              <Icon v-else-if="pipeline.event === 'deployment'" name="deployment" size="1rem" />
              <Icon v-else-if="pipeline.event === 'tag' || pipeline.event === 'release'" name="tag" size="1rem" />
              <Icon v-else-if="pipeline.event === 'cron'" name="branch" size="1rem" />
              <Icon v-else-if="pipeline.event === 'manual'" name="manual-pipeline" size="1rem" />
              <Icon v-else name="branch" size="1rem" />
            </span>
            <span class="truncate">{{ prettyRef }}</span>
          </div>

          <div class="flex min-w-0 items-center space-x-2">
            <Icon name="commit" size="1rem" /> <span class="truncate">{{ pipeline.commit.slice(0, 10) }}</span>
          </div>

          <div class="flex min-w-0 items-center space-x-2" :title="$t('repo.pipeline.duration')">
            <Icon name="duration" size="1rem" /> <span class="truncate">{{ duration }}</span>
          </div>

          <div class="flex min-w-0 items-center space-x-2" :title="$t('repo.pipeline.created', { created })">
            <Icon name="startDate" size="1rem" /> <span class="truncate">{{ startDate }}</span>
          </div>
        </div>
      </div>

      <!-- Action buttons -->
      <div class="flex flex-col md:flex-row items-start md:items-center gap-0.5 md:gap-1 pr-2 md:pr-4 py-2 shrink-0">
        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger as-child>
              <Button variant="ghost" size="icon-sm" :disabled="isRerunning" @click="rerunPipeline">
                <Icon name="lucide:refresh-ccw" class="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              <p>{{ $t('repo.pipeline.rerun_latest') }}</p>
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
        <TooltipProvider v-if="!showCheckbox">
          <Tooltip>
            <TooltipTrigger as-child>
              <Button variant="ghost" size="icon-sm" @click="isHidden ? unhidePipeline($event) : hidePipeline($event)">
                <Icon :name="isHidden ? 'lucide:eye' : 'lucide:eye-off'" class="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              <p>{{ isHidden ? $t('repo.pipeline.unhide') : $t('repo.pipeline.hide') }}</p>
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>
    </ListItem>
  </div>
</template>

<script lang="ts" setup>
import { computed, toRef } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import Checkbox from '~/components/ui/checkbox/Checkbox.vue';
import UserAvatar from '~/components/user/UserAvatar.vue';
import ListItem from '~/components/atomic/ListItem.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import { Button } from '~/components/ui/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import useApiClient from '~/compositions/useApiClient';
import useNotifications from '~/compositions/useNotifications';
import usePipeline from '~/compositions/usePipeline';
import { requiredInject } from '~/compositions/useInjectProvide';
import type { Pipeline } from '~/lib/api/types';

const props = defineProps<{
  pipeline: Pipeline;
  isSelected?: boolean;
  showCheckbox?: boolean;
  isHidden?: boolean;
  to?: any; // Route location for navigation
}>();

const emit = defineEmits<{
  toggleSelect: [];
  hide: [pipelineId: number];
  unhide: [pipelineId: number];
}>();

const { t } = useI18n();
const apiClient = useApiClient();
const notifications = useNotifications();
const repo = requiredInject('repo');
const isRerunning = computed(() => false); // Can be expanded to track per-pipeline state

const pipeline = toRef(props, 'pipeline');
const { startDate, duration, message, prettyRef, created } = usePipeline(pipeline);

// Use pipeline title if available, otherwise use message
const displayTitle = computed(() => {
  // Check if we have a title field
  if (pipeline.value.title && pipeline.value.title.trim() !== '') {
    return pipeline.value.title;
  }
  return message.value;
});

const shortDisplayTitle = computed(() => {
  return displayTitle.value.split('\n')[0];
});

const restartIndicator = computed(() => {
  if (pipeline.value.parent) {
    return `(restart of #${pipeline.value.parent})`;
  }
  return '';
});

const pipelineEventTitle = computed(() => {
  switch (pipeline.value.event) {
    case 'pull_request':
      return t('repo.pipeline.event.pr');
    case 'pull_request_closed':
      return t('repo.pipeline.event.pr_closed');
    case 'pull_request_edited':
      return t('repo.pipeline.event.pr_edited');
    case 'deployment':
      return t('repo.pipeline.event.deploy');
    case 'tag':
      return t('repo.pipeline.event.tag');
    case 'release':
      return t('repo.pipeline.event.release');
    case 'cron':
      return t('repo.pipeline.event.cron');
    case 'manual':
      return t('repo.pipeline.event.manual');
    default:
      return t('repo.pipeline.event.push');
  }
});

async function rerunPipeline(event: Event) {
  event.preventDefault();
  event.stopPropagation();

  try {
    await apiClient.restartPipeline(repo.value.id, pipeline.value.number.toString());
    notifications.notify({
      title: t('repo.pipeline.rerun_success'),
      type: 'success',
    });
    // Reload to show the new pipeline
    setTimeout(() => {
      window.location.reload();
    }, 1000);
  } catch {
    notifications.notify({
      title: t('repo.pipeline.rerun_failed'),
      type: 'error',
    });
  }
}

function hidePipeline(event: Event) {
  event.preventDefault();
  event.stopPropagation();
  emit('hide', pipeline.value.id);
}

function unhidePipeline(event: Event) {
  event.preventDefault();
  event.stopPropagation();
  emit('unhide', pipeline.value.id);
}
</script>

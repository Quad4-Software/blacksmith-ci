<template>
  <div v-if="pipeline" class="flex items-center">
    <!-- Checkbox positioned outside the main item with fixed width spacer -->
    <div v-if="showCheckbox" class="w-7 flex-shrink-0">
      <input
        type="checkbox"
        class="h-4 w-4 rounded border-wp-custom-200 accent-wp-custom-highlight-100 focus:ring-2 focus:ring-wp-custom-highlight-100 focus:ring-offset-1 transition-all cursor-pointer"
        :checked="isSelected"
        @click.stop
        @change="$emit('toggleSelect')"
      />
    </div>

    <!-- Main pipeline item -->
    <ListItem
      :to="to"
      class="flex-1 p-0!"
      :class="{ 'ring-2 ring-wp-custom-highlight-100/50 bg-wp-custom-highlight-100/5': isSelected }"
    >
      <div class="flex w-11 shrink-0 items-center justify-center">
        <PipelineStatusIcon class="mx-2 md:mx-3" :status="pipeline.status" />
      </div>

      <div class="flex min-w-0 grow flex-wrap px-4 py-0.5 pl-0.5! md:flex-nowrap">
        <div class="hidden shrink-0 items-center md:flex">
          <Icon v-if="pipeline.event === 'cron'" name="stopwatch" class="text-wp-text-100" />
          <img v-else class="w-6 h-6 rounded-md object-cover" :src="pipeline.author_avatar" />
        </div>

        <div class="flex w-full min-w-0 items-center md:mx-4 md:w-auto">
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          <span class="md:display-unset hidden text-wp-text-alt-100">#{{ pipeline.number }}</span>
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          <span class="md:display-unset mx-2 hidden text-wp-text-alt-100">-</span>
          <span
            class="overflow-hidden text-ellipsis whitespace-nowrap text-wp-text-100 underline md:no-underline"
            :title="displayTitle"
          >
            {{ shortDisplayTitle }}
          </span>
          <!-- Restart indicator -->
          <span v-if="pipeline.parent" class="ml-2 text-xs text-wp-text-alt-100 whitespace-nowrap">
            {{ restartIndicator }}
          </span>
        </div>

        <div
          class="grid w-full shrink-0 grid-flow-col grid-cols-1 grid-rows-2 gap-y-2 py-2 text-wp-text-100 md:ml-auto md:w-85"
        >
          <div class="flex min-w-0 items-center space-x-2">
            <span :title="pipelineEventTitle">
              <Icon v-if="pipeline.event === 'pull_request'" name="pull-request" class="" />
              <Icon v-else-if="pipeline.event === 'pull_request_closed'" name="pull-request-closed" class="" />
              <Icon v-else-if="pipeline.event === 'pull_request_edited'" name="pull-request-edited" class="" />
              <Icon v-else-if="pipeline.event === 'deployment'" name="deployment" class="" />
              <Icon v-else-if="pipeline.event === 'tag' || pipeline.event === 'release'" name="tag" class="" />
              <Icon v-else-if="pipeline.event === 'cron'" name="branch" class="" />
              <Icon v-else-if="pipeline.event === 'manual'" name="manual-pipeline" class="" />
              <Icon v-else name="branch" size="1.3rem" class="" />
            </span>
            <span class="truncate">{{ prettyRef }}</span>
          </div>

          <div class="flex min-w-0 items-center space-x-2">
            <Icon name="commit" class="" /> <span class="truncate">{{ pipeline.commit.slice(0, 10) }}</span>
          </div>

          <div class="flex min-w-0 items-center space-x-2" :title="$t('repo.pipeline.duration')">
            <Icon name="duration" /> <span class="truncate">{{ duration }}</span>
          </div>

          <div class="flex min-w-0 items-center space-x-2" :title="$t('repo.pipeline.created', { created })">
            <Icon name="startDate" /> <span class="truncate">{{ startDate }}</span>
          </div>
        </div>
      </div>
    </ListItem>
  </div>
</template>

<script lang="ts" setup>
import { computed, toRef } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import ListItem from '~/components/atomic/ListItem.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import usePipeline from '~/compositions/usePipeline';
import type { Pipeline } from '~/lib/api/types';

const props = defineProps<{
  pipeline: Pipeline;
  isSelected?: boolean;
  showCheckbox?: boolean;
  to?: any; // Route location for navigation
}>();

defineEmits<{
  toggleSelect: [];
}>();

const { t } = useI18n();

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
</script>

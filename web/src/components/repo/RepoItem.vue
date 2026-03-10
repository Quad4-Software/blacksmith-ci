<template>
  <Transition name="fade" mode="out-in">
    <div v-if="repo && repo.active" ref="cardRef" class="flex items-center gap-2 min-w-0 w-full">
      <!-- Drag handle positioned outside the main item (like PipelineItem checkbox) -->
      <div v-if="showDragHandle" class="w-8 shrink-0 flex items-center justify-center" @click.stop.prevent>
        <div
          class="drag-handle cursor-grab active:cursor-grabbing text-muted-foreground hover:text-foreground transition-colors"
        >
          <Icon name="lucide:grip-vertical" class="h-5 w-5" />
        </div>
      </div>

      <ListItem :to="{ name: 'repo', params: { repoId: repo.id } }" class="flex-1 p-0! min-w-0">
        <!-- Status icon column (matches PipelineItem w-11) -->
        <div class="flex w-11 shrink-0 items-center justify-center self-stretch">
          <PipelineStatusIcon v-if="lastPipeline" :status="lastPipeline.status" class="mx-2 md:mx-3" />
        </div>

        <!-- Rerun button (fixed-width slot so content always starts at the same position) -->
        <div class="w-8 shrink-0 flex items-center justify-center" @click.stop>
          <TooltipProvider v-if="lastPipeline">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" :disabled="isRerunning" @click.prevent="rerunPipeline">
                  <Icon name="lucide:refresh-ccw" size="1rem" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                <p>{{ $t('repo.pipeline.rerun_latest') }}</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>

        <!-- Content area -->
        <div class="flex min-w-0 flex-1 items-center gap-2 pl-1 pr-2 py-2">
          <!-- Repo identity: responsive width, fixed at lg for alignment -->
          <div class="min-w-0 max-w-64 shrink sm:shrink-0 sm:w-44 md:w-48 lg:w-52 xl:w-64 flex items-center gap-1.5">
            <span class="truncate font-medium text-foreground text-sm">
              {{ `${repo.owner} / ${repo.name}` }}
            </span>
            <TooltipProvider v-if="repo.visibility === RepoVisibility.Private">
              <Tooltip>
                <TooltipTrigger as-child>
                  <Icon name="lucide:lock" class="hidden lg:inline-flex h-3.5 w-3.5 shrink-0 text-destructive" />
                </TooltipTrigger>
                <TooltipContent>
                  <p>{{ $t('repo.visibility.private.private') }}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
            <TooltipProvider v-else-if="repo.visibility === RepoVisibility.Internal">
              <Tooltip>
                <TooltipTrigger as-child>
                  <Icon name="lucide:lock" class="hidden lg:inline-flex h-3.5 w-3.5 shrink-0 text-chart-2" />
                </TooltipTrigger>
                <TooltipContent>
                  <p>{{ $t('repo.visibility.internal.internal') }}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>

          <!-- Pin + forge (hidden on mobile) -->
          <div class="hidden sm:flex shrink-0 items-center">
            <TooltipProvider @click.stop>
              <Tooltip>
                <TooltipTrigger as-child>
                  <span class="inline-flex">
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      :disabled="!canPin && !isPinned"
                      @click.prevent="emit('togglePin')"
                    >
                      <Icon
                        :name="isPinned ? 'lucide:pin' : 'lucide:pin-off'"
                        class="h-4 w-4"
                        :class="{ 'text-muted-foreground': isPinned }"
                      />
                    </Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent>
                  <p>
                    {{ isPinned ? $t('repo.unpin') : !canPin ? $t('repositories.pin_limit_reached') : $t('repo.pin') }}
                  </p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
            <a
              v-if="forge && repo.forge_url"
              :href="repo.forge_url"
              :title="$t('repo.open_in_forge')"
              target="_blank"
              class="hidden lg:inline-flex h-7 w-7 items-center justify-center rounded-md hover:bg-accent transition-colors"
              @click.stop
            >
              <Icon :name="getForgeIconName(forge)" class="h-4 w-4" />
            </a>
          </div>

          <!-- Event icon + pipeline message (hidden on mobile) -->
          <template v-if="lastPipeline">
            <span class="hidden sm:flex shrink-0 text-muted-foreground items-center">
              <Icon v-if="lastPipeline.event === 'pull_request'" name="pull-request" size="0.875rem" />
              <Icon
                v-else-if="lastPipeline.event === 'pull_request_closed'"
                name="pull-request-closed"
                size="0.875rem"
              />
              <Icon
                v-else-if="lastPipeline.event === 'pull_request_merged'"
                name="pull-request-merged"
                size="0.875rem"
              />
              <Icon
                v-else-if="lastPipeline.event === 'pull_request_edited'"
                name="pull-request-edited"
                size="0.875rem"
              />
              <Icon v-else-if="lastPipeline.event === 'deployment'" name="deployment" size="0.875rem" />
              <Icon
                v-else-if="lastPipeline.event === 'tag' || lastPipeline.event === 'release'"
                name="tag"
                size="0.875rem"
              />
              <Icon v-else-if="lastPipeline.event === 'cron'" name="stopwatch" size="0.875rem" />
              <Icon v-else-if="lastPipeline.event === 'manual'" name="manual-pipeline" size="0.875rem" />
              <Icon v-else name="branch" size="0.875rem" />
            </span>
            <span class="hidden sm:inline truncate text-sm text-muted-foreground min-w-0 flex-1">
              {{ shortMessage }}
            </span>
          </template>
          <span v-else class="hidden sm:inline text-sm text-muted-foreground flex-1 min-w-0 truncate">
            {{ $t('repo.pipeline.no_pipelines') }}
          </span>

          <!-- History dots (lg+ only) -->
          <div v-if="recentPipelines.length > 0" class="hidden lg:flex items-center gap-1 shrink-0" @click.stop>
            <router-link
              v-for="pipeline in recentPipelines"
              :key="pipeline.id"
              :to="{ name: 'repo-pipeline', params: { repoId: repo.id, pipelineId: pipeline.number } }"
              :title="`Pipeline #${pipeline.number} - ${pipeline.status}`"
              class="hover:scale-125 transition-all hover:drop-shadow-md rounded-full p-0.5 hover:bg-accent"
            >
              <PipelineStatusIcon :status="pipeline.status" size="1rem" />
            </router-link>
          </div>
        </div>

        <!-- Metadata column (lg+ only) -->
        <div class="hidden lg:flex shrink-0 w-72 items-center gap-4 text-sm text-muted-foreground py-2 mr-4">
          <template v-if="lastPipeline">
            <div class="flex items-center gap-1 whitespace-nowrap w-32 shrink-0">
              <Icon name="duration" size="0.875rem" class="shrink-0" />
              <span class="truncate">{{ duration }}</span>
            </div>
            <div class="flex items-center gap-1 whitespace-nowrap">
              <Icon name="startDate" size="0.875rem" class="shrink-0" />
              <span>{{ startDate }}</span>
            </div>
          </template>
        </div>

        <!-- Power/disable button -->
        <div class="shrink-0 pr-2 pl-2 flex items-center" @click.stop>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" :disabled="isUpdating" @click.prevent="toggleRepoActive(false)">
                  <Icon name="lucide:power" class="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                <p>{{ $t('repo.disable') }}</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>
      </ListItem>
    </div>
  </Transition>
</template>

<script lang="ts" setup>
import { useIntersectionObserver } from '@vueuse/core';
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import ListItem from '~/components/atomic/ListItem.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import { Button } from '~/components/ui/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import useApiClient from '~/compositions/useApiClient';
import { useForges } from '~/compositions/useForges';
import useNotifications from '~/compositions/useNotifications';
import usePipeline from '~/compositions/usePipeline';
import type { Repo, Forge, Pipeline } from '~/lib/api/types';
import { RepoVisibility } from '~/lib/api/types';
import type { IconNames } from '~/components/atomic/Icon.types';
import { useRepoStore } from '~/store/repos';

const props = defineProps<{
  repo: Repo;
  isPinned?: boolean;
  canPin?: boolean;
  showDragHandle?: boolean;
}>();

const emit = defineEmits<{
  togglePin: [];
}>();

const { t } = useI18n();
const apiClient = useApiClient();
const notifications = useNotifications();
const repoStore = useRepoStore();
const isUpdating = ref(false);
const recentPipelines = ref<Pipeline[]>([]);
const isRerunning = ref(false);

// Use global forge data
const { getForgeById } = useForges();
const forge = computed(() => getForgeById(props.repo.forge_id));

// Map forge types to local icon names, with special handling for known instances
function getForgeIconName(forge: Forge): IconNames {
  // Check for specific forge instances by URL
  if (forge.url.toLowerCase().includes('codeberg')) {
    return 'codeberg';
  }
  if (forge.url.toLowerCase().includes('codefloe')) {
    return 'codefloe';
  }
  // Fall back to forge type
  const typeToIcon: Record<string, IconNames> = {
    github: 'github',
    gitlab: 'gitlab',
    gitea: 'gitea',
    forgejo: 'forgejo',
    bitbucket: 'bitbucket',
    'bitbucket-dc': 'bitbucket',
    addon: 'repo',
  };
  return typeToIcon[forge.type] || 'repo';
}

async function toggleRepoActive(checked: boolean) {
  if (!checked) {
    // First update the local state to trigger the fade-out transition
    repoStore.setRepo({
      ...props.repo,
      active: false,
    });

    // Wait for transition to complete (300ms as defined in CSS)
    await new Promise((resolve) => setTimeout(resolve, 350));

    // Then make the API call to disable
    isUpdating.value = true;
    try {
      await apiClient.deleteRepo(props.repo.id, false); // false = disable, not delete
      notifications.notify({
        title: t('repo.settings.actions.disable.notify_text_success'),
        type: 'success',
      });
    } catch {
      // If the API call fails, revert the local state
      repoStore.setRepo({
        ...props.repo,
        active: true,
      });
      notifications.notify({
        title: t('notify_title_error'),
        type: 'error',
      });
    } finally {
      isUpdating.value = false;
    }
  } else {
    // This shouldn't happen since we only show active repos
    // But if somehow toggled on, reactivate
    isUpdating.value = true;
    try {
      const updatedRepo = await apiClient.activateRepo(props.repo.forge_remote_id);
      repoStore.setRepo(updatedRepo);
      notifications.notify({
        title: t('repo.settings.actions.enable.notify_text_success'),
        type: 'success',
      });
    } catch {
      notifications.notify({
        title: t('notify_title_error'),
        type: 'error',
      });
    } finally {
      isUpdating.value = false;
    }
  }
}

const lastPipeline = computed(() => props.repo.last_pipeline);
const { shortMessage, duration, startDate } = usePipeline(lastPipeline);

// Lazy-load last 5 pipelines only when the card scrolls into view
const cardRef = ref<HTMLElement | null>(null);
const pipelinesLoaded = ref(false);

const { stop: stopObserver } = useIntersectionObserver(
  cardRef,
  async ([{ isIntersecting }]) => {
    if (isIntersecting && !pipelinesLoaded.value) {
      pipelinesLoaded.value = true;
      stopObserver();
      try {
        const pipelines = await apiClient.getPipelineList(props.repo.id, { perPage: 5 });
        recentPipelines.value = pipelines || [];
      } catch {
        // Silently fail - the card still shows the last pipeline from the repo object
      }
    }
  },
  { threshold: 0.1 },
);

async function rerunPipeline() {
  if (!lastPipeline.value) return;

  isRerunning.value = true;
  try {
    await apiClient.restartPipeline(props.repo.id, lastPipeline.value.number.toString());
    notifications.notify({
      title: t('repo.pipeline.rerun_success'),
      type: 'success',
    });
    // Refresh the pipeline list
    const pipelines = await apiClient.getPipelineList(props.repo.id, { perPage: 5 });
    recentPipelines.value = pipelines || [];
  } catch {
    notifications.notify({
      title: t('repo.pipeline.rerun_failed'),
      type: 'error',
    });
  } finally {
    isRerunning.value = false;
  }
}
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: all 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}
</style>

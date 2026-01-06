<template>
  <Transition name="fade" mode="out-in">
    <router-link v-if="repo && repo.active" :to="{ name: 'repo', params: { repoId: repo.id } }" class="block w-full">
      <Card class="p-0 hover:bg-accent/50 transition-colors cursor-pointer">
        <CardHeader class="mb-2 p-4 pb-1">
          <div class="flex items-start justify-between gap-4">
            <div class="flex items-center gap-2 flex-1 min-w-0">
              <!-- Drag handle for reordering pinned repos -->
              <div
                v-if="showDragHandle"
                class="drag-handle flex items-center cursor-grab active:cursor-grabbing text-muted-foreground hover:text-foreground transition-colors shrink-0"
                @click.stop.prevent
              >
                <Icon name="lucide:grip-vertical" class="h-5 w-5" />
              </div>
              <CardTitle class="text-base md:text-lg flex-1 min-w-0 truncate">
                {{ `${repo.owner} / ${repo.name}` }}
              </CardTitle>
            </div>

            <div class="flex items-center gap-1 md:gap-2 shrink-0 flex-wrap justify-end">
              <!-- Visibility badge (moved first, hidden on mobile) -->
              <Badge
                v-if="repo.visibility === RepoVisibility.Private"
                variant="secondary"
                class="hidden md:inline-flex"
              >
                <Icon name="visibility-private" class="mr-1 h-3 w-3" />
                {{ $t('repo.visibility.private.private') }}
              </Badge>
              <Badge
                v-else-if="repo.visibility === RepoVisibility.Internal"
                variant="secondary"
                class="hidden md:inline-flex"
              >
                <Icon name="visibility-internal" class="mr-1 h-3 w-3" />
                {{ $t('repo.visibility.internal.internal') }}
              </Badge>

              <!-- Pin button -->
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
                      {{
                        isPinned ? $t('repo.unpin') : !canPin ? $t('repositories.pin_limit_reached') : $t('repo.pin')
                      }}
                    </p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>

              <!-- Power button (disable repo) -->
              <TooltipProvider @click.stop>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      :disabled="isUpdating"
                      @click.prevent="toggleRepoActive(false)"
                    >
                      <Icon name="lucide:power" class="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>{{ $t('repo.disable') }}</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>

              <!-- Forge link -->
              <a
                v-if="forge && repo.forge_url"
                :href="repo.forge_url"
                :title="`${$t('repo.open_in_forge')}`"
                target="_blank"
                class="inline-flex h-8 w-8 items-center justify-center rounded-md hover:bg-accent transition-colors"
                @click.stop
              >
                <Icon :name="getForgeIconName(forge)" class="h-4 w-4" />
              </a>
            </div>
          </div>
        </CardHeader>

        <CardContent class="p-4 pt-0">
          <div v-if="lastPipeline" class="flex items-center justify-between gap-4">
            <!-- Rerun button, last pipeline, and history -->
            <div class="flex min-w-0 flex-1 items-center gap-2 -ml-2">
              <TooltipProvider @click.stop>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon-sm" :disabled="isRerunning" @click="rerunPipeline">
                      <Icon name="lucide:refresh-ccw" size="1rem" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>{{ $t('repo.pipeline.rerun_latest') }}</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>

              <router-link
                :to="{ name: 'repo-pipeline', params: { repoId: repo.id, pipelineId: lastPipeline.number } }"
                class="flex items-center gap-2 hover:underline min-w-0"
                @click.stop
              >
                <PipelineStatusIcon :status="lastPipeline.status" size="1rem" class="shrink-0" />
                <span class="truncate text-sm text-muted-foreground">
                  {{ shortMessage }}
                </span>
              </router-link>

              <!-- Separator (hidden on mobile) -->
              <Separator
                v-if="recentPipelines.length > 0"
                orientation="vertical"
                class="hidden md:block h-5 bg-muted-foreground/30"
              />

              <!-- Last 5 pipeline status indicators (hidden on mobile) -->
              <div v-if="recentPipelines.length > 0" class="hidden md:flex items-center gap-2" @click.stop>
                <span class="text-sm text-muted-foreground">{{ $t('repo.pipeline.history') }}</span>
                <div class="flex items-center gap-1">
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
            </div>

            <!-- Duration -->
            <div class="flex shrink-0 items-center gap-1 text-sm text-muted-foreground">
              <Icon name="duration" size="1rem" />
              <span>{{ duration }}</span>
            </div>
          </div>
          <div v-else class="text-sm text-muted-foreground" :class="{ 'ml-1.5': showDragHandle }">
            {{ $t('repo.pipeline.no_pipelines') }}
          </div>
        </CardContent>
      </Card>
    </router-link>
  </Transition>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import PipelineStatusIcon from '~/components/repo/pipeline/PipelineStatusIcon.vue';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '~/components/ui/card';
import { Separator } from '~/components/ui/separator';
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

const apiClient = useApiClient();
const notifications = useNotifications();
const repoStore = useRepoStore();
const isUpdating = ref(false);
const recentPipelines = ref<Pipeline[]>([]);
const isRerunning = ref(false);

// Use global forge data
const { getForgeById } = useForges();
const forge = computed(() => getForgeById(props.repo.forge_id));

// Map forge types to local icon names
function getForgeIconName(_forge: Forge): IconNames {
  // TEMP: Hardcoded for inspection
  return 'gitea';
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
        title: 'Repository disabled',
        type: 'success',
      });
    } catch {
      // If the API call fails, revert the local state
      repoStore.setRepo({
        ...props.repo,
        active: true,
      });
      notifications.notify({
        title: 'Failed to update repository',
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
        title: 'Repository enabled',
        type: 'success',
      });
    } catch {
      notifications.notify({
        title: 'Failed to update repository',
        type: 'error',
      });
    } finally {
      isUpdating.value = false;
    }
  }
}

const lastPipeline = computed(() => props.repo.last_pipeline);
const { shortMessage, duration } = usePipeline(lastPipeline);

// Fetch last 5 pipelines
onMounted(async () => {
  try {
    const pipelines = await apiClient.getPipelineList(props.repo.id, { perPage: 5 });
    recentPipelines.value = pipelines || [];
  } catch (error) {
    console.error('Failed to fetch recent pipelines:', error);
  }
});

async function rerunPipeline() {
  if (!lastPipeline.value) return;

  isRerunning.value = true;
  try {
    await apiClient.restartPipeline(props.repo.id, lastPipeline.value.number.toString());
    notifications.notify({
      title: 'Pipeline restarted',
      type: 'success',
    });
    // Refresh the pipeline list
    const pipelines = await apiClient.getPipelineList(props.repo.id, { perPage: 5 });
    recentPipelines.value = pipelines || [];
  } catch {
    notifications.notify({
      title: 'Failed to restart pipeline',
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

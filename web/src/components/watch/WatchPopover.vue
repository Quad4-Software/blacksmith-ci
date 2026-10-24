<template>
  <Popover v-if="key" v-model:open="open">
    <PopoverTrigger as-child>
      <Button variant="outline" size="sm" class="h-8 shrink-0 gap-2" data-testid="watch-popover-trigger">
        <Icon :name="existing ? 'ph:eye' : 'ph:eye-slash'" class="h-4 w-4" />
        {{ existing ? $t('repo.watch.watching') : $t('repo.watch.watch') }}
      </Button>
    </PopoverTrigger>
    <PopoverContent :collision-padding="12" align="end" class="w-96 max-w-[calc(100vw-2rem)]">
      <div class="flex flex-col gap-4">
        <div class="flex flex-col gap-1">
          <h4 class="text-sm font-semibold">{{ $t('repo.watch.popover_title', { label }) }}</h4>
          <p class="text-sm text-muted-foreground">{{ $t('repo.watch.popover_desc') }}</p>
        </div>
        <WatchSettingsFields v-model="settings" @valid="valid = $event" />
        <div class="flex justify-between gap-2">
          <Button
            v-if="existing"
            variant="ghost"
            size="sm"
            :disabled="isRemoving || isSaving"
            data-testid="watch-unwatch"
            @click="remove()"
          >
            <Icon v-if="isRemoving" name="spinner" class="mr-2 size-4 animate-spin" />
            {{ $t('repo.watch.unwatch') }}
          </Button>
          <Button
            size="sm"
            class="ml-auto"
            :disabled="!valid || isSaving || isRemoving"
            data-testid="watch-save"
            @click="save()"
          >
            <Icon v-if="isSaving" name="spinner" class="mr-2 size-4 animate-spin" />
            {{ $t('save') }}
          </Button>
        </div>
      </div>
    </PopoverContent>
  </Popover>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import WatchSettingsFields from '~/components/watch/WatchSettingsFields.vue';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useNotifications from '~/compositions/useNotifications';
import { useWatches } from '~/compositions/useWatches';
import type { Pipeline, PipelineWatch, WatchSettings } from '~/lib/api/types';
import { watchKeyFor, watchLabel } from '~/lib/watchKey';

const props = defineProps<{ repoId: number; pipeline: Pipeline }>();

const { t } = useI18n();
const apiClient = useApiClient();
const { notify } = useNotifications();
const { load } = useWatches();

const open = ref(false);
const existing = ref<PipelineWatch | null>(null);
const valid = ref(true);
const settings = ref<WatchSettings>(defaults());

const key = computed(() => watchKeyFor(props.pipeline));
const label = computed(() => (key.value ? watchLabel(key.value.event, key.value.target) : ''));

function defaults(): WatchSettings {
  return { in_app: true, email: false, email_events: ['failure'] };
}

function applyExisting(w: PipelineWatch | null) {
  existing.value = w;
  settings.value = w ? { in_app: w.in_app, email: w.email, email_events: [...(w.email_events ?? [])] } : defaults();
}

async function refresh() {
  const k = key.value;
  if (!k) return;
  try {
    const list = await apiClient.getRepoWatches(props.repoId);
    applyExisting(list.find((i) => i.watch.event === k.event && i.watch.target === k.target)?.watch ?? null);
  } catch (error) {
    console.error(error);
  }
}

function fail() {
  notify({ title: t('repo.watch.action_failed'), type: 'error' });
}

const { doSubmit: save, isLoading: isSaving } = useAsyncAction(async () => {
  const k = key.value;
  if (!k) return;
  const body = { ...settings.value };
  const result = existing.value
    ? await apiClient.updateWatch(existing.value.id, body)
    : await apiClient.createRepoWatch(props.repoId, { event: k.event, target: k.target, ...body });
  applyExisting({
    ...(existing.value ?? ({ event: k.event, target: k.target, repo_id: props.repoId } as PipelineWatch)),
    ...result,
    ...body,
  });
  open.value = false;
  await load();
}, fail);

const { doSubmit: remove, isLoading: isRemoving } = useAsyncAction(async () => {
  if (!existing.value) return;
  await apiClient.deleteWatch(existing.value.id);
  applyExisting(null);
  open.value = false;
  await load();
}, fail);

onMounted(refresh);
watch(
  () => [props.repoId, key.value?.event, key.value?.target],
  () => void refresh(),
);
</script>

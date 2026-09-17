<template>
  <Popover v-if="names.length" v-model:open="open">
    <PopoverTrigger as-child>
      <Button variant="outline" size="sm" class="h-8 gap-2 border-dashed">
        <Icon name="ph:flow-arrow" class="h-4 w-4" />
        {{ $t('repo.pipeline.filter.workflows') }}
        <Badge v-if="model.length" variant="secondary" class="rounded-sm px-1.5 font-normal">{{ model.length }}</Badge>
        <Icon name="chevron-down" class="h-3 w-3" />
      </Button>
    </PopoverTrigger>
    <PopoverContent :collision-padding="12" align="start" class="w-80 max-w-[calc(100vw-2rem)] p-1">
      <div class="max-h-[min(60vh,var(--reka-popover-content-available-height))] overflow-y-auto">
        <div
          v-for="name in names"
          :key="name"
          class="flex items-center gap-1 rounded-sm px-2 hover:bg-accent"
          :class="{ 'bg-accent/60': model.includes(name) }"
        >
          <label class="flex min-w-0 flex-1 cursor-pointer items-center gap-2 py-2 text-sm">
            <Checkbox :checked="model.includes(name)" @update:checked="toggle(name)" />
            <span class="truncate" :title="name">{{ name }}</span>
          </label>
        </div>
      </div>
      <div v-if="model.length" class="mt-1 border-t pt-1">
        <Button variant="ghost" size="sm" class="w-full justify-start" @click="model = []">{{
          $t('clear_filters')
        }}</Button>
      </div>
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import Icon from '~/components/atomic/Icon.vue';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import useApiClient from '~/compositions/useApiClient';
import type { Repo } from '~/lib/api/types';

const props = defineProps<{ repo: Repo }>();
const model = defineModel<string[]>({ required: true });
const api = useApiClient();
const open = ref(false);
const names = ref<string[]>([]);

watch(
  () => props.repo.id,
  async (id, _, onCleanup) => {
    let stale = false;
    onCleanup(() => {
      stale = true;
    });
    names.value = [];
    open.value = false;
    try {
      const result = await api.getWorkflowNames(id, { silent: true });
      if (!stale) names.value = result;
    } catch {
      // A repository without workflow history has no filters to show.
    }
  },
  { immediate: true },
);

function toggle(name: string) {
  model.value = model.value.includes(name) ? model.value.filter((value) => value !== name) : [...model.value, name];
}
</script>

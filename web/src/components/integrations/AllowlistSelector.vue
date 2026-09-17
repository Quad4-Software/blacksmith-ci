<template>
  <div class="space-y-2">
    <Label>{{ label }}</Label>
    <Popover v-model:open="open">
      <PopoverTrigger as-child>
        <Button variant="outline" role="combobox" :aria-expanded="open" class="w-full justify-between">
          <span v-if="selectedItems.length === 0" class="text-muted-foreground">
            {{ placeholder }}
          </span>
          <span v-else class="truncate"> {{ selectedItems.length }} {{ $t('integrations.allowlist.selected') }} </span>
          <Icon name="ph:arrows-down-up" class="ml-2 h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent class="w-[400px] p-0" align="start">
        <Command>
          <CommandInput v-model="searchQuery" :placeholder="searchPlaceholder" />
          <CommandList>
            <CommandEmpty>{{ $t('integrations.allowlist.no_results') }}</CommandEmpty>
            <CommandGroup>
              <CommandItem
                v-for="item in filteredItems"
                :key="item.id"
                :value="String(item.id)"
                @select="toggleItem(item.id)"
              >
                <Checkbox :checked="isSelected(item.id)" class="mr-2" />
                <span class="truncate">{{ item.name }}</span>
                <span v-if="item.description" class="ml-2 text-xs text-muted-foreground truncate">
                  {{ item.description }}
                </span>
              </CommandItem>
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>

    <!-- Selected items as tags -->
    <div v-if="selectedItems.length > 0" class="flex flex-wrap gap-1 mt-2">
      <Badge v-for="item in selectedItems" :key="item.id" variant="secondary" class="gap-1">
        {{ item.name }}
        <button type="button" class="ml-1 hover:text-destructive" @click="removeItem(item.id)">
          <Icon name="ph:x" class="h-3 w-3" />
        </button>
      </Badge>
    </div>

    <p v-if="helpText" class="text-xs text-muted-foreground">{{ helpText }}</p>
  </div>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '~/components/ui/command';
import { Label } from '~/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';

export interface AllowlistItem {
  id: number;
  name: string;
  description?: string;
}

const props = defineProps<{
  modelValue: number[];
  items: AllowlistItem[];
  label: string;
  placeholder: string;
  searchPlaceholder: string;
  helpText?: string;
  loading?: boolean;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', value: number[]): void;
}>();

const open = ref(false);
const searchQuery = ref('');

const filteredItems = computed(() => {
  if (!searchQuery.value.trim()) {
    return props.items;
  }
  const query = searchQuery.value.toLowerCase();
  return props.items.filter(
    (item) =>
      item.name.toLowerCase().includes(query) || (item.description && item.description.toLowerCase().includes(query)),
  );
});

const selectedItems = computed(() => {
  return props.items.filter((item) => props.modelValue.includes(item.id));
});

function isSelected(id: number): boolean {
  return props.modelValue.includes(id);
}

function toggleItem(id: number) {
  const newValue = [...props.modelValue];
  const index = newValue.indexOf(id);
  if (index === -1) {
    newValue.push(id);
  } else {
    newValue.splice(index, 1);
  }
  emit('update:modelValue', newValue);
}

function removeItem(id: number) {
  emit(
    'update:modelValue',
    props.modelValue.filter((v) => v !== id),
  );
}

// Clear search when closing
watch(open, (isOpen) => {
  if (!isOpen) {
    searchQuery.value = '';
  }
});
</script>

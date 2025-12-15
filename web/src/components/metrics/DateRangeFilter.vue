<template>
  <div class="flex items-center gap-2">
    <DropdownMenu>
      <DropdownMenuTrigger as-child>
        <Button variant="outline" size="sm" class="h-9 border-dashed">
          <Icon name="duration" class="mr-2 h-4 w-4" />
          {{ presetLabel }}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem @click="selectPreset('all')">
          {{ $t('metrics.filter.all_time') }}
        </DropdownMenuItem>
        <DropdownMenuItem @click="selectPreset('year')">
          {{ $t('metrics.filter.last_year') }}
        </DropdownMenuItem>
        <DropdownMenuItem @click="selectPreset('quarter')">
          {{ $t('metrics.filter.last_quarter') }}
        </DropdownMenuItem>
        <DropdownMenuItem @click="selectPreset('month')">
          {{ $t('metrics.filter.last_month') }}
        </DropdownMenuItem>
        <DropdownMenuItem @click="selectPreset('week')">
          {{ $t('metrics.filter.last_week') }}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem @click="selectPreset('custom')">
          {{ $t('metrics.filter.custom_range') }}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>

    <Popover v-model:open="showCustomPicker">
      <PopoverTrigger as-child>
        <span />
      </PopoverTrigger>
      <PopoverContent class="w-auto p-4" align="end">
        <div class="space-y-4">
          <div class="font-medium">{{ $t('metrics.filter.custom_range') }}</div>
          <div class="flex flex-col gap-3">
            <div>
              <label class="text-sm text-muted-foreground mb-1 block">{{ $t('metrics.filter.from') }}</label>
              <DatePicker v-model="customAfterDate" :placeholder="$t('metrics.filter.start_date')" class="w-full" />
            </div>
            <div>
              <label class="text-sm text-muted-foreground mb-1 block">{{ $t('metrics.filter.to') }}</label>
              <DatePicker v-model="customBeforeDate" :placeholder="$t('metrics.filter.end_date')" class="w-full" />
            </div>
          </div>
          <div class="flex justify-end gap-2">
            <Button variant="ghost" size="sm" @click="clearCustomRange">
              {{ $t('metrics.filter.clear') }}
            </Button>
            <Button size="sm" @click="applyCustomRange">
              {{ $t('metrics.filter.apply') }}
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import DatePicker from '~/components/ui/date-picker/DatePicker.vue';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';

interface DateRange {
  after?: number; // Unix timestamp
  before?: number; // Unix timestamp
}

const props = defineProps<{
  modelValue: DateRange;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: DateRange];
}>();

const { t } = useI18n();

// Local state for custom date range
const customAfterDate = ref<string | undefined>();
const customBeforeDate = ref<string | undefined>();
const showCustomPicker = ref(false);

// Predefined ranges
type PresetKey = 'all' | 'week' | 'month' | 'quarter' | 'year' | 'custom';
const selectedPreset = ref<PresetKey>('all');

// Determine current preset from modelValue
watch(
  () => props.modelValue,
  (newVal) => {
    if (!newVal.after && !newVal.before) {
      selectedPreset.value = 'all';
    } else {
      // Check if it matches a preset
      const now = Math.floor(Date.now() / 1000);
      const diff = newVal.after ? now - newVal.after : 0;

      if (Math.abs(diff - 7 * 24 * 60 * 60) < 3600) {
        selectedPreset.value = 'week';
      } else if (Math.abs(diff - 30 * 24 * 60 * 60) < 3600) {
        selectedPreset.value = 'month';
      } else if (Math.abs(diff - 90 * 24 * 60 * 60) < 3600) {
        selectedPreset.value = 'quarter';
      } else if (Math.abs(diff - 365 * 24 * 60 * 60) < 3600) {
        selectedPreset.value = 'year';
      } else {
        selectedPreset.value = 'custom';
      }
    }
  },
  { immediate: true },
);

const presetLabel = computed(() => {
  switch (selectedPreset.value) {
    case 'all':
      return t('metrics.filter.all_time');
    case 'week':
      return t('metrics.filter.last_week');
    case 'month':
      return t('metrics.filter.last_month');
    case 'quarter':
      return t('metrics.filter.last_quarter');
    case 'year':
      return t('metrics.filter.last_year');
    case 'custom':
      return t('metrics.filter.custom_range');
    default:
      return t('metrics.filter.all_time');
  }
});

function selectPreset(preset: PresetKey) {
  selectedPreset.value = preset;
  const now = Math.floor(Date.now() / 1000);
  let range: DateRange = {};

  switch (preset) {
    case 'week':
      range.after = now - 7 * 24 * 60 * 60;
      break;
    case 'month':
      range.after = now - 30 * 24 * 60 * 60;
      break;
    case 'quarter':
      range.after = now - 90 * 24 * 60 * 60;
      break;
    case 'year':
      range.after = now - 365 * 24 * 60 * 60;
      break;
    case 'custom':
      showCustomPicker.value = true;
      return;
    case 'all':
    default:
      range = {};
  }

  emit('update:modelValue', range);
}

function applyCustomRange() {
  const range: DateRange = {};

  if (customAfterDate.value) {
    // Convert YYYY-MM-DD to Unix timestamp (start of day)
    const date = new Date(customAfterDate.value);
    range.after = Math.floor(date.getTime() / 1000);
  }

  if (customBeforeDate.value) {
    // Convert YYYY-MM-DD to Unix timestamp (end of day)
    const date = new Date(customBeforeDate.value);
    date.setHours(23, 59, 59, 999);
    range.before = Math.floor(date.getTime() / 1000);
  }

  emit('update:modelValue', range);
  showCustomPicker.value = false;
}

function clearCustomRange() {
  customAfterDate.value = undefined;
  customBeforeDate.value = undefined;
}
</script>

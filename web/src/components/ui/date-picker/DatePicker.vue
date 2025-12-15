<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { CalendarDate, type DateValue, getLocalTimeZone, today } from '@internationalized/date';
import { CalendarRoot } from 'reka-ui';
import { Button } from '~/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import {
  CalendarCell,
  CalendarCellTrigger,
  CalendarGrid,
  CalendarGridBody,
  CalendarGridHead,
  CalendarGridRow,
  CalendarHeadCell,
  CalendarNextButton,
  CalendarPrevButton,
} from '~/components/ui/calendar';
import { cn } from '~/lib/utils';
import Icon from '~/components/atomic/Icon.vue';

const props = defineProps<{
  modelValue?: string;
  placeholder?: string;
  class?: string;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: string | undefined];
}>();

const dateValue = computed<DateValue | undefined>(() => {
  if (!props.modelValue) return undefined;
  const [year, month, day] = props.modelValue.split('-').map(Number);
  return new CalendarDate(year, month, day);
});

function handleDateSelect(date: DateValue | undefined) {
  if (date) {
    const isoString = date.toString(); // CalendarDate.toString() returns YYYY-MM-DD
    emit('update:modelValue', isoString);
  } else {
    emit('update:modelValue', undefined);
  }
}

const displayValue = computed(() => {
  if (!props.modelValue) return props.placeholder || 'Pick a date';
  const date = new Date(props.modelValue);
  return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
});

function setToday() {
  const todayDate = today(getLocalTimeZone());
  handleDateSelect(todayDate);
}

function clearDate() {
  emit('update:modelValue', undefined);
}

// Month/year navigation
const placeholderValue = ref(dateValue.value || today(getLocalTimeZone()));

// Update placeholder when modelValue changes
watch(dateValue, (newValue) => {
  if (newValue) {
    placeholderValue.value = newValue;
  }
});

const months = [
  { value: 1, label: 'January' },
  { value: 2, label: 'February' },
  { value: 3, label: 'March' },
  { value: 4, label: 'April' },
  { value: 5, label: 'May' },
  { value: 6, label: 'June' },
  { value: 7, label: 'July' },
  { value: 8, label: 'August' },
  { value: 9, label: 'September' },
  { value: 10, label: 'October' },
  { value: 11, label: 'November' },
  { value: 12, label: 'December' },
];

const currentYear = new Date().getFullYear();
const years = Array.from({ length: 21 }, (_, i) => currentYear - 10 + i);

function handleMonthChange(value: string) {
  placeholderValue.value = placeholderValue.value.set({ month: Number(value) });
}

function handleYearChange(value: string) {
  placeholderValue.value = placeholderValue.value.set({ year: Number(value) });
}
</script>

<template>
  <Popover>
    <PopoverTrigger as-child>
      <Button
        variant="outline"
        :class="cn('w-full justify-start text-left font-normal', !modelValue && 'text-muted-foreground', props.class)"
      >
        <Icon name="lucide:calendar" class="mr-2 h-4 w-4" />
        {{ displayValue }}
      </Button>
    </PopoverTrigger>
    <PopoverContent class="w-auto max-w-[320px] p-0" align="end">
      <div class="p-3 space-y-2">
        <CalendarRoot
          v-slot="{ grid, weekDays }"
          :model-value="dateValue"
          v-model:placeholder="placeholderValue as any"
          @update:model-value="handleDateSelect"
        >
          <!-- Cycling buttons and Today/Clear buttons in same row -->
          <div class="flex items-center justify-between mb-3">
            <div class="flex items-center gap-1">
              <CalendarPrevButton class="!relative !left-auto" />
              <CalendarNextButton class="!relative !right-auto" />
            </div>
            <div class="flex items-center gap-1">
              <Button variant="ghost" size="sm" class="h-6 px-2 text-xs" @click="setToday"> Today </Button>
              <Button variant="ghost" size="sm" class="h-6 px-2 text-xs" @click="clearDate"> Clear </Button>
            </div>
          </div>

          <!-- Month/Year selectors -->
          <div class="flex items-center justify-center gap-1 pb-3">
            <Select :model-value="String(placeholderValue.month)" @update:model-value="handleMonthChange as any">
              <SelectTrigger class="h-7 text-xs w-[90px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="m in months" :key="m.value" :value="String(m.value)">
                  {{ m.label }}
                </SelectItem>
              </SelectContent>
            </Select>
            <Select :model-value="String(placeholderValue.year)" @update:model-value="handleYearChange as any">
              <SelectTrigger class="h-7 text-xs w-[70px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="y in years" :key="y" :value="String(y)">
                  {{ y }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="flex flex-col gap-y-4">
            <CalendarGrid v-for="month in grid" :key="month.value.toString()">
              <CalendarGridHead>
                <CalendarGridRow>
                  <CalendarHeadCell v-for="day in weekDays" :key="day">
                    {{ day }}
                  </CalendarHeadCell>
                </CalendarGridRow>
              </CalendarGridHead>
              <CalendarGridBody>
                <CalendarGridRow
                  v-for="(weekDates, index) in month.rows"
                  :key="`weekDate-${index}`"
                  class="mt-2 w-full"
                >
                  <CalendarCell v-for="weekDate in weekDates" :key="weekDate.toString()" :date="weekDate">
                    <CalendarCellTrigger :day="weekDate" :month="month.value" />
                  </CalendarCell>
                </CalendarGridRow>
              </CalendarGridBody>
            </CalendarGrid>
          </div>
        </CalendarRoot>
      </div>
    </PopoverContent>
  </Popover>
</template>

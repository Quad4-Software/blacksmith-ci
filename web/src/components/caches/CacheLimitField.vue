<template>
  <div class="space-y-2">
    <Label :for="`${id}-mode`">{{ label }}</Label>
    <div class="flex flex-wrap gap-2">
      <Select :id="`${id}-mode`" :model-value="mode" @update:model-value="setMode">
        <SelectTrigger class="w-48" :data-testid="`${id}-mode`">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-if="!noDefault" value="default">{{
            $t('caches.limit.default', { value: defaultLabel })
          }}</SelectItem>
          <SelectItem value="none">{{ noneLabel }}</SelectItem>
          <SelectItem value="custom">{{ $t('caches.limit.custom') }}</SelectItem>
        </SelectContent>
      </Select>
      <template v-if="mode === 'custom'">
        <Input
          :id="`${id}-amount`"
          v-model.number="amount"
          type="number"
          min="1"
          step="any"
          class="w-28"
          :data-testid="`${id}-amount`"
          @update:model-value="emitCustom"
        />
        <Select :model-value="unit" @update:model-value="setUnit">
          <SelectTrigger class="w-24">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="u in units" :key="u.name" :value="u.name">{{ u.name }}</SelectItem>
          </SelectContent>
        </Select>
      </template>
    </div>
    <p v-if="hint" class="text-xs text-muted-foreground">{{ hint }}</p>
  </div>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { formatCacheSize, formatCacheTTL, limitMode } from '~/lib/build-cache';
import type { LimitMode } from '~/lib/build-cache';

const props = defineProps<{
  id: string;
  label: string;
  kind: 'bytes' | 'seconds';
  // The server default in effect for mode "default", 0 meaning unlimited.
  defaultValue: number;
  hint?: string;
  // Without a server default, 0 is unlimited too, as for the limits of an own bucket.
  noDefault?: boolean;
}>();

// A limit as stored by the server: 0 applies the server default, -1 lifts the limit,
// a positive value is bytes or seconds depending on kind.
const model = defineModel<number>({ required: true });

const { t } = useI18n();

const byteUnits = [
  { name: 'MiB', factor: 1024 ** 2 },
  { name: 'GiB', factor: 1024 ** 3 },
  { name: 'TiB', factor: 1024 ** 4 },
];
const secondUnits = [
  { name: t('caches.limit.hours'), factor: 3600 },
  { name: t('caches.limit.days'), factor: 86400 },
];
const units = computed(() => (props.kind === 'bytes' ? byteUnits : secondUnits));

function modeOf(value: number): LimitMode {
  const mode = limitMode(value);
  return props.noDefault && mode === 'default' ? 'none' : mode;
}

const mode = ref<LimitMode>(modeOf(model.value));
const unit = ref('');
const amount = ref(1);

// Pick the largest unit that renders the value as a whole number.
function split(value: number) {
  const fitting = units.value.toReversed().find((u) => value % u.factor === 0) ?? units.value[0];
  unit.value = fitting.name;
  amount.value = value / fitting.factor;
}

function factor(): number {
  return units.value.find((u) => u.name === unit.value)?.factor ?? units.value[0].factor;
}

watch(
  model,
  (value) => {
    mode.value = modeOf(value);
    if (value > 0 && value !== Math.round(amount.value * factor())) split(value);
  },
  { immediate: true },
);
if (!unit.value) unit.value = units.value[1].name;

function emitCustom() {
  const value = Math.round(Number(amount.value) * factor());
  if (value > 0) model.value = value;
}

function setMode(value: unknown) {
  mode.value = value as LimitMode;
  if (mode.value === 'default') model.value = 0;
  else if (mode.value === 'none') model.value = props.noDefault ? 0 : -1;
  else emitCustom();
}

function setUnit(value: unknown) {
  unit.value = String(value);
  emitCustom();
}

function format(value: number): string {
  return props.kind === 'bytes' ? formatCacheSize(value) : formatCacheTTL(value);
}

const noneLabel = computed(() => (props.kind === 'bytes' ? t('caches.unlimited') : t('caches.limit.keep')));
const defaultLabel = computed(() => (props.defaultValue > 0 ? format(props.defaultValue) : noneLabel.value));
</script>

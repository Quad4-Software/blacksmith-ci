<template>
  <div ref="root" class="flex min-w-0 flex-1 flex-wrap items-center gap-1">
    <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
    <span
      v-for="(chip, i) in chips"
      v-show="i < visibleCount"
      :key="chip.key"
      data-chip
      class="inline-flex shrink-0 items-center rounded-none border border-border bg-muted/40 px-1 py-px font-mono leading-tight"
    >
      <span class="text-[10px] text-muted-foreground/70">{{ chip.key }}=</span>
      <span v-if="chip.present" class="text-[11px]" :style="{ color: matrixValueColor(String(chip.value)) }">{{
        chip.value
      }}</span>
      <span v-else class="text-[11px] text-muted-foreground/60 italic">&mdash;</span>
    </span>
    <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->

    <TooltipProvider v-if="hiddenCount > 0">
      <Tooltip>
        <TooltipTrigger as-child>
          <button
            type="button"
            class="shrink-0 rounded-none border border-border bg-accent px-1 py-px font-mono text-[11px] leading-tight text-muted-foreground hover:text-foreground transition-colors"
            @click.stop
          >
            {{ $t('repo.pipeline.matrix.more_axes', { count: hiddenCount }) }}
          </button>
        </TooltipTrigger>
        <TooltipContent align="start">
          <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
          <div class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 font-mono text-xs">
            <template v-for="chip in chips" :key="chip.key">
              <span class="text-muted-foreground">{{ chip.key }}</span>
              <span v-if="chip.present" :style="{ color: matrixValueColor(String(chip.value)) }">{{ chip.value }}</span>
              <span v-else class="italic opacity-60">&mdash;</span>
            </template>
          </div>
          <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  </div>
</template>

<script lang="ts" setup>
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import { matrixValueColor } from '~/lib/pipeline-utils';
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';

export interface MatrixChip {
  key: string;
  value?: string;
  present: boolean;
}

const props = defineProps<{
  chips: MatrixChip[];
}>();

// Gap between chips (gap-1 = 0.25rem) and space reserved on line 2 for the "+N" pill.
const GAP = 4;
const PILL_RESERVE = 34;
const MAX_LINES = 2;

const root = ref<HTMLElement | null>(null);
const visibleCount = ref(props.chips.length);
const hiddenCount = ref(0);

// Cached natural widths of each chip, measured once while all are visible. Chip
// widths are content-driven and stable, so wrapping can be simulated in JS on
// resize without re-rendering (no flash).
let widths: number[] = [];
let ro: ResizeObserver | null = null;

/**
 * Count how many leading chips fit within MAX_LINES lines of width `w`,
 * reserving `reserve` px on the last line for the overflow pill.
 */
function packCount(w: number, reserve: number): number {
  let line = 1;
  let x = 0;
  let count = 0;
  for (let i = 0; i < widths.length; i++) {
    const cw = widths[i];
    const limit = line === MAX_LINES ? w - reserve : w;
    if (x === 0) {
      // First chip on the current line.
      if (line === MAX_LINES && cw > limit) return count;
      x = cw;
      count++;
    } else if (x + GAP + cw <= limit) {
      x += GAP + cw;
      count++;
    } else {
      // Wrap to the next line.
      if (line === MAX_LINES) return count;
      line++;
      const nextLimit = line === MAX_LINES ? w - reserve : w;
      if (cw > nextLimit) return count;
      x = cw;
      count++;
    }
  }
  return count;
}

function computeFit() {
  const el = root.value;
  if (!el || widths.length === 0) return;
  const w = el.clientWidth;
  if (w <= 0) return;

  const all = packCount(w, 0);
  if (all >= widths.length) {
    visibleCount.value = widths.length;
    hiddenCount.value = 0;
    return;
  }
  const visible = Math.max(1, packCount(w, PILL_RESERVE));
  visibleCount.value = visible;
  hiddenCount.value = widths.length - visible;
}

async function measure() {
  // Show all chips so their natural widths can be read, then simulate the fit.
  visibleCount.value = props.chips.length;
  hiddenCount.value = 0;
  await nextTick();
  const el = root.value;
  if (!el) return;
  const chipEls = [...el.querySelectorAll<HTMLElement>('[data-chip]')];
  widths = chipEls.map((c) => c.offsetWidth);
  computeFit();
}

onMounted(() => {
  measure();
  ro = new ResizeObserver(() => computeFit());
  if (root.value) ro.observe(root.value);
});

onBeforeUnmount(() => {
  ro?.disconnect();
  ro = null;
});

watch(
  () => props.chips,
  () => measure(),
  { deep: true },
);
</script>

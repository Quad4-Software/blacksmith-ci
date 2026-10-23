<template>
  <div class="space-y-3">
    <div class="h-56" data-testid="cache-daily-chart">
      <Bar :data="chartData" :options="chartOptions" :aria-label="$t('caches.daily.aria')" role="img" />
    </div>
    <details class="text-sm">
      <summary class="cursor-pointer text-muted-foreground">{{ $t('caches.daily.table') }}</summary>
      <Table class="mt-2">
        <TableHeader>
          <TableRow>
            <TableHead>{{ $t('caches.daily.day') }}</TableHead>
            <TableHead class="text-right">{{ $t('caches.hits') }}</TableHead>
            <TableHead class="text-right">{{ $t('caches.partial_hits') }}</TableHead>
            <TableHead class="text-right">{{ $t('caches.misses') }}</TableHead>
            <TableHead class="text-right">{{ $t('caches.egress') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="row in withTraffic" :key="row.day">
            <TableCell>{{ dayLabel(row.day) }}</TableCell>
            <TableCell class="text-right">{{ row.hits }}</TableCell>
            <TableCell class="text-right">{{ row.partial_hits }}</TableCell>
            <TableCell class="text-right">{{ row.misses }}</TableCell>
            <TableCell class="text-right">{{ formatCacheSize(row.egress) }}</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </details>
  </div>
</template>

<script lang="ts" setup>
import { BarElement, CategoryScale, Chart as ChartJS, Legend, LinearScale, Tooltip } from 'chart.js';
import type { ChartData, ChartOptions, TooltipItem } from 'chart.js';
import { computed } from 'vue';
import { Bar } from 'vue-chartjs';
import { useI18n } from 'vue-i18n';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import { getCSSVar } from '~/compositions/useMetricsCharts';
import { useTheme } from '~/compositions/useTheme';
import type { BuildCacheUsageDay } from '~/lib/api/types';
import { formatCacheSize } from '~/lib/build-cache';

const props = defineProps<{
  daily: BuildCacheUsageDay[];
  // Number of days shown, ending today.
  days: number;
}>();

ChartJS.register(CategoryScale, LinearScale, BarElement, Tooltip, Legend);

const { t, locale } = useI18n();
const { theme } = useTheme();

const SECONDS_PER_DAY = 86400;

// Hits, restore-key hits and misses; validated for contrast and colour-blind separation on each surface.
const seriesColors = {
  light: ['#388e3c', '#0891b2', '#d9480f'],
  dark: ['#3f9a43', '#0e94b5', '#d97706'],
};

// One row per day of the period, including days without traffic, oldest first.
const rows = computed<BuildCacheUsageDay[]>(() => {
  const byDay = new Map(props.daily.map((row) => [row.day, row]));
  const today = Math.floor(Date.now() / 1000 / SECONDS_PER_DAY) * SECONDS_PER_DAY;
  return Array.from({ length: props.days }, (_, i) => {
    const day = today - (props.days - 1 - i) * SECONDS_PER_DAY;
    return byDay.get(day) ?? { day, hits: 0, partial_hits: 0, misses: 0, denied: 0, saves: 0, egress: 0, ingress: 0 };
  });
});

const withTraffic = computed(() =>
  rows.value.filter((row) => row.hits + row.partial_hits + row.misses + row.denied + row.saves > 0).toReversed(),
);

function dayLabel(day: number): string {
  return new Date(day * 1000).toLocaleDateString(locale.value, { month: 'short', day: 'numeric', timeZone: 'UTC' });
}

const chartData = computed<ChartData<'bar'>>(() => {
  const colors = seriesColors[theme.value === 'dark' ? 'dark' : 'light'];
  const surface = getCSSVar('--card') || (theme.value === 'dark' ? '#2d313d' : '#ffffff');
  const series: { label: string; values: number[] }[] = [
    { label: t('caches.hits'), values: rows.value.map((row) => row.hits) },
    { label: t('caches.partial_hits'), values: rows.value.map((row) => row.partial_hits) },
    { label: t('caches.misses'), values: rows.value.map((row) => row.misses + row.denied) },
  ];
  return {
    labels: rows.value.map((row) => dayLabel(row.day)),
    datasets: series.map((s, i) => ({
      label: s.label,
      data: s.values,
      backgroundColor: colors[i],
      borderColor: surface,
      borderWidth: { top: 2 },
      borderRadius: i === series.length - 1 ? { topLeft: 4, topRight: 4 } : 0,
      borderSkipped: 'bottom',
      maxBarThickness: 18,
    })),
  };
});

const chartOptions = computed<ChartOptions<'bar'>>(() => {
  const border = getCSSVar('--border') || '#434858';
  const muted = getCSSVar('--muted-foreground') || '#9ca3af';
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index', intersect: false },
    plugins: {
      legend: { position: 'top', align: 'start', labels: { color: muted, boxWidth: 12, boxHeight: 12 } },
      tooltip: {
        callbacks: {
          footer: (items: TooltipItem<'bar'>[]) => {
            const row = rows.value[items[0]?.dataIndex ?? 0];
            return row ? `${t('caches.egress')}: ${formatCacheSize(row.egress)}` : '';
          },
        },
      },
    },
    scales: {
      x: {
        stacked: true,
        grid: { display: false },
        ticks: { color: muted, maxTicksLimit: 8 },
        border: { color: border },
      },
      y: {
        stacked: true,
        beginAtZero: true,
        grid: { color: border },
        border: { display: false },
        ticks: { color: muted, precision: 0 },
        title: { display: true, text: t('caches.daily.restores'), color: muted },
      },
    },
  };
});
</script>

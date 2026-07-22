<template>
  <div class="space-y-4">
    <!-- Controls -->
    <Card>
      <CardContent>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div class="flex flex-col gap-1">
            <label class="text-sm text-muted-foreground">{{ $t('metrics.builder.dimension') }}</label>
            <Select v-model="dimension">
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="o in dimensionOptions" :key="o.value" :value="o.value">{{ o.label }}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="flex flex-col gap-1">
            <label class="text-sm text-muted-foreground">{{ $t('metrics.builder.metric') }}</label>
            <Select v-model="metric">
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="o in metricOptions" :key="o.value" :value="o.value">{{ o.label }}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div v-if="dimension === 'time'" class="flex flex-col gap-1">
            <label class="text-sm text-muted-foreground">{{ $t('metrics.builder.bucket') }}</label>
            <Select v-model="bucket">
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="o in bucketOptions" :key="o.value" :value="o.value">{{ o.label }}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="flex flex-col gap-1">
            <label class="text-sm text-muted-foreground">{{ $t('metrics.builder.chart_type') }}</label>
            <Select v-model="chartType">
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="o in chartTypeOptions" :key="o.value" :value="o.value">{{ o.label }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Result -->
    <Card>
      <CardContent>
        <div v-if="loading" class="flex justify-center py-16">
          <Icon name="spinner" class="h-8 w-8 animate-spin" />
        </div>
        <div v-else-if="error" class="py-16 text-center text-muted-foreground">
          {{ error }}
        </div>
        <div v-else-if="!hasData" class="py-16 text-center text-muted-foreground">
          {{ $t('metrics.builder.no_data') }}
        </div>
        <component :is="chartType === 'line' ? Line : Bar" v-else :data="chartData" :options="chartOptions" />
      </CardContent>
    </Card>
  </div>
</template>

<script lang="ts" setup>
import { useStorage, watchDebounced } from '@vueuse/core';
import Icon from '~/components/atomic/Icon.vue';
import { Card, CardContent } from '~/components/ui/card';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import useApiClient from '~/compositions/useApiClient';
import { buildChartOptions, buildSeriesData, maxOf } from '~/compositions/useMetricsCharts';
import type { MetricBucket, MetricDimension, MetricName, MetricSeries } from '~/lib/api/types';
import {
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  Title,
  Tooltip,
} from 'chart.js';
import { computed, onMounted, ref, watch } from 'vue';
import { Bar, Line } from 'vue-chartjs';
import { useI18n } from 'vue-i18n';

const props = defineProps<{
  dateRange: { after?: number; before?: number };
  repoIds: number[];
  showAll: boolean;
}>();

const { t } = useI18n();
const apiClient = useApiClient();

ChartJS.register(CategoryScale, LinearScale, BarElement, LineElement, PointElement, Title, Tooltip, Legend);

// Which metrics are valid per dimension — mirrors the server-side matrix.
const metricsByDimension: Record<MetricDimension, MetricName[]> = {
  repo: ['pipeline_count', 'workflow_count', 'avg_build_time', 'total_build_time', 'avg_steps'],
  org: ['pipeline_count', 'workflow_count', 'avg_build_time', 'total_build_time', 'avg_steps'],
  agent: ['workflow_count', 'avg_build_time', 'total_build_time'],
  time: ['pipeline_count', 'workflow_count', 'avg_build_time', 'total_build_time', 'avg_steps'],
};

const dimension = useStorage<MetricDimension>('crow:metrics-builder-dimension', 'repo');
const metric = useStorage<MetricName>('crow:metrics-builder-metric', 'pipeline_count');
const bucket = useStorage<MetricBucket>('crow:metrics-builder-bucket', 'day');
const chartType = useStorage<'bar' | 'line'>('crow:metrics-builder-chart-type', 'bar');

// Explicit (static) i18n keys so the linter can verify usage.
const dimensionOptions = computed<{ value: MetricDimension; label: string }[]>(() => [
  { value: 'repo', label: t('metrics.builder.dimensions.repo') },
  { value: 'org', label: t('metrics.builder.dimensions.org') },
  { value: 'agent', label: t('metrics.builder.dimensions.agent') },
  { value: 'time', label: t('metrics.builder.dimensions.time') },
]);

const metricLabels = computed<Record<MetricName, string>>(() => ({
  pipeline_count: t('metrics.builder.metrics.pipeline_count'),
  workflow_count: t('metrics.builder.metrics.workflow_count'),
  avg_build_time: t('metrics.builder.metrics.avg_build_time'),
  total_build_time: t('metrics.builder.metrics.total_build_time'),
  avg_steps: t('metrics.builder.metrics.avg_steps'),
}));

const bucketOptions = computed<{ value: MetricBucket; label: string }[]>(() => [
  { value: 'day', label: t('metrics.builder.buckets.day') },
  { value: 'week', label: t('metrics.builder.buckets.week') },
  { value: 'month', label: t('metrics.builder.buckets.month') },
]);

const chartTypeOptions = computed<{ value: 'bar' | 'line'; label: string }[]>(() => [
  { value: 'bar', label: t('metrics.builder.chart_types.bar') },
  { value: 'line', label: t('metrics.builder.chart_types.line') },
]);

const metricOptions = computed(() =>
  metricsByDimension[dimension.value].map((m) => ({ value: m, label: metricLabels.value[m] })),
);

const series = ref<MetricSeries>();
const loading = ref(false);
const error = ref('');

const hasData = computed(() => (series.value?.points.length ?? 0) > 0);

// Repo/org bars have long labels, so render them horizontally; agent/time (and
// all line charts) render vertically.
const orientation = computed<'vertical' | 'horizontal'>(() =>
  chartType.value === 'bar' && (dimension.value === 'repo' || dimension.value === 'org') ? 'horizontal' : 'vertical',
);

const chartData = computed(() => {
  const points = series.value?.points ?? [];
  return buildSeriesData(
    points.map((p) => p.label),
    points.map((p) => p.value),
    { label: metricLabels.value[metric.value], colorIndex: 0, type: chartType.value },
  );
});

const chartOptions = computed(() => {
  const unit = series.value?.unit === 'seconds' ? 'seconds' : 'count';
  return buildChartOptions({
    unit,
    orientation: orientation.value,
    maxValue: maxOf((series.value?.points ?? []).map((p) => p.value)),
    axisTitle:
      unit === 'seconds'
        ? t('admin.settings.metrics.charts.axis.duration')
        : t('admin.settings.metrics.charts.axis.count'),
  });
});

// Keep the selected metric valid when the dimension changes, and default the
// chart type to line for the time dimension.
watch(dimension, (dim) => {
  if (!metricsByDimension[dim].includes(metric.value)) {
    metric.value = metricsByDimension[dim][0];
  }
  if (dim === 'time') {
    chartType.value = 'line';
  }
});

async function loadSeries() {
  loading.value = true;
  error.value = '';
  try {
    series.value = await apiClient.queryMetrics({
      dimension: dimension.value,
      metric: metric.value,
      bucket: dimension.value === 'time' ? bucket.value : undefined,
      after: props.dateRange.after,
      before: props.dateRange.before,
      all: props.showAll || undefined,
      repos: !props.showAll && props.repoIds.length > 0 ? props.repoIds.join(',') : undefined,
    });
  } catch {
    error.value = t('metrics.builder.invalid_combo');
    series.value = undefined;
  } finally {
    loading.value = false;
  }
}

watchDebounced(
  [dimension, metric, bucket, () => props.dateRange, () => props.repoIds, () => props.showAll],
  loadSeries,
  {
    debounce: 250,
    deep: true,
  },
);

onMounted(() => {
  // Repair a persisted invalid combination before the first fetch.
  if (!metricsByDimension[dimension.value].includes(metric.value)) {
    metric.value = metricsByDimension[dimension.value][0];
  }
  loadSeries();
});
</script>

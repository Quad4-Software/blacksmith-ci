<template>
  <div v-if="metrics" class="space-y-4">
    <!-- Workflows by Agent -->
    <Card v-if="topAgents.length > 0">
      <CardHeader>
        <CardTitle>{{ $t('admin.settings.metrics.charts.workflows_by_agent') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <Bar :data="agentsChartData" :options="agentsOptions" />
      </CardContent>
    </Card>

    <!-- Pipelines by Repo (Top 10) -->
    <Card v-if="topRepos.length > 0">
      <CardHeader>
        <CardTitle>{{ $t('admin.settings.metrics.charts.pipelines_by_repo') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <Bar :data="reposChartData" :options="reposOptions" />
      </CardContent>
    </Card>

    <!-- Pipelines by Org -->
    <Card v-if="topOrgs.length > 0">
      <CardHeader>
        <CardTitle>{{ $t('admin.settings.metrics.charts.pipelines_by_org') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <Bar :data="orgsChartData" :options="orgsOptions" />
      </CardContent>
    </Card>

    <!-- Average Build Time by Repo -->
    <Card v-if="topRepos.length > 0">
      <CardHeader>
        <CardTitle>{{ $t('admin.settings.metrics.charts.avg_build_time_by_repo') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <Bar :data="repoAvgChartData" :options="repoAvgOptions" />
      </CardContent>
    </Card>

    <!-- Total Build Time by Repo -->
    <Card v-if="topRepos.length > 0">
      <CardHeader>
        <CardTitle>{{ $t('admin.settings.metrics.charts.total_build_time_by_repo') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <Bar :data="repoTotalChartData" :options="repoTotalOptions" />
      </CardContent>
    </Card>

    <!-- Average Build Time by Org -->
    <Card v-if="topOrgs.length > 0">
      <CardHeader>
        <CardTitle>{{ $t('admin.settings.metrics.charts.avg_build_time_by_org') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <Bar :data="orgAvgChartData" :options="orgAvgOptions" />
      </CardContent>
    </Card>

    <!-- Total Build Time by Org -->
    <Card v-if="topOrgs.length > 0">
      <CardHeader>
        <CardTitle>{{ $t('admin.settings.metrics.charts.total_build_time_by_org') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <Bar :data="orgTotalChartData" :options="orgTotalOptions" />
      </CardContent>
    </Card>
  </div>
</template>

<script lang="ts" setup>
import { Card, CardContent, CardHeader, CardTitle } from '~/components/ui/card';
import { buildChartOptions, buildSeriesData, maxOf } from '~/compositions/useMetricsCharts';
import type { AdminMetrics } from '~/lib/api/types';
import { BarElement, CategoryScale, Chart as ChartJS, Legend, LinearScale, Title, Tooltip } from 'chart.js';
import { computed } from 'vue';
import { Bar } from 'vue-chartjs';
import { useI18n } from 'vue-i18n';

const props = defineProps<{
  metrics: AdminMetrics | undefined;
}>();

const { t } = useI18n();

ChartJS.register(CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend);

const topAgents = computed(() => {
  if (!props.metrics?.agent_details) return [];
  return props.metrics.agent_details.toSorted((a, b) => b.workflow_count - a.workflow_count).slice(0, 10);
});

const topRepos = computed(() => {
  if (!props.metrics?.repo_details) return [];
  return props.metrics.repo_details.toSorted((a, b) => b.pipeline_count - a.pipeline_count).slice(0, 10);
});

const topOrgs = computed(() => {
  if (!props.metrics?.org_details) return [];
  return props.metrics.org_details.toSorted((a, b) => b.pipeline_count - a.pipeline_count).slice(0, 10);
});

const countAxis = computed(() => t('admin.settings.metrics.charts.axis.count'));
const durationAxis = computed(() => t('admin.settings.metrics.charts.axis.duration'));

// Workflows by Agent
const agentValues = computed(() => topAgents.value.map((a) => a.workflow_count));
const agentsChartData = computed(() =>
  buildSeriesData(
    topAgents.value.map((a) => a.agent_name),
    agentValues.value,
    { label: t('admin.settings.metrics.charts.dataset.workflows'), colorIndex: 0 },
  ),
);
const agentsOptions = computed(() =>
  buildChartOptions({
    unit: 'count',
    orientation: 'vertical',
    maxValue: maxOf(agentValues.value),
    axisTitle: countAxis.value,
  }),
);

// Pipelines by Repo (horizontal)
const repoPipelineValues = computed(() => topRepos.value.map((r) => r.pipeline_count));
const reposChartData = computed(() =>
  buildSeriesData(
    topRepos.value.map((r) => r.repo_full_name),
    repoPipelineValues.value,
    { label: t('admin.settings.metrics.charts.dataset.pipelines'), colorIndex: 1 },
  ),
);
const reposOptions = computed(() =>
  buildChartOptions({
    unit: 'count',
    orientation: 'horizontal',
    maxValue: maxOf(repoPipelineValues.value),
    axisTitle: countAxis.value,
  }),
);

// Pipelines by Org
const orgPipelineValues = computed(() => topOrgs.value.map((o) => o.pipeline_count));
const orgsChartData = computed(() =>
  buildSeriesData(
    topOrgs.value.map((o) => o.org_name),
    orgPipelineValues.value,
    { label: t('admin.settings.metrics.charts.dataset.pipelines'), colorIndex: 2 },
  ),
);
const orgsOptions = computed(() =>
  buildChartOptions({
    unit: 'count',
    orientation: 'vertical',
    maxValue: maxOf(orgPipelineValues.value),
    axisTitle: countAxis.value,
  }),
);

// Average Build Time by Repo (horizontal, seconds)
const repoAvgValues = computed(() => topRepos.value.map((r) => r.avg_build_time));
const repoAvgChartData = computed(() =>
  buildSeriesData(
    topRepos.value.map((r) => r.repo_full_name),
    repoAvgValues.value,
    { label: t('admin.settings.metrics.charts.dataset.avg_build_time'), colorIndex: 3 },
  ),
);
const repoAvgOptions = computed(() =>
  buildChartOptions({
    unit: 'seconds',
    orientation: 'horizontal',
    maxValue: maxOf(repoAvgValues.value),
    axisTitle: durationAxis.value,
  }),
);

// Total Build Time by Repo (horizontal, seconds)
const repoTotalValues = computed(() => topRepos.value.map((r) => r.total_build_time));
const repoTotalChartData = computed(() =>
  buildSeriesData(
    topRepos.value.map((r) => r.repo_full_name),
    repoTotalValues.value,
    { label: t('admin.settings.metrics.charts.dataset.total_build_time'), colorIndex: 4 },
  ),
);
const repoTotalOptions = computed(() =>
  buildChartOptions({
    unit: 'seconds',
    orientation: 'horizontal',
    maxValue: maxOf(repoTotalValues.value),
    axisTitle: durationAxis.value,
  }),
);

// Average Build Time by Org (vertical, seconds)
const orgAvgValues = computed(() => topOrgs.value.map((o) => o.avg_build_time));
const orgAvgChartData = computed(() =>
  buildSeriesData(
    topOrgs.value.map((o) => o.org_name),
    orgAvgValues.value,
    { label: t('admin.settings.metrics.charts.dataset.avg_build_time'), colorIndex: 5 },
  ),
);
const orgAvgOptions = computed(() =>
  buildChartOptions({
    unit: 'seconds',
    orientation: 'vertical',
    maxValue: maxOf(orgAvgValues.value),
    axisTitle: durationAxis.value,
  }),
);

// Total Build Time by Org (vertical, seconds)
const orgTotalValues = computed(() => topOrgs.value.map((o) => o.total_build_time));
const orgTotalChartData = computed(() =>
  buildSeriesData(
    topOrgs.value.map((o) => o.org_name),
    orgTotalValues.value,
    { label: t('admin.settings.metrics.charts.dataset.total_build_time'), colorIndex: 6 },
  ),
);
const orgTotalOptions = computed(() =>
  buildChartOptions({
    unit: 'seconds',
    orientation: 'vertical',
    maxValue: maxOf(orgTotalValues.value),
    axisTitle: durationAxis.value,
  }),
);
</script>

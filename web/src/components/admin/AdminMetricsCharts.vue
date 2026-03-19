<template>
  <div v-if="metrics" class="space-y-4">
    <!-- Workflows by Agent -->
    <Card v-if="topAgents.length > 0">
      <CardHeader>
        <CardTitle>{{ $t('admin.settings.metrics.charts.workflows_by_agent') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <Bar :data="agentsChartData" :options="barOptions" />
      </CardContent>
    </Card>

    <!-- Pipelines by Repo (Top 10) -->
    <Card v-if="topRepos.length > 0">
      <CardHeader>
        <CardTitle>{{ $t('admin.settings.metrics.charts.pipelines_by_repo') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <Bar :data="reposChartData" :options="horizontalBarOptions" />
      </CardContent>
    </Card>

    <!-- Pipelines by Org -->
    <Card v-if="topOrgs.length > 0">
      <CardHeader>
        <CardTitle>{{ $t('admin.settings.metrics.charts.pipelines_by_org') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <Bar :data="orgsBarChartData" :options="barOptions" />
      </CardContent>
    </Card>

    <!-- Average Build Time by Repo -->
    <Card v-if="topRepos.length > 0">
      <CardHeader>
        <CardTitle>{{ $t('admin.settings.metrics.charts.avg_build_time_by_repo') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <Bar :data="repoAvgBuildTimeChartData" :options="horizontalTimeBarOptions" />
      </CardContent>
    </Card>

    <!-- Total Build Time by Repo -->
    <Card v-if="topRepos.length > 0">
      <CardHeader>
        <CardTitle>{{ $t('admin.settings.metrics.charts.total_build_time_by_repo') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <Bar :data="repoTotalBuildTimeChartData" :options="horizontalTimeBarOptions" />
      </CardContent>
    </Card>

    <!-- Average Build Time by Org -->
    <Card v-if="topOrgs.length > 0">
      <CardHeader>
        <CardTitle>{{ $t('admin.settings.metrics.charts.avg_build_time_by_org') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <Bar :data="orgAvgBuildTimeChartData" :options="timeBarOptions" />
      </CardContent>
    </Card>

    <!-- Total Build Time by Org -->
    <Card v-if="topOrgs.length > 0">
      <CardHeader>
        <CardTitle>{{ $t('admin.settings.metrics.charts.total_build_time_by_org') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <Bar :data="orgTotalBuildTimeChartData" :options="timeBarOptions" />
      </CardContent>
    </Card>
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import { BarElement, CategoryScale, Chart as ChartJS, Legend, LinearScale, Title, Tooltip } from 'chart.js';
import { Bar } from 'vue-chartjs';

import { Card, CardContent, CardHeader, CardTitle } from '~/components/ui/card';
import type { AdminMetrics } from '~/lib/api/types';

const props = defineProps<{
  metrics: AdminMetrics | undefined;
}>();

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

// Crow CI color palette - using theme colors
const colors = [
  '#4caf50', // primary green
  '#0891b2', // cyan/info
  '#ff9b00', // highlight orange
  '#16a34a', // state ok green
  '#facc15', // warn yellow
  '#ea580c', // hover orange
  '#8ad97f', // light green
  '#68c464', // primary 200
  '#369943', // primary 400
  '#248438', // primary 500
];

const agentsChartData = computed(() => ({
  labels: topAgents.value.map((a) => a.agent_name),
  datasets: [
    {
      label: 'Workflows',
      data: topAgents.value.map((a) => a.workflow_count),
      backgroundColor: colors[0],
      borderWidth: 0,
      borderRadius: 4,
      maxBarThickness: 40,
    },
  ],
}));

const reposChartData = computed(() => ({
  labels: topRepos.value.map((r) => r.repo_full_name),
  datasets: [
    {
      label: 'Pipelines',
      data: topRepos.value.map((r) => r.pipeline_count),
      backgroundColor: colors[1],
      borderWidth: 0,
      borderRadius: 4,
      maxBarThickness: 40,
    },
  ],
}));

const orgsBarChartData = computed(() => ({
  labels: topOrgs.value.map((o) => o.org_name),
  datasets: [
    {
      label: 'Pipelines',
      data: topOrgs.value.map((o) => o.pipeline_count),
      backgroundColor: colors[2],
      borderWidth: 0,
      borderRadius: 4,
      maxBarThickness: 40,
    },
  ],
}));

const repoAvgBuildTimeChartData = computed(() => ({
  labels: topRepos.value.map((r) => r.repo_full_name),
  datasets: [
    {
      label: 'Avg Build Time',
      data: topRepos.value.map((r) => r.avg_build_time),
      backgroundColor: colors[3],
      borderWidth: 0,
      borderRadius: 4,
      maxBarThickness: 40,
    },
  ],
}));

const repoTotalBuildTimeChartData = computed(() => ({
  labels: topRepos.value.map((r) => r.repo_full_name),
  datasets: [
    {
      label: 'Total Build Time',
      data: topRepos.value.map((r) => r.total_build_time),
      backgroundColor: colors[4],
      borderWidth: 0,
      borderRadius: 4,
      maxBarThickness: 40,
    },
  ],
}));

const orgAvgBuildTimeChartData = computed(() => ({
  labels: topOrgs.value.map((o) => o.org_name),
  datasets: [
    {
      label: 'Avg Build Time',
      data: topOrgs.value.map((o) => o.avg_build_time),
      backgroundColor: colors[5],
      borderWidth: 0,
      borderRadius: 4,
      maxBarThickness: 40,
    },
  ],
}));

const orgTotalBuildTimeChartData = computed(() => ({
  labels: topOrgs.value.map((o) => o.org_name),
  datasets: [
    {
      label: 'Total Build Time',
      data: topOrgs.value.map((o) => o.total_build_time),
      backgroundColor: colors[6],
      borderWidth: 0,
      borderRadius: 4,
      maxBarThickness: 40,
    },
  ],
}));

function formatDuration(seconds: number): string {
  if (!seconds || seconds === 0) return '0s';

  const absSecs = Math.abs(seconds);

  // For very large durations, use days
  if (absSecs >= 86400) {
    const days = Math.floor(absSecs / 86400);
    const hours = Math.floor((absSecs % 86400) / 3600);
    if (hours > 0) {
      return `${days}d ${hours}h`;
    }
    return `${days}d`;
  }

  // For medium durations, use hours and minutes
  if (absSecs >= 3600) {
    const hours = Math.floor(absSecs / 3600);
    const minutes = Math.floor((absSecs % 3600) / 60);
    if (minutes > 0) {
      return `${hours}h ${minutes}m`;
    }
    return `${hours}h`;
  }

  // For small durations, use minutes and seconds
  if (absSecs >= 60) {
    const minutes = Math.floor(absSecs / 60);
    const secs = Math.floor(absSecs % 60);
    if (secs > 0) {
      return `${minutes}m ${secs}s`;
    }
    return `${minutes}m`;
  }

  // For very small durations, just show seconds
  return `${Math.floor(absSecs)}s`;
}

// Get CSS variable value
function getCSSVar(varName: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(varName).trim();
}

// Chart.js options with Crow CI theming
const barOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: true,
  aspectRatio: 2,
  plugins: {
    legend: {
      display: false,
    },
    tooltip: {
      backgroundColor: getCSSVar('--popover') || '#2d313d',
      titleColor: getCSSVar('--foreground') || '#ffffff',
      bodyColor: getCSSVar('--foreground') || '#ffffff',
      borderColor: getCSSVar('--border') || '#434858',
      borderWidth: 1,
      padding: 12,
      displayColors: false,
      callbacks: {
        label: (context: any) => {
          const label = context.dataset.label || '';
          const value = context.parsed.y;
          // Check if this is a time-based chart
          if (label.toLowerCase().includes('time')) {
            return `${label}: ${formatDuration(value)}`;
          }
          return `${label}: ${value}`;
        },
      },
    },
  },
  scales: {
    x: {
      grid: {
        display: false,
        color: getCSSVar('--border') || '#434858',
      },
      ticks: {
        color: getCSSVar('--muted-foreground') || '#9ca3af',
      },
      border: {
        color: getCSSVar('--border') || '#434858',
      },
    },
    y: {
      grid: {
        color: getCSSVar('--border') || '#434858',
      },
      ticks: {
        color: getCSSVar('--muted-foreground') || '#9ca3af',
        callback(value: any) {
          return value;
        },
      },
      border: {
        color: getCSSVar('--border') || '#434858',
      },
      title: {
        display: true,
        text: 'Count',
        color: getCSSVar('--muted-foreground') || '#9ca3af',
      },
    },
  },
}));

const horizontalBarOptions = computed(() => ({
  ...barOptions.value,
  indexAxis: 'y' as const,
  aspectRatio: 1.5,
  scales: {
    x: {
      grid: {
        color: getCSSVar('--border') || '#434858',
      },
      ticks: {
        color: getCSSVar('--muted-foreground') || '#9ca3af',
      },
      border: {
        color: getCSSVar('--border') || '#434858',
      },
      title: {
        display: true,
        text: 'Count',
        color: getCSSVar('--muted-foreground') || '#9ca3af',
      },
    },
    y: {
      grid: {
        display: false,
      },
      ticks: {
        color: getCSSVar('--muted-foreground') || '#9ca3af',
      },
      border: {
        color: getCSSVar('--border') || '#434858',
      },
    },
  },
}));

// Options for time-based charts (vertical bars)
const timeBarOptions = computed(() => ({
  ...barOptions.value,
  scales: {
    ...barOptions.value.scales,
    y: {
      ...barOptions.value.scales.y,
      title: {
        display: true,
        text: 'Time (minutes)',
        color: getCSSVar('--muted-foreground') || '#9ca3af',
      },
      ticks: {
        color: getCSSVar('--muted-foreground') || '#9ca3af',
        callback(value: any) {
          // Convert seconds to minutes for display, show 1 decimal to avoid duplicates
          return (value / 60).toFixed(1);
        },
      },
    },
  },
}));

// Options for time-based charts (horizontal bars)
const horizontalTimeBarOptions = computed(() => ({
  ...barOptions.value,
  indexAxis: 'y' as const,
  aspectRatio: 1.5,
  scales: {
    x: {
      grid: {
        color: getCSSVar('--border') || '#434858',
      },
      ticks: {
        color: getCSSVar('--muted-foreground') || '#9ca3af',
        callback(value: any) {
          // Convert seconds to minutes for display, show 1 decimal to avoid duplicates
          return (value / 60).toFixed(1);
        },
      },
      border: {
        color: getCSSVar('--border') || '#434858',
      },
      title: {
        display: true,
        text: 'Time (minutes)',
        color: getCSSVar('--muted-foreground') || '#9ca3af',
      },
    },
    y: {
      grid: {
        display: false,
      },
      ticks: {
        color: getCSSVar('--muted-foreground') || '#9ca3af',
      },
      border: {
        color: getCSSVar('--border') || '#434858',
      },
    },
  },
}));
</script>

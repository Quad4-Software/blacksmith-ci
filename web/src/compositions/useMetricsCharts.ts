// Copyright 2026 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Shared chart helpers for the metrics dashboard and the chart builder. Keeping
// them here means the fixed charts (AdminMetricsCharts.vue) and the dynamic
// ChartBuilder.vue render identically and share the "nice duration axis" fix.

import type { TooltipItem } from 'chart.js';

export type MetricChartUnit = 'count' | 'seconds';
export type MetricChartOrientation = 'vertical' | 'horizontal';

// Crow CI palette — reused for categorical series.
export const chartColors = [
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

// Ascending ladder of "nice" durations (seconds) used to snap value-axis ticks
// to round intervals (15s, 1m, 5m, 1h, 1d, 1w, …) instead of arbitrary numbers.
const DURATION_STEP_LADDER = [
  15, 30, 60, 120, 300, 600, 900, 1800, 3600, 7200, 10800, 21600, 43200, 86400, 172800, 604800,
];
const SECONDS_PER_DAY = 86400;

// getCSSVar reads a CSS custom property from the document root so charts follow
// the active theme. Evaluated lazily (inside computed chart options).
export function getCSSVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

// formatDuration renders a seconds value as a compact human string (e.g. "1h 23m").
export function formatDuration(seconds: number): string {
  if (!seconds || seconds === 0) return '0s';

  const absSecs = Math.abs(seconds);

  if (absSecs >= SECONDS_PER_DAY) {
    const days = Math.floor(absSecs / SECONDS_PER_DAY);
    const hours = Math.floor((absSecs % SECONDS_PER_DAY) / 3600);
    return hours > 0 ? `${days}d ${hours}h` : `${days}d`;
  }
  if (absSecs >= 3600) {
    const hours = Math.floor(absSecs / 3600);
    const minutes = Math.floor((absSecs % 3600) / 60);
    return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`;
  }
  if (absSecs >= 60) {
    const minutes = Math.floor(absSecs / 60);
    const secs = Math.floor(absSecs % 60);
    return secs > 0 ? `${minutes}m ${secs}s` : `${minutes}m`;
  }
  return `${Math.floor(absSecs)}s`;
}

// niceDurationStep picks a round tick interval (seconds) so a value axis up to
// maxSeconds shows about targetTicks gridlines on human-friendly boundaries.
export function niceDurationStep(maxSeconds: number, targetTicks = 6): number {
  if (maxSeconds <= 0) return 60;
  const step = DURATION_STEP_LADDER.find((candidate) => maxSeconds / candidate <= targetTicks);
  if (step !== undefined) return step;
  // Above the ladder: round up to a whole number of days.
  return Math.ceil(maxSeconds / (targetTicks * SECONDS_PER_DAY)) * SECONDS_PER_DAY;
}

export interface ChartOptionsConfig {
  unit: MetricChartUnit;
  maxValue: number;
  axisTitle: string;
  orientation?: MetricChartOrientation;
}

function themeColors() {
  return {
    border: getCSSVar('--border') || '#434858',
    muted: getCSSVar('--muted-foreground') || '#9ca3af',
    popover: getCSSVar('--popover') || '#2d313d',
    foreground: getCSSVar('--foreground') || '#ffffff',
  };
}

function valueAxis(unit: MetricChartUnit, maxValue: number, axisTitle: string) {
  const { border, muted } = themeColors();
  const axis: Record<string, unknown> = {
    beginAtZero: true,
    grid: { color: border },
    border: { color: border },
    title: { display: true, text: axisTitle, color: muted },
  };

  if (unit === 'seconds') {
    const step = niceDurationStep(maxValue);
    axis.ticks = {
      color: muted,
      stepSize: step,
      callback: (value: number | string) => formatDuration(Number(value)),
    };
    if (maxValue > 0) {
      axis.max = Math.ceil(maxValue / step) * step;
    }
  } else {
    axis.ticks = {
      color: muted,
      precision: 0,
      callback: (value: number | string) => Number(value).toLocaleString(),
    };
  }
  return axis;
}

function categoryAxis() {
  const { border, muted } = themeColors();
  return {
    grid: { display: false, color: border },
    border: { color: border },
    ticks: { color: muted },
  };
}

function tooltip(unit: MetricChartUnit, horizontal: boolean) {
  const { border, popover, foreground } = themeColors();
  return {
    backgroundColor: popover,
    titleColor: foreground,
    bodyColor: foreground,
    borderColor: border,
    borderWidth: 1,
    padding: 12,
    displayColors: false,
    callbacks: {
      label: (context: TooltipItem<'bar' | 'line'>) => {
        const label = context.dataset.label ?? '';
        const parsed = context.parsed as { x: number; y: number };
        const value = horizontal ? parsed.x : parsed.y;
        if (unit === 'seconds') {
          return `${label}: ${formatDuration(value)}`;
        }
        return `${label}: ${value.toLocaleString()}`;
      },
    },
  };
}

// buildChartOptions returns Chart.js options with Crow theming and, for duration
// axes, round tick steps. Chart data stays in raw seconds so tooltips stay exact;
// only the tick *formatting* changes — this replaces the old `(v/60).toFixed(1)`
// that produced axis labels like 23.3 and 233333.3.
export function buildChartOptions(config: ChartOptionsConfig) {
  const { unit, maxValue, axisTitle, orientation = 'vertical' } = config;
  const horizontal = orientation === 'horizontal';
  const value = valueAxis(unit, maxValue, axisTitle);
  const category = categoryAxis();

  return {
    responsive: true,
    maintainAspectRatio: true,
    aspectRatio: horizontal ? 1.5 : 2,
    indexAxis: horizontal ? ('y' as const) : ('x' as const),
    plugins: {
      legend: { display: false },
      tooltip: tooltip(unit, horizontal),
    },
    scales: horizontal ? { x: value, y: category } : { x: category, y: value },
  };
}

export interface SeriesDataOptions {
  label: string;
  colorIndex?: number;
  type?: 'bar' | 'line';
}

// buildSeriesData maps labels + values into a single-dataset Chart.js structure.
export function buildSeriesData(labels: string[], values: number[], options: SeriesDataOptions) {
  const color = chartColors[(options.colorIndex ?? 0) % chartColors.length];
  const isLine = options.type === 'line';
  return {
    labels,
    datasets: [
      {
        label: options.label,
        data: values,
        backgroundColor: color,
        borderColor: color,
        borderWidth: isLine ? 2 : 0,
        borderRadius: isLine ? 0 : 4,
        maxBarThickness: 40,
        tension: isLine ? 0.3 : 0,
        pointRadius: isLine ? 3 : 0,
        fill: false,
      },
    ],
  };
}

// maxOf returns the largest value in the list (0 when empty), for axis scaling.
export function maxOf(values: number[]): number {
  return values.reduce((max, v) => (v > max ? v : max), 0);
}

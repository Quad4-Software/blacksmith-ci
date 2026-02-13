<template>
  <div class="w-full h-full min-h-0 flex flex-col">
    <!-- Search and Filter Controls -->
    <div
      class="flex flex-col md:flex-row md:items-center gap-2 md:gap-3 px-2 md:px-4 py-2 md:py-3 bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700"
    >
      <div class="flex-1">
        <div class="relative">
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Filter steps and workflows..."
            class="w-full pl-8 pr-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-1 focus:ring-primary focus:border-primary bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
          />
          <div class="absolute inset-y-0 left-0 pl-2.5 flex items-center pointer-events-none">
            <svg class="h-4 w-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
              />
            </svg>
          </div>
        </div>
      </div>
      <div class="flex items-center gap-2 flex-wrap">
        <select
          v-model="selectedStateFilter"
          class="flex-1 md:flex-none min-w-0 px-2 md:px-2.5 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md focus:ring-1 focus:ring-blue-500 focus:border-transparent bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300"
        >
          <option value="all">{{ t('repo.pipeline.filter.all_states') }}</option>
          <option value="success">{{ t('repo.pipeline.filter.success') }}</option>
          <option value="failure">{{ t('repo.pipeline.filter.failed') }}</option>
          <option value="running">{{ t('repo.pipeline.filter.running') }}</option>
          <option value="pending">{{ t('repo.pipeline.filter.pending') }}</option>
          <option value="skipped">{{ t('repo.pipeline.filter.skipped') }}</option>
          <option value="killed">{{ t('repo.pipeline.filter.killed') }}</option>
        </select>
        <select
          v-model="selectedWorkflowFilter"
          class="flex-1 md:flex-none min-w-0 px-2 md:px-2.5 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md focus:ring-1 focus:ring-blue-500 focus:border-transparent bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300"
        >
          <option value="all">{{ t('repo.pipeline.filter.all_workflows') }}</option>
          <option v-for="workflow in workflows" :key="workflow.pid" :value="workflow.pid">
            {{ workflow.name || `Workflow ${workflow.pid}` }}
          </option>
        </select>
        <button
          class="px-2 md:px-2.5 py-1.5 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
          @click="clearFilters"
        >
          {{ t('clear') }}
        </button>
        <button
          class="px-2 md:px-2.5 py-1.5 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors flex items-center gap-1"
          title="Download DAG as PNG"
          @click="downloadGraph"
        >
          <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
            />
          </svg>
          <span class="hidden md:inline">{{ t('download') }}</span>
        </button>
      </div>
    </div>

    <!-- Pipeline Progress -->
    <div class="px-2 md:px-4 py-2 bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700">
      <div class="flex items-center justify-between text-sm">
        <span class="text-gray-600 dark:text-gray-400">{{ t('repo.pipeline.progress') }}</span>
        <span class="text-gray-900 dark:text-gray-100"
          >{{ `${pipelineProgress.completed}/${pipelineProgress.total}` }} {{ t('repo.pipeline.steps') }}</span
        >
      </div>
      <div class="mt-1 w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
        <div
          class="bg-primary h-2 rounded-full transition-all duration-300"
          :style="{ width: `${pipelineProgress.percentage}%` }"
        ></div>
      </div>
    </div>

    <!-- DAG Visualization -->
    <div class="flex-1 min-h-0 relative">
      <div
        v-if="!filteredNodes.length"
        class="flex items-center justify-center h-full bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded"
      >
        <div class="text-center">
          <p class="text-gray-600 dark:text-gray-300">
            {{
              searchQuery || selectedStateFilter !== 'all' || selectedWorkflowFilter !== 'all'
                ? t('repo.pipeline.no_matching_steps')
                : t('repo.pipeline.no_pipeline_steps')
            }}
          </p>
        </div>
      </div>

      <!-- VueFlow container with stable dimensions -->
      <div
        v-else
        class="w-full h-full relative overflow-hidden"
        :class="{ 'transition-opacity duration-300': isLayouting }"
      >
        <VueFlow
          id="pipeline-dag"
          ref="vueFlowRef"
          :nodes="filteredNodes"
          :edges="filteredEdges"
          class="w-full h-full"
          :default-viewport="{ x: 0, y: 0, zoom: 1 }"
          :fit-view-on-init="false"
          :fit-view-options="{ padding: 0.1, includeHiddenNodes: false, duration: 200 }"
          tabindex="0"
          @nodes-initialized="onNodesInitialized"
          @node-click="handleNodeClick"
          @keydown="handleKeyDown"
        >
          <template #node-pipeline-step="nodeProps">
            <div
              class="bg-white dark:bg-gray-800 border-2 rounded-lg p-4 shadow-md dark:shadow-lg dark:shadow-black/25 relative min-w-[120px] max-w-[320px] w-max transition-all duration-200 cursor-pointer"
              :class="{
                'border-green-500 bg-green-50 dark:border-green-400 dark:bg-green-900/20':
                  nodeProps.data?.step?.state === 'success',
                'border-red-600 bg-red-50 dark:bg-red-900/20':
                  nodeProps.data?.step?.state === 'failure' || nodeProps.data?.step?.state === 'error',
                'border-yellow-600/40 bg-yellow-50 dark:border-yellow-400 dark:bg-yellow-900/20':
                  nodeProps.data?.step?.state === 'running',
                'border-gray-300 dark:border-gray-600':
                  nodeProps.data?.step?.state === 'pending' || nodeProps.data?.step?.state === 'skipped',
                'border-blue-500 bg-blue-50 dark:border-blue-400 dark:bg-blue-900/20':
                  nodeProps.data?.step?.state === 'killed',
                'ring-2 ring-primary ring-opacity-50 scale-105': selectedNodeId === nodeProps.id,
              }"
            >
              <Handle
                :id="`${nodeProps.id}-target`"
                type="target"
                :position="Position.Left"
                class="step-handle !bg-gray-500 dark:!bg-gray-400"
              />
              <div class="flex items-start justify-between mb-2 gap-2">
                <div class="font-semibold text-gray-900 dark:text-gray-100 leading-tight wrap-break-word flex-1">
                  {{ nodeProps.data?.step?.name || 'Unknown' }}
                </div>
                <div
                  class="w-3 h-3 rounded-full"
                  :class="{
                    'bg-green-500 dark:bg-green-400': nodeProps.data?.step?.state === 'success',
                    'bg-red-600': nodeProps.data?.step?.state === 'failure' || nodeProps.data?.step?.state === 'error',
                    'bg-yellow-500 dark:bg-yellow-400': nodeProps.data?.step?.state === 'running',
                    'bg-gray-400 dark:bg-gray-500':
                      nodeProps.data?.step?.state === 'pending' || nodeProps.data?.step?.state === 'skipped',
                    'bg-blue-500 dark:bg-blue-400': nodeProps.data?.step?.state === 'killed',
                  }"
                ></div>
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400 mt-1 wrap-break-word">
                {{ formatExecutionTime(nodeProps.data?.step) }}
              </div>
              <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
              <div
                v-if="nodeProps.data?.workflow?.environ && Object.keys(nodeProps.data.workflow.environ).length > 0"
                class="flex flex-wrap gap-1 mt-1.5"
              >
                <span
                  v-for="(value, key) in nodeProps.data.workflow.environ"
                  :key="key"
                  class="text-[10px] bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 px-1.5 py-0.5 rounded font-mono leading-tight"
                >
                  {{ key }}={{ value }}
                </span>
              </div>
              <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
              <Handle
                :id="`${nodeProps.id}-source`"
                type="source"
                :position="Position.Right"
                class="step-handle !bg-gray-500 dark:!bg-gray-400"
              />
            </div>
          </template>

          <template #node-workflow-group="nodeProps">
            <div
              class="border-2 border-dashed rounded-lg p-4 transition-all duration-200 bg-transparent dark:bg-gray-800/20 shadow-sm dark:shadow-black/20 relative"
              :class="{
                'border-green-500/40 dark:border-green-400/60': nodeProps.data?.workflow?.state === 'success',
                'border-red-600/40 dark:border-red-400/60':
                  nodeProps.data?.workflow?.state === 'failure' || nodeProps.data?.workflow?.state === 'error',
                'border-yellow-600/40 dark:border-yellow-400/60': nodeProps.data?.workflow?.state === 'running',
                'border-blue-800 dark:border-blue-400/60': nodeProps.data?.workflow?.state === 'killed',
                'border-gray-800 dark:border-gray-600/60':
                  nodeProps.data?.workflow?.state === 'pending' ||
                  nodeProps.data?.workflow?.state === 'skipped' ||
                  !nodeProps.data?.workflow?.state,
              }"
              :style="{
                width: `${nodeProps.data?.width}px`,
                height: `${nodeProps.data?.height}px`,
              }"
            >
              <!-- Left handle for incoming workflow dependencies -->
              <Handle
                :id="`${nodeProps.id}-target`"
                type="target"
                :position="Position.Left"
                class="workflow-handle !bg-gray-500 dark:!bg-gray-400"
              />

              <!-- Right handle for outgoing workflow dependencies -->
              <Handle
                :id="`${nodeProps.id}-source`"
                type="source"
                :position="Position.Right"
                class="workflow-handle !bg-gray-500 dark:!bg-gray-400"
              />

              <!-- Drag handle icon -->
              <div
                class="absolute top-3 right-3 opacity-0 group-hover:opacity-100 transition-all duration-200 z-30 bg-black/10 dark:bg-white/10 p-1 rounded hover:bg-black/20 dark:hover:bg-white/20 cursor-move"
              >
                <svg
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  class="text-gray-600 hover:text-gray-800 dark:text-gray-300 dark:hover:text-gray-100"
                >
                  <path
                    d="M9 3H11V5H9V3ZM13 3H15V5H13V3ZM9 7H11V9H9V7ZM13 7H15V9H13V7ZM9 11H11V13H9V11ZM13 11H15V13H13V11ZM9 15H11V17H9V15ZM13 15H15V17H13V15ZM9 19H11V21H9V19ZM13 19H15V21H13V19Z"
                    fill="currentColor"
                  />
                </svg>
              </div>

              <div class="absolute top-4 left-7 flex items-center gap-2 z-20">
                <!-- Main workflow badge -->
                <div
                  class="text-sm font-bold px-3 py-1 rounded-lg whitespace-nowrap text-center backdrop-blur-sm"
                  :class="{
                    'bg-green-600 border-green-700 text-white shadow-green-500/25':
                      nodeProps.data?.workflow?.state === 'success',
                    'bg-red-600 border-red-700 text-white shadow-red-500/25':
                      nodeProps.data?.workflow?.state === 'failure' || nodeProps.data?.workflow?.state === 'error',
                    'bg-amber-500 border-amber-600 text-white shadow-amber-500/25':
                      nodeProps.data?.workflow?.state === 'running',
                    'bg-blue-700 border-blue-800 text-white shadow-blue-500/25':
                      nodeProps.data?.workflow?.state === 'killed',
                    'bg-slate-600 border-slate-700 text-white shadow-slate-500/25':
                      nodeProps.data?.workflow?.state === 'pending' ||
                      nodeProps.data?.workflow?.state === 'skipped' ||
                      !nodeProps.data?.workflow?.state,
                  }"
                >
                  <span class="flex items-center gap-2">
                    <div
                      class="w-2.5 h-2.5 rounded-full ring-1 ring-white/30"
                      :class="{
                        'bg-green-200': nodeProps.data?.workflow?.state === 'success',
                        'bg-red-200':
                          nodeProps.data?.workflow?.state === 'failure' || nodeProps.data?.workflow?.state === 'error',
                        'bg-amber-200': nodeProps.data?.workflow?.state === 'running',
                        'bg-blue-200': nodeProps.data?.workflow?.state === 'killed',
                        'bg-slate-200':
                          nodeProps.data?.workflow?.state === 'pending' ||
                          nodeProps.data?.workflow?.state === 'skipped' ||
                          !nodeProps.data?.workflow?.state,
                      }"
                    ></div>
                    <span class="font-bold">{{ nodeProps.data?.workflow?.name || 'Unknown Workflow' }}</span>
                    <span
                      v-if="nodeProps.data?.isMatrixGroup"
                      class="text-xs font-normal opacity-90 bg-black/10 px-1.5 py-0.5 rounded"
                    >
                      {{ `${nodeProps.data?.matrixWorkflows?.length || 0}×` }}
                    </span>
                  </span>
                </div>

                <!-- Workflow dependencies badge -->
                <div
                  v-if="nodeProps.data?.workflow?.depends_on?.length"
                  class="text-xs bg-gray-100 dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 px-2 py-1 rounded-md font-medium backdrop-blur-sm"
                >
                  {{ `${t('repo.pipeline.workflow_dependencies')}: ${nodeProps.data.workflow.depends_on.length}` }}
                </div>
              </div>
            </div>
          </template>

          <Background />
          <Controls />

          <!-- Custom platform markers -->
          <PlatformMarker id="platform" type="platform" />
          <PlatformMarker id="platform-large" type="platform-large" />
        </VueFlow>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount, nextTick, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { VueFlow, Handle, Position, useVueFlow } from '@vue-flow/core';
import { Background } from '@vue-flow/background';
import { Controls } from '@vue-flow/controls';
import PlatformMarker from './PlatformMarker.vue';
import '@vue-flow/core/dist/style.css';
import '@vue-flow/core/dist/theme-default.css';
import '@vue-flow/controls/dist/style.css';
import type { Node, Edge } from '@vue-flow/core';
import type { Pipeline, PipelineStep, PipelineWorkflow, PipelineStatus } from '~/lib/api/types';
import { identifyMatrixGroupsMap } from '~/lib/pipeline-utils';
import { useLayout } from '~/compositions/useLayout';

interface Props {
  pipeline: Pipeline;
  selectedStepId?: number | null;
}

interface Emits {
  (event: 'update:selected-step-id', stepId: number): void;
  (event: 'dagLoading', loading: boolean): void;
  (event: 'dagReady'): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();
const { t } = useI18n();

const nodes = ref<Node[]>([]);
const edges = ref<Edge[]>([]);
const { layout } = useLayout();
const debounceTimeout = ref<ReturnType<typeof setTimeout> | null>(null);
const isLayouting = ref(false);
const isInitialized = ref(false);

// Use VueFlow composable with the specific flow ID
const { fitView } = useVueFlow('pipeline-dag');

// Create a separate ref for the VueFlow component
const vueFlowRef = ref();

// Note: Screenshot functionality removed due to rendering artifacts

// Search and filter state
const searchQuery = ref('');
const selectedStateFilter = ref('all');
const selectedWorkflowFilter = ref('all');
const selectedNodeId = ref<string | null>(null);

// Computed properties for filtering
const workflows = computed(() => props.pipeline.workflows || []);

// Memoized filtered nodes and edges
const filteredNodes = computed(() => {
  if (!searchQuery.value && selectedStateFilter.value === 'all' && selectedWorkflowFilter.value === 'all') {
    return nodes.value;
  }

  // First pass: identify workflows that match search criteria
  const matchingWorkflowNames = new Set<string>();
  const matchingWorkflowPids = new Set<number>();

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase();

    // Check workflow groups for name matches
    nodes.value.forEach((node: Node) => {
      if (node.type === 'workflow-group') {
        const workflowName = node.data?.workflow?.name?.toLowerCase() || '';
        if (workflowName.includes(query)) {
          matchingWorkflowNames.add(node.data?.workflow?.name || '');

          // For matrix groups, add all workflow names and PIDs
          if (node.data?.isMatrixGroup && node.data?.matrixWorkflows) {
            node.data.matrixWorkflows.forEach((workflow: PipelineWorkflow) => {
              matchingWorkflowNames.add(workflow.name);
              matchingWorkflowPids.add(workflow.pid);
            });
          } else if (node.data?.workflow?.pid) {
            matchingWorkflowPids.add(node.data.workflow.pid);
          }
        }
      }
    });

    // Also check individual step workflows for name matches
    nodes.value.forEach((node: Node) => {
      if (node.type === 'pipeline-step') {
        const workflowName = node.data?.workflow?.name?.toLowerCase() || '';
        if (workflowName.includes(query)) {
          matchingWorkflowNames.add(node.data?.workflow?.name || '');
          matchingWorkflowPids.add(node.data?.workflow?.pid || 0);
        }
      }
    });
  }

  return nodes.value.filter((node: Node) => {
    // Filter workflow groups
    if (node.type === 'workflow-group') {
      // Workflow filter
      if (selectedWorkflowFilter.value !== 'all') {
        const filterPid = Number.parseInt(selectedWorkflowFilter.value, 10);
        if (node.data?.isMatrixGroup && node.data?.matrixWorkflows) {
          // For matrix groups, check if any matrix workflow matches
          const hasMatchingWorkflow = node.data.matrixWorkflows.some(
            (workflow: PipelineWorkflow) => workflow.pid === filterPid,
          );
          if (!hasMatchingWorkflow) return false;
        } else {
          if (node.data?.workflow?.pid !== filterPid) return false;
        }
      }

      // Search query filter for workflow groups
      if (searchQuery.value) {
        const query = searchQuery.value.toLowerCase();
        const workflowName = node.data?.workflow?.name?.toLowerCase() || '';

        // Show workflow group if:
        // 1. The workflow group name matches the search
        // 2. Any of its child workflows match the search (for matrix groups)
        // 3. Any steps in associated workflows match the search
        let shouldShow = workflowName.includes(query);

        if (!shouldShow && node.data?.isMatrixGroup && node.data?.matrixWorkflows) {
          // Check if any matrix workflow matches
          shouldShow = node.data.matrixWorkflows.some((workflow: PipelineWorkflow) =>
            workflow.name.toLowerCase().includes(query),
          );
        }

        if (!shouldShow) {
          // Check if any associated workflow has matching steps or workflow names
          if (node.data?.workflow?.name) {
            shouldShow = matchingWorkflowNames.has(node.data.workflow.name);
          }
          if (!shouldShow && node.data?.isMatrixGroup && node.data?.matrixWorkflows) {
            shouldShow = node.data.matrixWorkflows.some((workflow: PipelineWorkflow) =>
              matchingWorkflowNames.has(workflow.name),
            );
          }
        }

        // Check if any workflow's environ matches the search
        if (!shouldShow && node.data?.isMatrixGroup && node.data?.matrixWorkflows) {
          shouldShow = node.data.matrixWorkflows.some(
            (workflow: PipelineWorkflow) =>
              workflow.environ &&
              Object.entries(workflow.environ).some(
                ([k, v]) =>
                  k.toLowerCase().includes(query) ||
                  v.toLowerCase().includes(query) ||
                  `${k}=${v}`.toLowerCase().includes(query),
              ),
          );
        } else if (!shouldShow && node.data?.workflow?.environ) {
          shouldShow = Object.entries(node.data.workflow.environ as Record<string, string>).some(
            ([k, v]) =>
              k.toLowerCase().includes(query) ||
              v.toLowerCase().includes(query) ||
              `${k}=${v}`.toLowerCase().includes(query),
          );
        }

        if (!shouldShow) return false;
      }

      return true;
    }

    // Filter step nodes
    if (node.type === 'pipeline-step') {
      const step = node.data?.step;
      const workflow = node.data?.workflow;

      // Search query filter
      if (searchQuery.value) {
        const query = searchQuery.value.toLowerCase();
        const stepName = step?.name?.toLowerCase() || '';
        const workflowName = workflow?.name?.toLowerCase() || '';

        // Show step if:
        // 1. Step name matches search
        // 2. Workflow name matches search
        // 3. Workflow is in the set of matching workflows (from workflow group search)
        const stepMatches = stepName.includes(query);
        const workflowMatches = workflowName.includes(query);
        const workflowInMatchingSet = workflow?.name ? matchingWorkflowNames.has(workflow.name) : false;
        const environMatches = workflow?.environ
          ? Object.entries(workflow.environ as Record<string, string>).some(
              ([k, v]) =>
                k.toLowerCase().includes(query) ||
                v.toLowerCase().includes(query) ||
                `${k}=${v}`.toLowerCase().includes(query),
            )
          : false;

        if (!stepMatches && !workflowMatches && !workflowInMatchingSet && !environMatches) {
          return false;
        }
      }

      // State filter
      if (selectedStateFilter.value !== 'all' && step?.state !== selectedStateFilter.value) {
        return false;
      }

      // Workflow filter
      if (
        selectedWorkflowFilter.value !== 'all' &&
        workflow?.pid !== Number.parseInt(selectedWorkflowFilter.value, 10)
      ) {
        return false;
      }

      return true;
    }

    return true;
  });
});

const filteredEdges = computed(() => {
  const visibleNodeIds = new Set(filteredNodes.value.map((node: Node) => node.id));
  return edges.value.filter((edge) => visibleNodeIds.has(edge.source) && visibleNodeIds.has(edge.target));
});

// Pipeline progress calculation
const pipelineProgress = computed(() => {
  const stepNodes = nodes.value.filter((node: Node) => node.type === 'pipeline-step');
  const total = stepNodes.length;
  const completed = stepNodes.filter((node: Node) => {
    const state = node.data?.step?.state;
    return state === 'success' || state === 'failure' || state === 'error' || state === 'killed' || state === 'skipped';
  }).length;

  return {
    total,
    completed,
    percentage: total > 0 ? (completed / total) * 100 : 0,
  };
});

// Download functionality
const downloadGraph = async () => {
  try {
    // Check if we have nodes to export
    if (filteredNodes.value.length === 0) {
      console.warn('No nodes to export');
      return;
    }

    // Check if VueFlow ref is available
    if (!vueFlowRef.value) {
      console.warn('VueFlow element not found');
      return;
    }

    // Fit the view to make sure everything is visible for manual screenshot
    if (fitView) {
      fitView({ padding: 0.1, includeHiddenNodes: false, duration: 300 });
      await new Promise((resolve) => setTimeout(resolve, 400));
    }

    // Show user-friendly screenshot modal
    const modal = document.createElement('div');
    modal.style.cssText = `
      position: fixed; top: 0; left: 0; right: 0; bottom: 0;
      background: rgba(0,0,0,0.8); color: white; z-index: 10000;
      display: flex; align-items: center; justify-content: center;
      font-family: Roboto, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif;
    `;

    const content = document.createElement('div');
    content.style.cssText = `
      background: #1f2937; padding: 2rem; border-radius: 12px;
      max-width: 500px; border: 1px solid #374151;
      box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1);
    `;

    content.innerHTML = `
      <h3 style="margin-top: 0; color: #60a5fa; font-size: 1.25rem;">📸 Screenshot Your Pipeline DAG</h3>
      <p style="margin: 1rem 0; line-height: 1.6; color: #e5e7eb; font-style: italic; font-size: 0.7rem">Built-in screenshotting is disabled due to artifact rendering issues with DAG edges.</p>
      <p style="margin: 0 0 1rem 0; line-height: 1.6; color: #e5e7eb;">Choose your preferred manual screenshot method:</p>

      <div style="background: #374151; padding: 1rem; border-radius: 6px; margin: 1rem 0;">
        <div style="margin-bottom: 1rem; padding: 0.75rem; background: #059669; border-radius: 4px;">
          <strong style="color: white; font-size: 1rem;">🦊 Recommended: Firefox Built-in Screenshot</strong><br>
          <span style="color: #d1fae5; font-size: 0.9rem; margin-top: 0.25rem; display: block;">
            Right-click on any node or workflow group → "Take Screenshot" → "Save visible" for perfect results!
          </span>
          <span style="color: #fef3c7; font-size: 0.85rem; margin-top: 0.25rem; display: block;">
            ⚠️ Note: Right-click on a node/workflow, not the empty canvas area
          </span>
        </div>

        <div style="margin-bottom: 0.75rem;"><strong style="color: #fbbf24;">System Screenshot Tools:</strong></div>
        <div style="margin-left: 1rem; font-size: 0.9rem; line-height: 1.4;">
          <div style="margin-bottom: 0.5rem;"><strong>Mac:</strong> <kbd style="background: #4b5563; padding: 2px 6px; border-radius: 3px;">⌘ + Shift + 4</kbd> then select DAG area</div>
          <div style="margin-bottom: 0.5rem;"><strong>Windows:</strong> <kbd style="background: #4b5563; padding: 2px 6px; border-radius: 3px;">⊞ + Shift + S</kbd> then select area</div>
          <div style="margin-bottom: 0.5rem;"><strong>Linux:</strong> <kbd style="background: #4b5563; padding: 2px 6px; border-radius: 3px;">PrtScn</kbd> or use your screenshot tool</div>
        </div>

        <div style="margin-top: 1rem; font-size: 0.85rem; color: #9ca3af;">
          💡 Tip: Make sure all DAG nodes are visible in your browser window before taking the screenshot
        </div>
      </div>

      <button id="closeModal" style="
        background: #3b82f6; color: white; border: none; padding: 0.75rem 1.5rem;
        border-radius: 6px; cursor: pointer; font-weight: 500; margin-top: 1rem;
        transition: background-color 0.2s;
      ">Got it!</button>
    `;

    const closeBtn = content.querySelector('#closeModal') as HTMLButtonElement;
    if (closeBtn) {
      closeBtn.addEventListener('click', () => {
        document.body.removeChild(modal);
      });
      closeBtn.addEventListener('mouseenter', (e) => {
        if (e.target instanceof HTMLElement) {
          e.target.style.backgroundColor = '#2563eb';
        }
      });
      closeBtn.addEventListener('mouseleave', (e) => {
        if (e.target instanceof HTMLElement) {
          e.target.style.backgroundColor = '#3b82f6';
        }
      });
    }

    modal.appendChild(content);
    document.body.appendChild(modal);

    // Auto-close after 15 seconds
    setTimeout(() => {
      if (document.body.contains(modal)) {
        document.body.removeChild(modal);
      }
    }, 15000);
  } catch (error) {
    console.error('Failed to download graph:', error);
    // Fallback message
    console.error("Download failed. Please try using your browser's screenshot functionality.");
  }
};

// Format execution time for display
function formatExecutionTime(step: PipelineStep | undefined): string {
  if (!step) return '-';

  if (step.started && step.finished) {
    const duration = (step.finished - step.started) * 1000;
    const seconds = Math.floor(duration / 1000);
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;

    if (minutes > 0) {
      return `${minutes}m ${remainingSeconds}s`;
    }
    return `${seconds}s`;
  } else if (step.started && step.state === 'running') {
    const duration = Math.floor(Date.now() - step.started * 1000);
    const seconds = Math.floor(duration / 1000);
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;

    if (minutes > 0) {
      return `${minutes}m ${remainingSeconds}s (running)`;
    }
    return `${remainingSeconds}s (running)`;
  }

  return '-';
}

// Handle nodes initialization and layout
function onNodesInitialized() {
  if (isLayouting.value) return; // Prevent multiple layout attempts

  isLayouting.value = true;
  emit('dagLoading', true);

  // Wait for nodes to be fully rendered before applying layout
  nextTick(() => {
    setTimeout(() => {
      applyLayout('LR');
    }, 150);
  });
}

// Apply layout using the useLayout composable
function applyLayout(direction: string) {
  if (nodes.value.length === 0) {
    return;
  }

  try {
    // Ensure nodes have been rendered and dimensions are available
    if (vueFlowRef.value) {
      const vueFlowInstance = vueFlowRef.value;

      // Check if nodes have dimensions (only check step nodes, not workflow groups)
      const stepNodes = nodes.value.filter((node) => node.type === 'pipeline-step');
      const nodesWithDimensions = stepNodes.every((node) => {
        const flowNode = vueFlowInstance.findNode(node.id);
        const hasDimensions = flowNode && flowNode.dimensions && flowNode.dimensions.width > 0;
        return hasDimensions;
      });

      if (!nodesWithDimensions) {
        // Retry after a short delay if nodes don't have dimensions yet
        setTimeout(() => applyLayout(direction), 25);
        return;
      }
    }

    // Use the layout function from the composable (only for step nodes)
    const stepNodes = nodes.value.filter((node) => node.type === 'pipeline-step');

    const updatedStepNodes = layout(stepNodes, edges.value, direction);

    // Update step nodes and immediately create workflow groups
    nodes.value = updatedStepNodes;

    // Create workflow groups immediately
    const workflowSteps = new Map<number, Node[]>();
    props.pipeline.workflows?.forEach((workflow) => {
      const stepNodesForWorkflow = updatedStepNodes.filter((node) => node.data?.workflow?.pid === workflow.pid);
      if (stepNodesForWorkflow.length > 0) {
        workflowSteps.set(workflow.pid, stepNodesForWorkflow);
      }
    });

    // Create workflow groups and add them to nodes
    const matrixGroups = identifyMatrixGroupsMap(props.pipeline.workflows || []);
    const workflowGroups = createWorkflowGroupsSync(workflowSteps, matrixGroups);

    // Apply workflow dependency layout for proper positioning
    const layoutResult = applyWorkflowDependencyLayout(workflowGroups, matrixGroups);
    const positionedGroups = layoutResult.workflowGroups;
    const xOffsets = layoutResult.xOffsets;
    const yOffsets = layoutResult.yOffsets;

    // Apply X and Y offsets to step nodes to keep them aligned with their workflow groups
    const repositionedStepNodes = updatedStepNodes.map((stepNode) => {
      const workflowPid = stepNode.data?.workflow?.pid;
      const workflowName = stepNode.data?.workflow?.name;

      if (workflowPid && workflowName) {
        // Find the correct group key for this workflow
        let groupKey = workflowName;
        for (const [gKey, workflows] of matrixGroups.entries()) {
          if (workflows.some((w) => w.name === workflowName)) {
            groupKey = gKey;
            break;
          }
        }

        const groupId = `workflow-group-${groupKey}`;
        const xOffset = xOffsets.get(groupId) || 0;
        const yOffset = yOffsets.get(groupId) || 0;

        if (xOffset !== 0 || yOffset !== 0) {
          return {
            ...stepNode,
            position: {
              x: stepNode.position.x + xOffset,
              y: stepNode.position.y + yOffset,
            },
          };
        }
      }

      return stepNode;
    });

    nodes.value = [...positionedGroups, ...repositionedStepNodes];

    // Trigger fit-to-view to ensure all nodes are visible after full render and layout
    nextTick(() => {
      // Wait for all layout calculations to complete
      setTimeout(() => {
        requestAnimationFrame(() => {
          requestAnimationFrame(() => {
            requestAnimationFrame(() => {
              if (vueFlowRef.value) {
                vueFlowRef.value.fitView({
                  padding: 0.1,
                  includeHiddenNodes: false,
                  duration: 200, // Smooth animation for fit-to-view
                });

                // Mark layout as complete and emit ready event
                setTimeout(() => {
                  isLayouting.value = false;
                  isInitialized.value = true;
                  emit('dagReady');
                }, 300); // Wait for fit-to-view animation to complete
              }
            });
          });
        });
      }, 100); // Reduced from 300ms for faster initial load
    });
  } catch (error) {
    console.error('Layout error:', error);
    // Fallback to simple grid layout
    applyFallbackLayout(direction);
  }
}

// Fallback layout when main layout fails
function applyFallbackLayout(direction: string) {
  const stepNodes = nodes.value.filter((node) => node.type === 'pipeline-step');
  const isHorizontal = direction === 'LR';

  const updatedStepNodes = stepNodes.map((node, index) => ({
    ...node,
    position: isHorizontal ? { x: index * 250, y: 0 } : { x: 0, y: index * 150 },
  }));

  // Create workflow groups
  const workflowSteps = new Map<number, Node[]>();
  props.pipeline.workflows?.forEach((workflow) => {
    const stepNodesForWorkflow = updatedStepNodes.filter((node) => node.data?.workflow?.pid === workflow.pid);
    if (stepNodesForWorkflow.length > 0) {
      workflowSteps.set(workflow.pid, stepNodesForWorkflow);
    }
  });

  const matrixGroups = identifyMatrixGroupsMap(props.pipeline.workflows || []);
  const workflowGroups = createWorkflowGroupsSync(workflowSteps, matrixGroups);

  // Apply workflow dependency layout for proper positioning
  const layoutResult = applyWorkflowDependencyLayout(workflowGroups, matrixGroups);
  const positionedGroups = layoutResult.workflowGroups;
  const xOffsets = layoutResult.xOffsets;
  const yOffsets = layoutResult.yOffsets;

  // Apply X and Y offsets to step nodes to keep them aligned with their workflow groups
  const repositionedStepNodes = updatedStepNodes.map((stepNode) => {
    const workflowPid = stepNode.data?.workflow?.pid;
    const workflowName = stepNode.data?.workflow?.name;

    if (workflowPid && workflowName) {
      // Find the correct group key for this workflow
      let groupKey = workflowName;
      for (const [gKey, workflows] of matrixGroups.entries()) {
        if (workflows.some((w) => w.name === workflowName)) {
          groupKey = gKey;
          break;
        }
      }

      const groupId = `workflow-group-${groupKey}`;
      const xOffset = xOffsets.get(groupId) || 0;
      const yOffset = yOffsets.get(groupId) || 0;

      if (xOffset !== 0 || yOffset !== 0) {
        return {
          ...stepNode,
          position: {
            x: stepNode.position.x + xOffset,
            y: stepNode.position.y + yOffset,
          },
        };
      }
    }

    return stepNode;
  });

  nodes.value = [...positionedGroups, ...repositionedStepNodes];

  nextTick(() => {
    // Wait for all layout calculations to complete
    setTimeout(() => {
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          requestAnimationFrame(() => {
            if (vueFlowRef.value) {
              vueFlowRef.value.fitView({
                padding: 0.1,
                includeHiddenNodes: false,
                duration: 200,
              });

              // Mark layout as complete and emit ready event
              setTimeout(() => {
                isLayouting.value = false;
                isInitialized.value = true;
                emit('dagReady');
              }, 300);
            }
          });
        });
      });
    }, 100);
  });
}

// Convert pipeline data to DAG nodes and edges
function buildDAG() {
  if (!props.pipeline.workflows) {
    return;
  }

  // Reset state and emit loading
  isInitialized.value = false;
  isLayouting.value = false;
  emit('dagLoading', true);

  try {
    const newNodes: Node[] = [];
    const newEdges: Edge[] = [];
    const globalStepMap = new Map<string, Node>();
    const workflowSteps = new Map<number, Node[]>();
    const workflowMap = new Map<string, PipelineWorkflow>();

    // Build workflow lookup map for cross-workflow dependencies
    props.pipeline.workflows.forEach((workflow) => {
      workflowMap.set(workflow.name, workflow);
    });

    // Identify matrix workflow groups using improved detection
    const matrixGroups = identifyMatrixGroupsMap(props.pipeline.workflows || []);

    // Create nodes for each step
    props.pipeline.workflows.forEach((workflow) => {
      const stepNodes: Node[] = [];

      workflow.children?.forEach((step) => {
        const nodeId = `step-${step.pid}`;
        const node: Node = {
          id: nodeId,
          type: 'pipeline-step',
          position: { x: 0, y: 0 },
          data: {
            step,
            workflow,
            pipeline: props.pipeline,
            selectedStepId: props.selectedStepId,
          },
          sourcePosition: Position.Right,
          targetPosition: Position.Left,
          zIndex: 2000, // Ensure step nodes appear above everything including edges
        };
        newNodes.push(node);
        stepNodes.push(node);

        // Register step in global map with workflow prefix
        globalStepMap.set(`${workflow.pid}-${step.name}`, node);
        // Also register with workflow name for cross-workflow lookups
        globalStepMap.set(`${workflow.name}-${step.name}`, node);
      });

      if (stepNodes.length > 0) {
        workflowSteps.set(workflow.pid, stepNodes);
      }
    });

    // Create virtual workflow group nodes for matrix workflows (not added to main nodes)
    const virtualGroupNodes = createVirtualMatrixGroupNodes(matrixGroups, workflowSteps);

    // Create edges for step-level dependencies
    props.pipeline.workflows.forEach((workflow) => {
      workflow.children?.forEach((step) => {
        const targetNode = globalStepMap.get(`${workflow.pid}-${step.name}`);
        if (!targetNode) {
          return;
        }

        // Use backend-provided depends_on field
        if (step.depends_on && step.depends_on.length > 0) {
          step.depends_on.forEach((dependency) => {
            let sourceNode: Node | undefined;

            // First, try to find dependency within the same workflow
            sourceNode = globalStepMap.get(`${workflow.pid}-${dependency}`);

            // If not found, try to find it in other workflows (cross-workflow dependency)
            if (!sourceNode) {
              // For matrix workflows, try to find clone step in other matrix variants
              // Look for dependency in workflow with similar base name
              const baseWorkflowName = workflow.name.replace(/-[^-]*$/, ''); // Remove matrix suffix
              for (const [key, node] of globalStepMap.entries()) {
                if (key.endsWith(`-${dependency}`) && key.includes(baseWorkflowName)) {
                  sourceNode = node;
                  break;
                }
              }
            }

            // If still not found, try global search across all workflows
            if (!sourceNode) {
              for (const [key, node] of globalStepMap.entries()) {
                if (key.endsWith(`-${dependency}`)) {
                  sourceNode = node;
                  break;
                }
              }
            }

            if (sourceNode) {
              const edge: Edge = {
                id: `edge-${sourceNode.id}-${targetNode.id}`,
                source: sourceNode.id,
                target: targetNode.id,
                sourceHandle: `${sourceNode.id}-source`,
                targetHandle: `${targetNode.id}-target`,
                type: 'smoothstep',
                animated: step.state === 'running',
                markerEnd: {
                  type: 'platform',
                  color: '#6b7280',
                } as any,
              };
              newEdges.push(edge);
            }
          });
        }
      });
    });

    // Create edges for workflow-level dependencies with matrix group awareness
    const workflowDependencyEdges = createWorkflowDependencyEdges(
      matrixGroups,
      workflowSteps,
      workflowMap,
      virtualGroupNodes,
    );
    newEdges.push(...workflowDependencyEdges);

    // Create workflow groups immediately
    const workflowGroups = createWorkflowGroupsSync(workflowSteps, matrixGroups);

    nodes.value = [...workflowGroups, ...newNodes];
    edges.value = newEdges;

    // Note: Fit-to-view will be handled by onNodesInitialized -> applyLayout
  } catch (error) {
    console.error('Error building DAG:', error);
    // Clear nodes and edges on error
    nodes.value = [];
    edges.value = [];
  }
}

// Create virtual workflow group nodes that participate in the DAG layout
function createVirtualMatrixGroupNodes(
  matrixGroups: Map<string, PipelineWorkflow[]>,
  _workflowSteps: Map<number, Node[]>,
): Node[] {
  const virtualNodes: Node[] = [];

  matrixGroups.forEach((workflows, _groupKey) => {
    if (workflows.length > 1) {
      // This is a matrix group, create a virtual node for dependency tracking
      const representativeWorkflow = workflows[0];
      const virtualNode: Node = {
        id: `virtual-matrix-group-${_groupKey}`,
        type: 'virtual-matrix-group',
        position: { x: 0, y: 0 },
        data: {
          groupKey: _groupKey,
          workflows,
          representativeWorkflow,
          isVirtual: true,
        },
        style: { display: 'none' }, // Hidden from view but participates in layout
        zIndex: -1,
        selectable: false,
        draggable: false,
      };
      virtualNodes.push(virtualNode);
    }
  });

  return virtualNodes;
}

// Create workflow dependency edges with matrix group awareness
function createWorkflowDependencyEdges(
  matrixGroups: Map<string, PipelineWorkflow[]>,
  _workflowSteps: Map<number, Node[]>,
  workflowMap: Map<string, PipelineWorkflow>,
  virtualGroupNodes: Node[],
): Edge[] {
  const workflowDependencyEdges: Edge[] = [];
  const groupNameToVirtualNode = new Map<string, Node>();

  // Map group names to virtual nodes
  virtualGroupNodes.forEach((node) => {
    if (node.data?.groupKey) {
      groupNameToVirtualNode.set(node.data.groupKey, node);
    }
  });

  // Create simplified mapping from dependency names to workflow groups
  const dependencyNameToGroups = new Map<string, PipelineWorkflow[]>();

  matrixGroups.forEach((workflows, groupKey) => {
    // Map the group key (pipeline name) to the workflows
    dependencyNameToGroups.set(groupKey, workflows);

    // Also map each individual workflow name to its group for backward compatibility
    workflows.forEach((workflow) => {
      dependencyNameToGroups.set(workflow.name, workflows);
    });
  });

  matrixGroups.forEach((workflows, currentGroupKey) => {
    workflows.forEach((workflow) => {
      if (workflow.depends_on && workflow.depends_on.length > 0) {
        workflow.depends_on.forEach((dependencyWorkflowName) => {
          // First try to find the dependency workflows by the dependency name
          let dependencyWorkflows = dependencyNameToGroups.get(dependencyWorkflowName);

          if (!dependencyWorkflows) {
            // Fallback: try to find individual workflow by name
            const dependencyWorkflow = workflowMap.get(dependencyWorkflowName);
            if (dependencyWorkflow) {
              dependencyWorkflows = [dependencyWorkflow];
            }
          }

          if (!dependencyWorkflows) {
            return;
          }

          // Find the correct group keys for both dependency and current workflow
          let dependencyGroupKey = dependencyWorkflowName;

          // Check if the dependency is a matrix group key
          if (matrixGroups.has(dependencyWorkflowName)) {
            dependencyGroupKey = dependencyWorkflowName;
          } else {
            // Find which group contains the dependency workflow
            for (const [groupKey, workflows] of matrixGroups.entries()) {
              if (workflows.some((w) => w.name === dependencyWorkflowName)) {
                dependencyGroupKey = groupKey;
                break;
              }
            }
          }

          const currentWorkflowGroupKey = currentGroupKey;

          const dependencyGroupId = `workflow-group-${dependencyGroupKey}`;
          const currentWorkflowGroupId = `workflow-group-${currentWorkflowGroupKey}`;

          // Create edge between workflow groups
          const edge: Edge = {
            id: `workflow-dependency-${dependencyGroupId}-${currentWorkflowGroupId}`,
            source: dependencyGroupId,
            target: currentWorkflowGroupId,
            sourceHandle: `${dependencyGroupId}-source`,
            targetHandle: `${currentWorkflowGroupId}-target`,
            type: 'smoothstep', // Use straight edges for workflow dependencies
            animated: false,
            style: {
              strokeDasharray: '8,8',
              strokeWidth: 3,
              stroke: '#6b7280', // gray-500 for consistent appearance across themes
              strokeOpacity: 0.8,
            },
            markerEnd: {
              type: 'platform-large',
              color: '#6b7280',
            } as any,
            zIndex: 5, // Above other edges
          };
          workflowDependencyEdges.push(edge);
        });
      }
    });
  });

  return workflowDependencyEdges;
}

// Create workflow group background nodes synchronously
function createWorkflowGroupsSync(
  workflowSteps: Map<number, Node[]>,
  matrixGroups?: Map<string, PipelineWorkflow[]>,
): Node[] {
  const workflowGroups: Node[] = [];

  if (!props.pipeline.workflows) return workflowGroups;

  // Use provided matrix groups or fallback to basic grouping
  const baseWorkflowGroups = matrixGroups || new Map<string, PipelineWorkflow[]>();

  if (!matrixGroups) {
    // Fallback to basic grouping (for backward compatibility)
    props.pipeline.workflows.forEach((workflow) => {
      let baseWorkflowName = workflow.name;

      // Check for auto-generated matrix pattern: name-matrix-N
      const matrixSuffixMatch = workflow.name.match(/^(.+)-matrix-\d+$/);
      if (matrixSuffixMatch) {
        baseWorkflowName = matrixSuffixMatch[1];
      }

      if (!baseWorkflowGroups.has(baseWorkflowName)) {
        baseWorkflowGroups.set(baseWorkflowName, []);
      }
      baseWorkflowGroups.get(baseWorkflowName)!.push(workflow);
    });
  }

  // Create workflow groups for each base workflow
  baseWorkflowGroups.forEach((workflows, baseWorkflowName) => {
    // Collect all step nodes for this workflow group
    const allStepNodes: Node[] = [];
    let representativeWorkflow: PipelineWorkflow | null = null;

    workflows.forEach((workflow) => {
      const stepNodes = workflowSteps.get(workflow.pid);
      if (stepNodes && stepNodes.length > 0) {
        allStepNodes.push(...stepNodes);
        if (!representativeWorkflow) {
          representativeWorkflow = workflow;
        }
      }
    });

    if (allStepNodes.length === 0 || !representativeWorkflow) return;

    // Calculate bounding box of all steps in this workflow group
    let minX = Infinity;
    let minY = Infinity;
    let maxX = -Infinity;
    let maxY = -Infinity;

    allStepNodes.forEach((stepNode) => {
      // Use node position directly since we're creating them synchronously
      // Estimate width based on content - use average of min/max with some padding
      const stepName = stepNode.data?.step?.name || 'Unknown';
      const workflowName = stepNode.data?.workflow?.name || 'Unknown';
      const estimatedWidth = Math.min(320, Math.max(120, Math.max(stepName.length * 8, workflowName.length * 6) + 80));
      const nodeHeight = 100; // Consistent height for all nodes without dependencies section

      minX = Math.min(minX, stepNode.position.x);
      minY = Math.min(minY, stepNode.position.y);
      maxX = Math.max(maxX, stepNode.position.x + estimatedWidth);
      maxY = Math.max(maxY, stepNode.position.y + nodeHeight);
    });

    if (minX !== Infinity && minY !== Infinity && representativeWorkflow) {
      const padding = 25;
      const topPadding = 55; // Extra space for workflow label
      const bottomPadding = 10; // Reduced bottom padding
      const groupWidth = maxX - minX + padding * 2;
      const groupHeight = maxY - minY + topPadding + bottomPadding;

      // Type assertion to help TypeScript understand the type
      const representative = representativeWorkflow as PipelineWorkflow;

      // Create a combined workflow object for display
      const displayWorkflow: PipelineWorkflow = {
        id: representative.id,
        pipeline_id: representative.pipeline_id,
        pid: representative.pid,
        name: baseWorkflowName,
        pipeline_name: representative.pipeline_name,
        state: getAggregatedWorkflowState(workflows),
        environ: representative.environ,
        started: representative.started,
        finished: representative.finished,
        agent_id: representative.agent_id,
        error: representative.error,
        depends_on: representative.depends_on,
        children: representative.children,
      };

      const groupNode: Node = {
        id: `workflow-group-${baseWorkflowName}`,
        type: 'workflow-group',
        position: {
          x: minX - padding,
          y: minY - topPadding,
        },
        data: {
          workflow: displayWorkflow,
          width: groupWidth,
          height: groupHeight,
          isMatrixGroup: workflows.length > 1,
          matrixWorkflows: workflows,
        },
        zIndex: 100, // Behind step nodes but above background and edges
        selectable: false,
        draggable: false,
      };

      workflowGroups.push(groupNode);
    }
  });

  return workflowGroups;
}

// Apply simple workflow dependency layout - position workflow groups based on dependencies
function applyWorkflowDependencyLayout(
  workflowGroups: Node[],
  matrixGroups: Map<string, PipelineWorkflow[]>,
): { workflowGroups: Node[]; xOffsets: Map<string, number>; yOffsets: Map<string, number> } {
  if (workflowGroups.length === 0) {
    return { workflowGroups: [], xOffsets: new Map(), yOffsets: new Map() };
  }

  // Create a simple dependency graph to determine horizontal positioning
  const dependencyGraph = new Map<string, string[]>();
  const incomingEdges = new Map<string, number>();
  const groupsByKey = new Map<string, Node>();

  // Initialize all workflow groups
  workflowGroups.forEach((group) => {
    dependencyGraph.set(group.id, []);
    incomingEdges.set(group.id, 0);
    groupsByKey.set(group.id, group);
  });

  // Build dependency graph from workflow dependencies
  matrixGroups.forEach((workflows, groupKey) => {
    workflows.forEach((workflow) => {
      if (workflow.depends_on && workflow.depends_on.length > 0) {
        const currentGroupId = `workflow-group-${groupKey}`;

        workflow.depends_on.forEach((dependency) => {
          // Find the dependency group key
          let dependencyGroupKey = dependency;
          if (matrixGroups.has(dependency)) {
            dependencyGroupKey = dependency;
          } else {
            // Find which group contains the dependency workflow
            for (const [gKey, gWorkflows] of matrixGroups.entries()) {
              if (gWorkflows.some((w) => w.name === dependency)) {
                dependencyGroupKey = gKey;
                break;
              }
            }
          }

          const dependencyGroupId = `workflow-group-${dependencyGroupKey}`;

          // Check if both groups exist
          if (dependencyGraph.has(dependencyGroupId) && dependencyGraph.has(currentGroupId)) {
            // Add edge from dependency to current
            dependencyGraph.get(dependencyGroupId)!.push(currentGroupId);
            incomingEdges.set(currentGroupId, (incomingEdges.get(currentGroupId) || 0) + 1);
          }
        });
      }
    });
  });

  // We don't need the sorted order, just the dependency rank calculation below

  // Calculate dynamic spacing based on workflow group dimensions
  const calculateDynamicSpacing = () => {
    const groupDimensions = new Map<string, { width: number; height: number }>();

    // Calculate actual dimensions for each workflow group
    workflowGroups.forEach((group) => {
      const width = group.data?.width || 300; // Fallback width
      const height = group.data?.height || 200; // Fallback height
      groupDimensions.set(group.id, { width, height });
    });

    // Calculate maximum dimensions across all groups
    const maxWidth = Math.max(...Array.from(groupDimensions.values()).map((d) => d.width));
    const maxHeight = Math.max(...Array.from(groupDimensions.values()).map((d) => d.height));

    // Use minimal grid spacing - this is the distance between grid positions, not content + padding
    const horizontalSpacing = Math.max(60, maxWidth * 0.2); // 20% of max width or minimum 60px
    const verticalSpacing = Math.max(40, maxHeight * 0.15); // 15% of max height or minimum 40px

    return {
      x: horizontalSpacing,
      y: verticalSpacing,
      groupDimensions,
    };
  };

  const spacingInfo = calculateDynamicSpacing();
  const gridSpacing = spacingInfo;

  const xOffsets = new Map<string, number>();
  const yOffsets = new Map<string, number>();

  // Track occupied cells and grid positions
  const occupiedCells = new Set<string>();
  const gridPositions = new Map<string, { x: number; y: number }>();

  // Helper function to find next available position with dependency-aware positioning
  function findAvailablePosition(preferredX: number, preferredY: number, groupId: string): { x: number; y: number } {
    const groupDimensions = spacingInfo.groupDimensions.get(groupId) || { width: 300, height: 200 };

    // Helper function to check if a position would cause overlap
    function wouldOverlap(x: number, y: number): boolean {
      const proposedLeft = x * gridSpacing.x;
      const proposedRight = proposedLeft + groupDimensions.width;
      const proposedTop = y * gridSpacing.y;
      const proposedBottom = proposedTop + groupDimensions.height;

      // Check against all existing positioned groups
      for (const [existingGroupId, existingPos] of gridPositions.entries()) {
        if (existingGroupId === groupId) continue; // Skip self

        const existingDimensions = spacingInfo.groupDimensions.get(existingGroupId) || { width: 300, height: 200 };
        const existingLeft = existingPos.x * gridSpacing.x;
        const existingRight = existingLeft + existingDimensions.width;
        const existingTop = existingPos.y * gridSpacing.y;
        const existingBottom = existingTop + existingDimensions.height;

        // Check for overlap with minimal viable margin
        const margin = 10; // Add 10px margin between groups
        if (
          proposedLeft < existingRight + margin &&
          proposedRight > existingLeft - margin &&
          proposedTop < existingBottom + margin &&
          proposedBottom > existingTop - margin
        ) {
          return true;
        }
      }

      return false;
    }

    // Find next available X position starting from preferredX
    let currentX = preferredX;
    let currentY = preferredY;

    // Try to find an available position starting from the preferred X
    while (currentX < preferredX + 10) {
      // Don't go too far from the intended position
      currentY = preferredY;

      // Try different Y positions at this X
      while (currentY < preferredY + 10) {
        if (!wouldOverlap(currentX, currentY)) {
          return { x: currentX, y: currentY };
        }
        currentY++;
      }

      // Move to next X position (further right)
      currentX += 0.1; // Use smaller increments for more precise positioning
    }

    // Fallback: find any available position by scanning systematically
    for (let y = 0; y < 20; y++) {
      for (let x = preferredX; x < preferredX + 20; x += 0.1) {
        // Start from preferred position
        if (!wouldOverlap(x, y)) {
          return { x, y };
        }
      }
    }

    // Last resort: place at preferred position (shouldn't happen)
    return { x: preferredX, y: preferredY };
  }

  // First pass: assign grid positions respecting dependencies
  const processedGroups = new Set<string>();

  // Calculate dependency ranks first
  const calculateDependencyRanks = () => {
    const ranks = new Map<string, number>();
    const visited = new Set<string>();

    function calculateRank(groupId: string): number {
      if (ranks.has(groupId)) return ranks.get(groupId)!;
      if (visited.has(groupId)) return 0; // Circular dependency fallback

      visited.add(groupId);

      // Find all groups that this group depends on
      let maxDependencyRank = -1;
      for (const [sourceId, dependents] of dependencyGraph.entries()) {
        if (dependents.includes(groupId)) {
          maxDependencyRank = Math.max(maxDependencyRank, calculateRank(sourceId));
        }
      }

      const rank = maxDependencyRank + 1;
      ranks.set(groupId, rank);
      visited.delete(groupId);
      return rank;
    }

    workflowGroups.forEach((group) => {
      calculateRank(group.id);
    });

    return ranks;
  };

  const dependencyRanks = calculateDependencyRanks();

  // Helper function to calculate minimum X position for a dependent workflow
  function calculateMinXForDependent(groupId: string): number {
    let minX = 0;

    // Find all dependencies of this group
    for (const [sourceId, dependents] of dependencyGraph.entries()) {
      if (dependents.includes(groupId)) {
        // This group depends on sourceId
        const sourcePos = gridPositions.get(sourceId);
        if (sourcePos) {
          const sourceDimensions = spacingInfo.groupDimensions.get(sourceId) || { width: 300, height: 200 };
          // Calculate the pixel position where the source ends
          const sourceEndX = sourcePos.x * gridSpacing.x + sourceDimensions.width;
          // Convert back to grid units and add a small buffer
          const sourceEndGridX = Math.ceil(sourceEndX / gridSpacing.x) + 0.5; // 0.5 grid units buffer
          minX = Math.max(minX, sourceEndGridX);
        }
      }
    }

    return minX;
  }

  // Helper function to recursively position groups with dependency-aware X positioning
  function positionGroupAndDependents(groupId: string, preferredY: number): { x: number; y: number } {
    if (processedGroups.has(groupId)) {
      return gridPositions.get(groupId)!;
    }

    const group = groupsByKey.get(groupId);
    if (!group) return { x: 0, y: 0 };

    // Calculate minimum X position based on dependencies
    const minX = calculateMinXForDependent(groupId);
    const rank = dependencyRanks.get(groupId) || 0;
    const preferredX = Math.max(rank, minX); // Use the larger of rank or minimum X

    // Find available position with dependency-aware collision detection
    const gridPos = findAvailablePosition(preferredX, preferredY, groupId);

    // Reserve this position
    occupiedCells.add(`${gridPos.x},${gridPos.y}`);
    gridPositions.set(groupId, gridPos);
    processedGroups.add(groupId);

    // Process dependents (workflows that depend on this one)
    const dependents = dependencyGraph.get(groupId) || [];

    let nextAvailableY = gridPos.y; // Start from parent's Y position

    dependents.forEach((dependentId, index) => {
      // For multiple dependents, stack them vertically
      const dependentPreferredY = nextAvailableY + index;

      // Position dependents (they will automatically be positioned to the right due to dependency calculation)
      const dependentPos = positionGroupAndDependents(dependentId, dependentPreferredY);
      // Update next available Y to be below this dependent
      nextAvailableY = Math.max(nextAvailableY, dependentPos.y + 1);
    });

    return gridPos;
  }

  // Start with workflows that have no dependencies (root workflows)
  const rootWorkflows = workflowGroups.filter((group) => {
    // Check if this group has no incoming dependencies
    for (const [, neighbors] of dependencyGraph.entries()) {
      if (neighbors.includes(group.id)) {
        return false; // This group depends on something
      }
    }
    return true; // This is a root workflow
  });

  // Position root workflows first, ensuring they don't overlap
  let nextRootY = 0;
  rootWorkflows.forEach((group) => {
    const rootPos = positionGroupAndDependents(group.id, nextRootY);
    // Update next available Y to be below this root and all its dependents
    nextRootY = Math.max(nextRootY, rootPos.y + 1);

    // Also account for space taken by dependents
    const dependents = dependencyGraph.get(group.id) || [];
    if (dependents.length > 0) {
      nextRootY = Math.max(nextRootY, rootPos.y + dependents.length);
    }
  });

  // Handle any remaining workflows (shouldn't happen with proper dependency graph)
  workflowGroups.forEach((group) => {
    if (!processedGroups.has(group.id)) {
      const rank = dependencyRanks.get(group.id) || 0;
      const gridPos = findAvailablePosition(rank, processedGroups.size, group.id);
      occupiedCells.add(`${gridPos.x},${gridPos.y}`);
      gridPositions.set(group.id, gridPos);
      processedGroups.add(group.id);
    }
  });

  // Convert grid positions to actual pixel positions
  const positionedGroups = workflowGroups.map((group) => {
    const gridPos = gridPositions.get(group.id) || { x: 0, y: 0 };
    const xPosition = gridPos.x * gridSpacing.x;
    const yPosition = gridPos.y * gridSpacing.y;

    const xOffset = xPosition - group.position.x;
    const yOffset = yPosition - group.position.y;

    xOffsets.set(group.id, xOffset);
    yOffsets.set(group.id, yOffset);

    return {
      ...group,
      position: {
        x: xPosition,
        y: yPosition,
      },
    };
  });

  return { workflowGroups: positionedGroups, xOffsets, yOffsets };
}

// Helper function to aggregate workflow states for matrix workflows
function getAggregatedWorkflowState(workflows: PipelineWorkflow[]): PipelineStatus {
  if (workflows.length === 1) {
    return workflows[0].state;
  }

  // Priority order: error > failure > killed > running > pending > success > skipped
  const statePriority: Record<PipelineStatus, number> = {
    error: 7,
    failure: 6,
    killed: 5,
    running: 4,
    pending: 3,
    success: 2,
    skipped: 1,
    blocked: 0,
    declined: 0,
    started: 4, // Same as running
  };

  let highestPriority = 0;
  let resultState: PipelineStatus = 'success';

  workflows.forEach((workflow) => {
    const priority = statePriority[workflow.state] || 0;
    if (priority > highestPriority) {
      highestPriority = priority;
      resultState = workflow.state;
    }
  });

  return resultState;
}

// Handle node click
function handleNodeClick(event: any) {
  const step = event.node?.data?.step as PipelineStep;
  if (step) {
    selectedNodeId.value = event.node.id;
    emit('update:selected-step-id', step.pid);
  }
}

// Handle keyboard navigation
function handleKeyDown(event: KeyboardEvent) {
  if (!selectedNodeId.value) return;

  event.preventDefault();

  const currentNode = filteredNodes.value.find((node) => node.id === selectedNodeId.value);
  if (!currentNode) return;

  let nextNode: Node | null = null;

  switch (event.key) {
    case 'ArrowRight': {
      // Find nodes connected via outgoing edges
      const outgoingEdges = filteredEdges.value.filter((edge) => edge.source === selectedNodeId.value);
      if (outgoingEdges.length > 0) {
        nextNode = filteredNodes.value.find((node) => node.id === outgoingEdges[0].target) || null;
      }
      break;
    }
    case 'ArrowLeft': {
      // Find nodes connected via incoming edges
      const incomingEdges = filteredEdges.value.filter((edge) => edge.target === selectedNodeId.value);
      if (incomingEdges.length > 0) {
        nextNode = filteredNodes.value.find((node) => node.id === incomingEdges[0].source) || null;
      }
      break;
    }
    case 'ArrowDown':
    case 'ArrowUp': {
      // Navigate to next/previous step in the same workflow
      const stepNodes = filteredNodes.value.filter(
        (node) => node.type === 'pipeline-step' && node.data?.workflow?.pid === currentNode.data?.workflow?.pid,
      );
      const currentIndex = stepNodes.findIndex((node) => node.id === selectedNodeId.value);
      if (currentIndex !== -1) {
        const nextIndex =
          event.key === 'ArrowDown'
            ? (currentIndex + 1) % stepNodes.length
            : (currentIndex - 1 + stepNodes.length) % stepNodes.length;
        nextNode = stepNodes[nextIndex];
      }
      break;
    }
    case 'Enter':
      // Trigger click on current node
      if (currentNode.data?.step) {
        emit('update:selected-step-id', currentNode.data.step.pid);
      }
      return;
  }

  if (nextNode && nextNode.data?.step) {
    selectedNodeId.value = nextNode.id;
    emit('update:selected-step-id', nextNode.data.step.pid);

    // Center the view on the selected node
    nextTick(() => {
      if (vueFlowRef.value) {
        vueFlowRef.value.fitView({
          nodes: [nextNode],
          duration: 300,
          padding: 0.3,
        });
      }
    });
  }
}

// Clear all filters
function clearFilters() {
  searchQuery.value = '';
  selectedStateFilter.value = 'all';
  selectedWorkflowFilter.value = 'all';
}

// Watch for pipeline changes
watch(
  () => props.pipeline,
  (newPipeline, oldPipeline) => {
    // Clear existing debounce timeout
    if (debounceTimeout.value) {
      clearTimeout(debounceTimeout.value);
    }

    // Debounce the update to prevent excessive re-renders
    debounceTimeout.value = setTimeout(() => {
      // Only rebuild if structure changed (workflows added/removed or steps changed)
      if (
        !oldPipeline ||
        !oldPipeline.workflows ||
        !newPipeline.workflows ||
        newPipeline.workflows.length !== oldPipeline.workflows.length ||
        hasStructuralChanges(newPipeline, oldPipeline)
      ) {
        // Clear existing state before building new DAG
        nodes.value = [];
        edges.value = [];

        // Build new DAG
        buildDAG();
      } else {
        // Only update node states for status changes (no loading needed)
        updateNodeStates(newPipeline);
      }
    }, 50); // Reduced debounce time for more responsive UI
  },
  { deep: true },
);

// Watch for selected step changes to update node data
watch(
  () => props.selectedStepId,
  () => {
    // Update node data to reflect selection changes
    nodes.value = nodes.value.map((node) => ({
      ...node,
      data: {
        ...node.data,
        selectedStepId: props.selectedStepId,
      },
    }));
  },
);

// Helper function to check if there are structural changes
function hasStructuralChanges(newPipeline: Pipeline, oldPipeline: Pipeline): boolean {
  if (!newPipeline.workflows || !oldPipeline.workflows) return true;

  return newPipeline.workflows.some((newWorkflow, index) => {
    const oldWorkflow = oldPipeline.workflows?.[index];
    if (!oldWorkflow) return true;

    // Check if workflow structure changed
    if (
      newWorkflow.pid !== oldWorkflow.pid ||
      newWorkflow.name !== oldWorkflow.name ||
      !newWorkflow.children ||
      !oldWorkflow.children ||
      newWorkflow.children.length !== oldWorkflow.children.length
    ) {
      return true;
    }

    // Check if step structure changed
    return newWorkflow.children.some((newStep, stepIndex) => {
      const oldStep = oldWorkflow.children?.[stepIndex];
      return (
        !oldStep ||
        newStep.pid !== oldStep.pid ||
        newStep.name !== oldStep.name ||
        JSON.stringify(newStep.depends_on) !== JSON.stringify(oldStep.depends_on)
      );
    });
  });
}

// Helper function to update only node states without rebuilding
function updateNodeStates(pipeline: Pipeline) {
  nodes.value = nodes.value.map((node) => {
    if (node.type === 'pipeline-step') {
      // Find the corresponding step in the updated pipeline
      const workflow = pipeline.workflows?.find((w) => w.pid === node.data?.workflow?.pid);
      const step = workflow?.children?.find((s) => s.pid === node.data?.step?.pid);

      if (step && workflow) {
        return {
          ...node,
          data: {
            ...node.data,
            step,
            workflow,
            pipeline,
          },
        };
      }
    } else if (node.type === 'workflow-group') {
      // Update workflow group data - handle both regular and matrix workflow groups
      if (node.data?.isMatrixGroup && node.data?.matrixWorkflows) {
        // Update matrix workflow group
        const updatedMatrixWorkflows = node.data.matrixWorkflows.map((matrixWorkflow: PipelineWorkflow) => {
          return pipeline.workflows?.find((w) => w.pid === matrixWorkflow.pid) || matrixWorkflow;
        });

        const representativeWorkflow = updatedMatrixWorkflows[0];
        if (representativeWorkflow) {
          const displayWorkflow: PipelineWorkflow = {
            id: representativeWorkflow.id,
            pipeline_id: representativeWorkflow.pipeline_id,
            pid: representativeWorkflow.pid,
            name: node.data.workflow.name, // Keep the base name
            pipeline_name: representativeWorkflow.pipeline_name,
            state: getAggregatedWorkflowState(updatedMatrixWorkflows),
            environ: representativeWorkflow.environ,
            started: representativeWorkflow.started,
            finished: representativeWorkflow.finished,
            agent_id: representativeWorkflow.agent_id,
            error: representativeWorkflow.error,
            depends_on: representativeWorkflow.depends_on,
            children: representativeWorkflow.children,
          };

          return {
            ...node,
            data: {
              ...node.data,
              workflow: displayWorkflow,
              matrixWorkflows: updatedMatrixWorkflows,
            },
          };
        }
      } else {
        // Update regular workflow group
        const workflow = pipeline.workflows?.find((w) => w.pid === node.data?.workflow?.pid);
        if (workflow) {
          return {
            ...node,
            data: {
              ...node.data,
              workflow,
            },
          };
        }
      }
    }
    return node;
  });

  // Recreate edges to ensure they are preserved and updated
  const newEdges: Edge[] = [];
  const globalStepMap = new Map<string, Node>();

  // Build step map from current nodes
  nodes.value.forEach((node) => {
    if (node.type === 'pipeline-step' && node.data?.workflow && node.data?.step) {
      globalStepMap.set(`${node.data.workflow.pid}-${node.data.step.name}`, node);
    }
  });

  // Recreate edges based on current pipeline data (same logic as buildDAG)
  const workflowMap = new Map<string, PipelineWorkflow>();
  pipeline.workflows?.forEach((workflow) => {
    workflowMap.set(workflow.name, workflow);
  });

  // Identify matrix groups for edge creation
  const matrixGroups = identifyMatrixGroupsMap(pipeline.workflows || []);

  // Step-level dependencies
  pipeline.workflows?.forEach((workflow) => {
    workflow.children?.forEach((step) => {
      const targetNode = globalStepMap.get(`${workflow.pid}-${step.name}`);
      if (!targetNode) return;

      if (step.depends_on && step.depends_on.length > 0) {
        step.depends_on.forEach((dependency) => {
          let sourceNode: Node | undefined;

          // First, try to find dependency within the same workflow
          sourceNode = globalStepMap.get(`${workflow.pid}-${dependency}`);

          // If not found, try enhanced search like in buildDAG
          if (!sourceNode) {
            const baseWorkflowName = workflow.name.replace(/-[^-]*$/, '');
            for (const [key, node] of globalStepMap.entries()) {
              if (key.endsWith(`-${dependency}`) && key.includes(baseWorkflowName)) {
                sourceNode = node;
                break;
              }
            }
          }

          if (!sourceNode) {
            for (const [key, node] of globalStepMap.entries()) {
              if (key.endsWith(`-${dependency}`)) {
                sourceNode = node;
                break;
              }
            }
          }

          if (sourceNode) {
            const edge: Edge = {
              id: `edge-${sourceNode.id}-${targetNode.id}`,
              source: sourceNode.id,
              target: targetNode.id,
              sourceHandle: `${sourceNode.id}-source`,
              targetHandle: `${targetNode.id}-target`,
              type: 'smoothstep',
              animated: step.state === 'running',
              markerEnd: {
                type: 'platform',
                color: '#6b7280',
              } as any,
            };
            newEdges.push(edge);
          }
        });
      }
    });
  });

  // Workflow-level dependencies with matrix awareness
  const workflowDependencyEdges = createWorkflowDependencyEdges(
    matrixGroups,
    new Map(
      Array.from(globalStepMap.entries()).reduce((acc, [key, node]) => {
        const match = key.match(/^(\d+)-/);
        if (match) {
          const workflowPid = Number.parseInt(match[1], 10);
          if (!acc.has(workflowPid)) {
            acc.set(workflowPid, []);
          }
          acc.get(workflowPid)!.push(node);
        }
        return acc;
      }, new Map<number, Node[]>()),
    ),
    workflowMap,
    [],
  );
  newEdges.push(...workflowDependencyEdges);

  edges.value = newEdges;
}

// Watch for workflow state changes to trigger layout (only for structural changes)
watch(
  () =>
    props.pipeline.workflows?.map((w) => ({
      pid: w.pid,
      name: w.name,
      childCount: w.children?.length || 0,
    })),
  (newWorkflows, oldWorkflows) => {
    // Only trigger layout if workflow structure changed
    if (!oldWorkflows || JSON.stringify(newWorkflows) !== JSON.stringify(oldWorkflows)) {
      // Clear existing debounce timeout
      if (debounceTimeout.value) {
        clearTimeout(debounceTimeout.value);
      }

      debounceTimeout.value = setTimeout(() => {
        nextTick(() => {
          const stepNodes = nodes.value.filter((node) => node.type === 'pipeline-step');
          if (stepNodes.length > 0) {
            applyLayout('LR');
          }
        });
      }, 150);
    }
  },
  { deep: true },
);

// Handle window resize to ensure proper fit
function handleResize() {
  nextTick(() => {
    setTimeout(() => {
      if (vueFlowRef.value && nodes.value.length > 0) {
        vueFlowRef.value.fitView({ padding: 0.1, includeHiddenNodes: false });
      }
    }, 100);
  });
}

// Initialize on mount
onMounted(() => {
  buildDAG();

  // Add resize listener
  window.addEventListener('resize', handleResize);
});

// Clean up resize listener and debounce timeout
onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize);

  if (debounceTimeout.value) {
    clearTimeout(debounceTimeout.value);
  }
});
</script>

<style scoped>
/* eslint-disable vue-scoped-css/no-unused-selector */
.vue-flow {
  background-color: transparent;
  height: 100% !important;
  min-height: 400px;
  width: 100% !important;
}

.vue-flow__viewport {
  height: 100% !important;
  width: 100% !important;
}

.vue-flow__pane {
  height: 100% !important;
  width: 100% !important;
}

.vue-flow__renderer {
  height: 100% !important;
  width: 100% !important;
}

.vue-flow__controls {
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  border-radius: 8px;
  background: white;
  border: 1px solid #e5e7eb;
  backdrop-filter: blur(8px);
}

.vue-flow__minimap {
  border-radius: 8px;
  border: 1px solid #e5e7eb;
  background: white;
  backdrop-filter: blur(8px);
}

.dark .vue-flow__controls {
  background: rgba(55, 65, 81, 0.95);
  border-color: #4b5563;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(16px);
}

.dark .vue-flow__minimap {
  background: rgba(55, 65, 81, 0.95);
  border-color: #4b5563;
  backdrop-filter: blur(16px);
}

/* Handle positioning for left-to-right flow - Force rectangular platform shape */
.vue-flow__handle {
  border-radius: 1px !important; /* Small rounded corners for platform look */
  border: none !important;
  box-shadow: none !important;
  /* Override VueFlow's default circular styling */
  width: auto !important;
  height: auto !important;
}

/* Step node handles - thin vertical platforms */
.step-handle.vue-flow__handle {
  width: 4px !important;
  height: 16px !important;
  border-radius: 1px !important;
  opacity: 0.6;
}

/* Workflow group handles - slightly larger platforms */
.workflow-handle.vue-flow__handle {
  width: 6px !important;
  height: 24px !important;
  border-radius: 1px !important;
  opacity: 0.5;
}

/* Universal override to force rectangular handles */
div.vue-flow__handle {
  border-radius: 1px !important;
}

.vue-flow__handle-left {
  left: -2px !important;
  top: 50% !important;
  transform: translateY(-50%) !important;
}

.vue-flow__handle-right {
  right: -2px !important;
  top: 50% !important;
  transform: translateY(-50%) !important;
}

/* Ensure proper edge routing */
.vue-flow__edge-path {
  stroke: #6b7280;
  stroke-width: 2;
}

.vue-flow__edge.animated .vue-flow__edge-path {
  stroke: #3b82f6;
  stroke-width: 2;
  animation: dash 0.5s linear infinite;
}

@keyframes dash {
  to {
    stroke-dashoffset: -10;
  }
}

/* Workflow group styling */
.vue-flow__node-workflow-group {
  pointer-events: none;
  z-index: 100 !important;
  isolation: isolate;
}

.vue-flow__node-workflow-group > div {
  opacity: 1 !important; /* Ensure full opacity */
  background: transparent !important;
  border-style: dashed !important;
  border-width: 2px !important;
}

/* Ensure edges appear above workflow groups but below step nodes */
.vue-flow__edge {
  z-index: 500 !important;
  isolation: isolate;
}

.vue-flow__edge-path {
  z-index: 500 !important;
  stroke-width: 2px !important;
  opacity: 1 !important;
}

.vue-flow__edge-interaction {
  z-index: 500 !important;
}

.vue-flow__edge-text {
  z-index: 500 !important;
}

.vue-flow__edge-label {
  z-index: 500 !important;
}

.vue-flow__edges {
  z-index: 500 !important;
  isolation: isolate;
}

.vue-flow__edge-wrapper {
  z-index: 500 !important;
}

/* Ensure workflow labels don't overlap with step nodes */
.vue-flow__node-workflow-group .absolute {
  min-height: 32px;
  white-space: nowrap;
  z-index: 20 !important;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
}

.vue-flow__node-workflow-group:hover {
  transform: scale(1.005);
  transition: transform 0.2s ease-in-out;
}

/* Ensure step nodes appear above everything */
.vue-flow__node-pipeline-step {
  z-index: 2000 !important;
  position: relative;
  opacity: 1 !important; /* Ensure full opacity */
}

/* Add subtle shadow and hover effects for workflow groups */
.vue-flow__node-workflow-group > div {
  backdrop-filter: blur(1px);
  transition: all 0.3s ease;
  opacity: 1 !important; /* Ensure full opacity */
}

.vue-flow__node-workflow-group:hover > div {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  border-width: 3px;
}

/* Workflow group label styling */
.vue-flow__node-workflow-group .absolute {
  backdrop-filter: blur(8px);
  transition: all 0.2s ease;
}

/* Enhanced workflow badge styling */
.vue-flow__node-workflow-group .absolute > div {
  transition: all 0.2s ease;
}

.vue-flow__node-workflow-group:hover .absolute > div {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

/* Dark mode improvements for workflow badges */
.dark .vue-flow__node-workflow-group .absolute > div {
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4);
}

.dark .vue-flow__node-workflow-group:hover .absolute > div {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.6);
}

/* Improve visual hierarchy with better spacing */
.vue-flow__node-pipeline-step {
  margin: 4px;
  opacity: 1 !important; /* Ensure full opacity */
}

.vue-flow__node-pipeline-step > div {
  opacity: 1 !important; /* Ensure full opacity for step content */
}

/* Add subtle animation for workflow state changes */
.vue-flow__node-workflow-group[data-state='running'] > div {
  animation: pulse-workflow 2s infinite;
}

@keyframes pulse-workflow {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.8;
  }
}

/* Dark mode support for workflow groups */
.dark .vue-flow__node-workflow-group > div {
  background-color: rgba(55, 65, 81, 0.1);
}

.dark .vue-flow__node-workflow-group .absolute {
  backdrop-filter: blur(8px);
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
}

/* Dark mode support for background */
.dark .vue-flow {
  background-color: #111827;
}

.dark .vue-flow__pane {
  background-color: #111827;
}

.dark .vue-flow__viewport {
  background-color: #111827;
}

/* Dark mode support for edges */
.dark .vue-flow__edge-path {
  stroke: #d1d5db;
  stroke-width: 2px;
}

.dark .vue-flow__edge.animated .vue-flow__edge-path {
  stroke: #3b82f6;
  stroke-width: 2px;
}

/* Dark mode support for handles - handled by component classes */

/* Ensure workflow labels have proper positioning */
.vue-flow__node-workflow-group {
  position: relative;
}

.vue-flow__node-workflow-group .absolute {
  min-width: fit-content;
  max-width: calc(100% - 32px);
}

/* Center text within workflow labels */
.vue-flow__node-workflow-group .absolute span {
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  width: 100%;
}
</style>

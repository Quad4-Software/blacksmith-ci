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
            class="w-full pl-8 pr-3 py-1.5 text-base md:text-sm border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-1 focus:ring-primary focus:border-primary bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
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
            <PipelineDagStepNode
              :id="nodeProps.id"
              :data="nodeProps.data"
              :selected="selectedNodeId === nodeProps.id"
            />
          </template>

          <template #node-workflow-group="nodeProps">
            <PipelineDagWorkflowNode :id="nodeProps.id" :data="nodeProps.data" />
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
import { Background } from '@vue-flow/background';
import { Controls } from '@vue-flow/controls';
import { useVueFlow, VueFlow } from '@vue-flow/core';
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import PipelineDagStepNode from './PipelineDagStepNode.vue';
import PipelineDagWorkflowNode from './PipelineDagWorkflowNode.vue';
import PlatformMarker from './PlatformMarker.vue';
import '@vue-flow/core/dist/style.css';
import '@vue-flow/core/dist/theme-default.css';
import '@vue-flow/controls/dist/style.css';
import type { Edge, Node } from '@vue-flow/core';
import { useLayout } from '~/compositions/useLayout';
import type { Pipeline, PipelineStep } from '~/lib/api/types';
import { filterPipelineGraph } from '~/lib/pipeline-dag-filter';
import { buildPipelineGraph, GROUP_NODE_TYPE, STEP_NODE_TYPE } from '~/lib/pipeline-dag-graph';
import type { StepNodeData } from '~/lib/pipeline-dag-graph';
import { getLatestAttempts } from '~/lib/pipeline-utils';

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

interface Props {
  pipeline: Pipeline;
  selectedStepId?: number | null;
}

interface Emits {
  (event: 'update:selected-step-id', stepId: number): void;
  (event: 'dagLoading', loading: boolean): void;
  (event: 'dagReady'): void;
}

const { t } = useI18n();

const nodes = shallowRef<Node[]>([]);
const edges = shallowRef<Edge[]>([]);
const { layout } = useLayout();
const debounceTimeout = ref<ReturnType<typeof setTimeout> | null>(null);
const isLayouting = ref(false);
const isInitialized = ref(false);

// Lifecycle guard: deferred callbacks (layout/fitView timers, rAF chains) must
// not run after the component starts unmounting, otherwise they mutate Vue Flow
// state while its node DOM elements are already gone, producing the intermittent
// "can't access property parentNode of null" crash from a stale internal effect.
let isActive = true;
const pendingTimers = new Set<ReturnType<typeof setTimeout>>();
const pendingRafs = new Set<number>();

function trackedSetTimeout(fn: () => void, delay: number) {
  if (!isActive) return;
  const id = setTimeout(() => {
    pendingTimers.delete(id);
    if (isActive) {
      fn();
    }
  }, delay);
  pendingTimers.add(id);
}

function trackedRaf(fn: () => void) {
  if (!isActive) return;
  const id = requestAnimationFrame(() => {
    pendingRafs.delete(id);
    if (isActive) {
      fn();
    }
  });
  pendingRafs.add(id);
}

// Use VueFlow composable with the specific flow ID
const { fitView } = useVueFlow('pipeline-dag');

// Create a separate ref for the VueFlow component
const vueFlowRef = ref();

// Search and filter state
const searchQuery = ref('');
const selectedStateFilter = ref('all');
const selectedWorkflowFilter = ref('all');
const selectedNodeId = ref<string | null>(null);

const workflows = computed(() => getLatestAttempts(props.pipeline.workflows || []));

const filteredNodes = computed(() =>
  filterPipelineGraph(nodes.value, {
    query: searchQuery.value,
    state: selectedStateFilter.value,
    workflow: selectedWorkflowFilter.value,
  }),
);

const filteredEdges = computed(() => {
  const visibleNodeIds = new Set(filteredNodes.value.map((node: Node) => node.id));
  return edges.value.filter((edge) => visibleNodeIds.has(edge.source) && visibleNodeIds.has(edge.target));
});

// Pipeline progress calculation
const pipelineProgress = computed(() => {
  const stepNodes = nodes.value.filter((node: Node) => node.type === STEP_NODE_TYPE);
  const total = stepNodes.length;
  const completed = stepNodes.filter((node: Node) => {
    const state = (node.data as StepNodeData).step.state;
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
      font-family: var(--font-sans);
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

// Handle nodes initialization and layout
function onNodesInitialized() {
  if (isLayouting.value) return; // Prevent multiple layout attempts

  isLayouting.value = true;
  emit('dagLoading', true);

  // Wait for nodes to be fully rendered before applying layout
  nextTick(() => {
    trackedSetTimeout(() => {
      applyLayout();
    }, 150);
  });
}

// Positions the nodes once every step node has been measured, then fits the view.
function applyLayout() {
  if (nodes.value.length === 0) {
    return;
  }

  const stepNodes = nodes.value.filter((node) => node.type === STEP_NODE_TYPE);
  const measured = stepNodes.every((node) => vueFlowRef.value?.findNode(node.id)?.dimensions?.width > 0);
  if (vueFlowRef.value && !measured) {
    // Retry after a short delay if nodes don't have dimensions yet
    trackedSetTimeout(() => applyLayout(), 25);
    return;
  }

  nodes.value = layout(nodes.value, edges.value);

  // Fit the view once the positioned nodes have been rendered
  nextTick(() => {
    trackedSetTimeout(() => {
      trackedRaf(() => {
        trackedRaf(() => {
          trackedRaf(() => {
            if (vueFlowRef.value) {
              vueFlowRef.value.fitView({
                padding: 0.1,
                includeHiddenNodes: false,
                duration: 200, // Smooth animation for fit-to-view
              });

              // Mark layout as complete and emit ready event
              trackedSetTimeout(() => {
                isLayouting.value = false;
                isInitialized.value = true;
                emit('dagReady');
              }, 300); // Wait for fit-to-view animation to complete
            }
          });
        });
      });
    }, 100);
  });
}

// Builds the graph from scratch; the layout runs once the nodes are rendered.
function buildDAG() {
  isInitialized.value = false;
  isLayouting.value = false;
  emit('dagLoading', true);

  const graph = buildPipelineGraph(workflows.value, props.pipeline.status);
  nodes.value = graph.nodes;
  edges.value = graph.edges;
}

// Handle node click
function handleNodeClick(event: any) {
  const step = event.node?.data?.step as PipelineStep | undefined;
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
      if (!isActive) {
        return;
      }
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
        buildDAG();
      } else {
        // Only update node states for status changes (no loading needed)
        updateNodeStates();
      }
    }, 50); // Reduced debounce time for more responsive UI
  },
  { deep: true },
);

// Highlight the step that is selected outside of the graph, e.g. in the step list
watch(
  () => props.selectedStepId,
  (stepId) => {
    selectedNodeId.value = stepId != null ? `step-${stepId}` : null;
  },
  { immediate: true },
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

// Rebuilds the nodes and edges for new step states, keeping the layout: the
// structure is unchanged, so every node keeps its position and size.
function updateNodeStates() {
  const previous = new Map(nodes.value.map((node) => [node.id, node]));
  const graph = buildPipelineGraph(workflows.value, props.pipeline.status);
  nodes.value = graph.nodes.map((node) => {
    const old = previous.get(node.id);
    if (!old) {
      return node;
    }
    if (node.type === GROUP_NODE_TYPE) {
      return {
        ...node,
        position: old.position,
        data: { ...node.data, width: old.data.width, height: old.data.height },
      };
    }
    return { ...node, position: old.position };
  });
  edges.value = graph.edges;
}

// Watch for workflow state changes to trigger layout (only for structural changes)
watch(
  () =>
    workflows.value.map((w) => ({
      pid: w.pid,
      name: w.name,
      childCount: w.children?.length || 0,
    })),
  (newWorkflows, oldWorkflows) => {
    // Only trigger layout if workflow structure changed
    // It has its own timer: sharing the one of the pipeline watcher would cancel
    // the rebuild that the same structural change scheduled there.
    if (!oldWorkflows || JSON.stringify(newWorkflows) !== JSON.stringify(oldWorkflows)) {
      trackedSetTimeout(() => {
        if (nodes.value.some((node) => node.type === STEP_NODE_TYPE)) {
          applyLayout();
        }
      }, 150);
    }
  },
  { deep: true },
);

// Handle window resize to ensure proper fit
function handleResize() {
  nextTick(() => {
    trackedSetTimeout(() => {
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

onBeforeUnmount(() => {
  isActive = false;
  window.removeEventListener('resize', handleResize);

  if (debounceTimeout.value) {
    clearTimeout(debounceTimeout.value);
  }

  pendingTimers.forEach((id) => clearTimeout(id));
  pendingTimers.clear();
  pendingRafs.forEach((id) => cancelAnimationFrame(id));
  pendingRafs.clear();
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
</style>

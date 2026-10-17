// Copyright 2025 Crow authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { useVueFlow } from '@vue-flow/core';
import type { Edge, Node } from '@vue-flow/core';
import { GROUP_NODE_TYPE, STEP_NODE_TYPE } from '~/lib/pipeline-dag-graph';
import type { GroupNodeData, StepNodeData } from '~/lib/pipeline-dag-graph';
import { layoutPipelineGraph } from '~/lib/pipeline-dag-layout';
import type { GraphLayout, LayoutGroup } from '~/lib/pipeline-dag-layout';

// Layouts of recently drawn graphs, so that switching back to a pipeline does
// not lay it out again.
const layoutCache = new Map<string, GraphLayout>();
const LAYOUT_CACHE_SIZE = 50;

// The group header holds the workflow name, a matrix count and a dependency
// count, so its width is estimated from the name.
function headerWidth(data: GroupNodeData): number {
  return data.workflow.name.length * 9 + ((data.workflow.depends_on ?? []).length > 0 ? 260 : 120);
}

/**
 * Composable to position the nodes of the pipeline graph. Step nodes are
 * measured once they are rendered; see layoutPipelineGraph for the algorithm.
 */
export function useLayout() {
  const { findNode } = useVueFlow('pipeline-dag');

  // Returns the measured size of a node, or a default before it is rendered.
  function nodeSize(id: string): { width: number; height: number } {
    const dimensions = findNode(id)?.dimensions;
    return {
      width: dimensions && dimensions.width > 0 ? dimensions.width : 240,
      height: dimensions && dimensions.height > 0 ? dimensions.height : 120,
    };
  }

  /** Returns the nodes with their layout positions, and the group nodes with their sizes. */
  function layout(nodes: Node[], edges: Edge[]): Node[] {
    const groups = new Map<string, LayoutGroup>();
    for (const node of nodes) {
      if (node.type === GROUP_NODE_TYPE) {
        groups.set(node.id, { id: node.id, nodes: [], minWidth: headerWidth(node.data as GroupNodeData) });
      }
    }
    for (const node of nodes) {
      if (node.type === STEP_NODE_TYPE) {
        groups.get((node.data as StepNodeData).groupId)?.nodes.push({ id: node.id, ...nodeSize(node.id) });
      }
    }
    const isStepEdge = (edge: Edge) => !groups.has(edge.source);
    const stepEdges = edges.filter(isStepEdge);
    const groupEdges = edges.filter((edge) => !isStepEdge(edge));

    // Positions depend only on the graph and the node sizes, not on step
    // states, so a state change that leaves the sizes alone reuses the layout.
    const cacheKey = JSON.stringify([
      [...groups.values()].map((group) => [group.id, group.minWidth, group.nodes]),
      edges.map((edge) => [edge.source, edge.target]),
    ]);
    let graphLayout = layoutCache.get(cacheKey);
    if (!graphLayout) {
      graphLayout = layoutPipelineGraph([...groups.values()], stepEdges, groupEdges);
      layoutCache.set(cacheKey, graphLayout);
      if (layoutCache.size > LAYOUT_CACHE_SIZE) {
        layoutCache.delete(layoutCache.keys().next().value!);
      }
    }

    return nodes.map((node) => {
      const box = graphLayout.groups.get(node.id);
      if (box) {
        return {
          ...node,
          position: { x: box.x, y: box.y },
          data: { ...(node.data as GroupNodeData), width: box.width, height: box.height },
        };
      }
      const position = graphLayout.nodes.get(node.id);
      return position ? { ...node, position } : node;
    });
  }

  return { layout };
}

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

/* eslint-disable ts/no-unsafe-assignment, ts/no-unsafe-call, ts/no-unsafe-member-access, ts/no-unnecessary-type-assertion */
import dagre from '@dagrejs/dagre';
import { Position, useVueFlow } from '@vue-flow/core';
import type { Node, Edge } from '@vue-flow/core';

// Type for node data with optional step
interface NodeDataWithStep {
  step?: {
    state?: string;
  };
}

// Helper function to safely get step state from node data
function getStepState(nodeData: unknown): string {
  const data = nodeData as NodeDataWithStep;
  return data?.step?.state ?? 'unknown';
}

/**
 * Composable to run the layout algorithm on the graph.
 * It uses the `dagre` library to calculate the layout of the nodes and edges.
 */
// Layout cache for better performance
const layoutCache = new Map<string, Node[]>();

export function useLayout() {
  const { findNode } = useVueFlow();

  function layout(nodes: Node[], edges: Edge[], direction: string = 'LR') {
    // Create cache key based on nodes, edges, and direction
    const cacheKey = `${nodes.map((n) => `${n.id}:${getStepState(n.data)}`).join(',')}-${edges.map((e) => `${e.source}-${e.target}`).join(',')}-${direction}`;

    // Check cache first
    if (layoutCache.has(cacheKey)) {
      const cachedLayout = layoutCache.get(cacheKey)!;
      // Return cached layout with updated node data
      return cachedLayout.map((cachedNode) => {
        const currentNode = nodes.find((n) => n.id === cachedNode.id);
        return currentNode ? { ...currentNode, position: cachedNode.position } : cachedNode;
      });
    }
    const dagreGraph = new (dagre as any).graphlib.Graph();

    (dagreGraph as any).setDefaultEdgeLabel(() => ({}));

    const isHorizontal = direction === 'LR';
    // https://github.com/dagrejs/dagre/wiki#configuring-the-layout
    (dagreGraph as any).setGraph({
      rankdir: direction,
      nodesep: isHorizontal ? 30 : 0, // Horizontal separation between nodes
      ranksep: 20, // default: 50
      acyclicer: 'greedy',
      align: 'UL', // Align to upper-left for consistent positioning
    });

    for (const node of nodes) {
      const graphNode = findNode(node.id);

      // Get actual dimensions or use reasonable defaults
      const width =
        graphNode?.dimensions?.width != null && graphNode.dimensions.width > 0 ? graphNode.dimensions.width : 240;
      const height =
        graphNode?.dimensions?.height != null && graphNode.dimensions.height > 0 ? graphNode.dimensions.height : 120;

      (dagreGraph as any).setNode(node.id, {
        width,
        height,
      });
    }

    for (const edge of edges) {
      (dagreGraph as any).setEdge(edge.source, edge.target);
    }

    (dagre as any).layout(dagreGraph);

    // set nodes with updated positions
    const layoutedNodes = nodes.map((node) => {
      const nodeWithPosition = (dagreGraph as any).node(node.id);

      // Dagre positions are center-based, adjust for top-left positioning
      const adjustedX = (nodeWithPosition as any).x - (nodeWithPosition as any).width / 2;
      const adjustedY = (nodeWithPosition as any).y - (nodeWithPosition as any).height / 2;

      return {
        ...node,
        targetPosition: isHorizontal ? Position.Left : Position.Top,
        sourcePosition: isHorizontal ? Position.Right : Position.Bottom,
        position: {
          x: Math.round(adjustedX),
          y: Math.round(adjustedY),
        },
      };
    });

    // Cache the layout result
    layoutCache.set(cacheKey, layoutedNodes);

    // Limit cache size to prevent memory issues
    if (layoutCache.size > 50) {
      const firstKey = layoutCache.keys().next().value;
      if (firstKey !== undefined) {
        layoutCache.delete(firstKey);
      }
    }

    return layoutedNodes;
  }

  return { layout };
}

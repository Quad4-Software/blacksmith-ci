import dagre from '@dagrejs/dagre';

export interface LayoutSize {
  width: number;
  height: number;
}

export interface LayoutPoint {
  x: number;
  y: number;
}

export type LayoutBox = LayoutPoint & LayoutSize;

export interface LayoutEdge {
  source: string;
  target: string;
}

/** A workflow box and the step nodes laid out inside it. */
export interface LayoutGroup {
  id: string;
  nodes: (LayoutSize & { id: string })[];
  // Room the box needs for its header even when its steps are narrower.
  minWidth?: number;
}

export interface GraphLayout {
  // Top-left corner of every step node.
  nodes: Map<string, LayoutPoint>;
  // Top-left corner and size of every group box.
  groups: Map<string, LayoutBox>;
}

// The space between a group box and its steps; the top leaves room for the header.
export const GROUP_PADDING = { top: 55, right: 25, bottom: 20, left: 25 };

// The part of a dagre graph used here. The package's typings do not resolve
// under this project's module settings, so the graph is typed by hand.
interface DagreGraph {
  setNode: (id: string, size: LayoutSize) => void;
  setEdge: (source: string, target: string) => void;
  hasNode: (id: string) => boolean;
  // dagre sets the center of every node it lays out.
  node: (id: string) => LayoutBox;
}

/* eslint-disable ts/no-unsafe-assignment, ts/no-unsafe-call, ts/no-unsafe-member-access */
function newGraph(nodesep: number, ranksep: number): DagreGraph {
  const graph = new dagre.graphlib.Graph();
  graph.setGraph({ rankdir: 'LR', nodesep, ranksep, acyclicer: 'greedy', align: 'UL' });
  graph.setDefaultEdgeLabel(() => ({}));
  return graph as DagreGraph;
}

function runLayout(graph: DagreGraph) {
  dagre.layout(graph);
}
/* eslint-enable ts/no-unsafe-assignment, ts/no-unsafe-call, ts/no-unsafe-member-access */

// Returns the top-left corner of a dagre node, whose position is its center.
function topLeft(node: LayoutBox): LayoutPoint {
  return { x: node.x - node.width / 2, y: node.y - node.height / 2 };
}

/**
 * Lays out the steps of each group on their own, then lays out the groups as
 * boxes along the dependencies between them. Doing it in two passes keeps every
 * group's steps inside one box sized to fit them and its header, so boxes
 * cannot overlap, which a single pass over all steps does not guarantee.
 * Edges whose ends lie in different groups only order the groups.
 */
export function layoutPipelineGraph(
  groups: LayoutGroup[],
  stepEdges: LayoutEdge[],
  groupEdges: LayoutEdge[],
): GraphLayout {
  const nodes = new Map<string, LayoutPoint>();
  const boxes = new Map<string, LayoutBox>();
  const groupOf = new Map<string, string>();
  for (const group of groups) {
    for (const node of group.nodes) {
      groupOf.set(node.id, group.id);
    }
  }

  // Lay out the steps of each group relative to the group's top-left corner.
  const outer = newGraph(40, 80);
  for (const group of groups) {
    const inner = newGraph(30, 20);
    for (const node of group.nodes) {
      inner.setNode(node.id, { width: node.width, height: node.height });
    }
    for (const edge of stepEdges) {
      if (groupOf.get(edge.source) === group.id && groupOf.get(edge.target) === group.id) {
        inner.setEdge(edge.source, edge.target);
      }
    }
    runLayout(inner);

    let minX = Infinity;
    let minY = Infinity;
    let maxX = -Infinity;
    let maxY = -Infinity;
    for (const node of group.nodes) {
      const corner = topLeft(inner.node(node.id));
      minX = Math.min(minX, corner.x);
      minY = Math.min(minY, corner.y);
      maxX = Math.max(maxX, corner.x + node.width);
      maxY = Math.max(maxY, corner.y + node.height);
    }
    if (group.nodes.length === 0) {
      minX = minY = maxX = maxY = 0;
    }
    for (const node of group.nodes) {
      const corner = topLeft(inner.node(node.id));
      nodes.set(node.id, {
        x: corner.x - minX + GROUP_PADDING.left,
        y: corner.y - minY + GROUP_PADDING.top,
      });
    }

    const width = Math.max(maxX - minX + GROUP_PADDING.left + GROUP_PADDING.right, group.minWidth ?? 0);
    const height = maxY - minY + GROUP_PADDING.top + GROUP_PADDING.bottom;
    outer.setNode(group.id, { width, height });
  }

  // Order the groups by the dependencies between them.
  for (const edge of groupEdges) {
    if (edge.source !== edge.target && outer.hasNode(edge.source) && outer.hasNode(edge.target)) {
      outer.setEdge(edge.source, edge.target);
    }
  }
  runLayout(outer);

  for (const group of groups) {
    const box = outer.node(group.id);
    const corner = topLeft(box);
    boxes.set(group.id, { x: Math.round(corner.x), y: Math.round(corner.y), width: box.width, height: box.height });
    for (const node of group.nodes) {
      const offset = nodes.get(node.id)!;
      nodes.set(node.id, { x: Math.round(corner.x + offset.x), y: Math.round(corner.y + offset.y) });
    }
  }

  return { nodes, groups: boxes };
}

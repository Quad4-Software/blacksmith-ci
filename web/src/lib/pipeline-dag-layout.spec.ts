import { describe, expect, it } from 'vitest';
import { GROUP_PADDING, layoutPipelineGraph } from './pipeline-dag-layout';
import type { LayoutBox, LayoutGroup } from './pipeline-dag-layout';

const node = (id: string) => ({ id, width: 200, height: 100 });

function overlaps(a: LayoutBox, b: LayoutBox): boolean {
  return a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;
}

describe('layoutPipelineGraph', () => {
  const groups: LayoutGroup[] = [
    { id: 'build', nodes: [node('clone'), node('compile'), node('test')] },
    { id: 'deploy', nodes: [node('push')] },
    { id: 'lint', nodes: [node('eslint')] },
  ];
  const stepEdges = [
    { source: 'clone', target: 'compile' },
    { source: 'clone', target: 'test' },
  ];
  const layout = layoutPipelineGraph(groups, stepEdges, [{ source: 'build', target: 'deploy' }]);

  it('keeps every step inside its group, below the header', () => {
    for (const group of groups) {
      const box = layout.groups.get(group.id)!;
      for (const { id, width, height } of group.nodes) {
        const position = layout.nodes.get(id)!;
        expect(position.x).toBeGreaterThanOrEqual(box.x + GROUP_PADDING.left);
        expect(position.y).toBeGreaterThanOrEqual(box.y + GROUP_PADDING.top);
        expect(position.x + width).toBeLessThanOrEqual(box.x + box.width);
        expect(position.y + height).toBeLessThanOrEqual(box.y + box.height);
      }
    }
  });

  it('does not overlap groups', () => {
    const boxes = [...layout.groups.values()];
    for (const [i, a] of boxes.entries()) {
      for (const b of boxes.slice(i + 1)) {
        expect(overlaps(a, b)).toBe(false);
      }
    }
  });

  it('places a group to the right of the group it depends on', () => {
    const build = layout.groups.get('build')!;
    const deploy = layout.groups.get('deploy')!;
    expect(deploy.x).toBeGreaterThanOrEqual(build.x + build.width);
  });

  it('places a step to the right of the step it depends on', () => {
    expect(layout.nodes.get('compile')!.x).toBeGreaterThan(layout.nodes.get('clone')!.x + 200);
  });

  it('widens a group to its minimum width', () => {
    const wide = layoutPipelineGraph([{ id: 'g', nodes: [node('a')], minWidth: 600 }], [], []);
    expect(wide.groups.get('g')!.width).toBe(600);
  });

  it('ignores edges to unknown groups and self edges', () => {
    const result = layoutPipelineGraph(
      [{ id: 'g', nodes: [node('a')] }],
      [{ source: 'a', target: 'missing' }],
      [
        { source: 'g', target: 'g' },
        { source: 'missing', target: 'g' },
      ],
    );
    expect([...result.groups.keys()]).toEqual(['g']);
  });
});

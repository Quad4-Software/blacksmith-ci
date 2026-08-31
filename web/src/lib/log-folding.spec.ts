import { describe, expect, it } from 'vitest';
import type { FoldableLogLine } from './log-folding';
import { commandTrace, findSectionId, groupLogLines } from './log-folding';

function line(index: number, opts: Partial<FoldableLogLine> = {}): FoldableLogLine {
  return { index, ...opts };
}

function command(index: number, cmd: string, time?: number): FoldableLogLine {
  return { index, time, command: cmd };
}

describe('commandTrace', () => {
  it('extracts the command a stdout trace announced', () => {
    expect(commandTrace('+ go build ./...', false)).toBe('go build ./...');
  });

  it('ignores the same shape on stderr, where `set -x` traces land', () => {
    expect(commandTrace('+ go build ./...', true)).toBeUndefined();
  });

  it('ignores a bare plus without a command', () => {
    expect(commandTrace('+ ', false)).toBeUndefined();
    expect(commandTrace('+', false)).toBeUndefined();
  });

  it('ignores plus signs that are not a trace prefix', () => {
    expect(commandTrace('  + indented', false)).toBeUndefined();
    expect(commandTrace('++ nested trace', false)).toBeUndefined();
    expect(commandTrace('1 + 1 = 2', false)).toBeUndefined();
  });

  it('tolerates a trailing newline or carriage return', () => {
    expect(commandTrace('+ echo hi\r\n', false)).toBe('echo hi');
  });
});

describe('groupLogLines', () => {
  it('returns plain line rows when no command was traced', () => {
    const lines = [line(0), line(1)];
    const rows = groupLogLines(lines, new Set());

    expect(rows).toHaveLength(2);
    expect(rows.every((row) => row.kind === 'line')).toBe(true);
  });

  it('keeps lines before the first command ungrouped', () => {
    const lines = [line(0), command(1, 'make'), line(2)];
    const rows = groupLogLines(lines, new Set());

    expect(rows.map((row) => row.kind)).toEqual(['line', 'header', 'line']);
    expect(rows[0]).toMatchObject({ kind: 'line', sectionId: null });
  });

  it('replaces the traced command line with its section header', () => {
    const rows = groupLogLines([command(0, 'make'), line(1)], new Set());

    expect(rows[0]).toMatchObject({ kind: 'header', id: 0, command: 'make' });
    expect(rows[1]).toMatchObject({ kind: 'line', sectionId: 0 });
  });

  it('starts a new section at every traced command', () => {
    const lines = [command(0, 'a'), line(1), command(2, 'b'), line(3)];
    const rows = groupLogLines(lines, new Set());

    expect(rows.filter((row) => row.kind === 'header').map((row) => row.command)).toEqual(['a', 'b']);
  });

  it('omits the lines of a collapsed section but keeps its header', () => {
    const lines = [command(0, 'a'), line(1), line(2), line(3), line(4), command(5, 'b'), line(6)];
    const rows = groupLogLines(lines, new Set([0]));

    expect(rows.map((row) => row.kind)).toEqual(['header', 'header', 'line']);
    expect(rows.filter((row) => row.kind === 'line').map((row) => row.line.index)).toEqual([6]);
  });

  it('reports how many lines a section hides', () => {
    const rows = groupLogLines([command(0, 'a'), line(1), line(2), line(3), line(4)], new Set([0]));

    expect(rows[0]).toMatchObject({ kind: 'header', lineCount: 4, foldable: true });
  });

  it('keeps short sections expanded and marks them as non-foldable', () => {
    const rows = groupLogLines([command(0, 'a'), line(1), line(2), line(3)], new Set([0]));

    expect(rows[0]).toMatchObject({ kind: 'header', lineCount: 3, foldable: false });
    expect(rows.filter((row) => row.kind === 'line').map((row) => row.line.index)).toEqual([1, 2, 3]);
  });

  it('measures a section up to the start of the next one', () => {
    const lines = [command(0, 'a', 2), line(1, { time: 3 }), command(2, 'b', 10), line(3, { time: 25 })];
    const rows = groupLogLines(lines, new Set()).filter((row) => row.kind === 'header');

    expect(rows[0].duration).toBe(8);
    expect(rows[1].duration).toBe(15);
  });

  it('carries the last known time forward across blanked duplicate timestamps', () => {
    // flushLogs drops a timestamp when it repeats the previous line's.
    const lines = [command(0, 'a', 4), line(1), line(2), command(3, 'b', 9)];
    const rows = groupLogLines(lines, new Set()).filter((row) => row.kind === 'header');

    expect(rows[0].duration).toBe(5);
  });

  it('leaves the duration unset while no timing is known yet', () => {
    const rows = groupLogLines([command(0, 'a'), line(1)], new Set()).filter((row) => row.kind === 'header');

    expect(rows[0].duration).toBeUndefined();
  });

  it('never reports a negative duration when times arrive out of order', () => {
    const lines = [command(0, 'a', 9), command(1, 'b', 4)];
    const rows = groupLogLines(lines, new Set()).filter((row) => row.kind === 'header');

    expect(rows[0].duration).toBe(0);
  });
});

describe('findSectionId', () => {
  const lines = [line(0), command(1, 'a'), line(2), command(3, 'b'), line(4)];

  it('finds the section a line belongs to', () => {
    expect(findSectionId(lines, 2)).toBe(1);
    expect(findSectionId(lines, 4)).toBe(3);
  });

  it('treats a header as part of its own section', () => {
    expect(findSectionId(lines, 3)).toBe(3);
  });

  it('returns null for lines ahead of the first command', () => {
    expect(findSectionId(lines, 0)).toBeNull();
  });

  it('returns null for an unknown line', () => {
    expect(findSectionId(lines, 99)).toBeNull();
  });
});

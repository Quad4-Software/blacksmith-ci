import { describe, expect, it } from 'vitest';
import type { IssueLogLine } from './log-issues';
import { buildIssueIndex, classifyLogLine } from './log-issues';

function line(index: number, opts: Partial<IssueLogLine> = {}): IssueLogLine {
  return { index, number: index + 1, type: null, ...opts };
}

function issue(index: number, type: 'error' | 'warning', text: string): IssueLogLine {
  return { index, number: index + 1, type, text };
}

describe('classifyLogLine', () => {
  it('marks a line announcing a warning', () => {
    expect(classifyLogLine('WARN System dependency check skipped', false)).toBe('warning');
  });

  it('marks a stderr line announcing an error', () => {
    expect(classifyLogLine('Error: failed to build xml2', true)).toBe('error');
  });

  it('leaves an error-shaped line on stdout unmarked', () => {
    expect(classifyLogLine('Error: failed to build xml2', false)).toBeNull();
  });

  it('prefers warning over error when a line carries both words', () => {
    expect(classifyLogLine('warning: treating this error as non-fatal', true)).toBe('warning');
  });

  it('leaves a success line on stderr unmarked', () => {
    expect(classifyLogLine('✓ build complete, 0 failed', true)).toBeNull();
  });

  it('leaves an informational stderr line unmarked', () => {
    expect(classifyLogLine('> 68 packages resolved', true)).toBeNull();
  });
});

describe('buildIssueIndex', () => {
  it('returns nothing when no line was classified', () => {
    expect(buildIssueIndex([line(0, { text: 'hello' }), line(1, { text: 'world' })])).toEqual([]);
  });

  it('lists a classified line with its severity, number and text', () => {
    const issues = buildIssueIndex([line(0, { text: 'ok' }), issue(1, 'error', 'boom')]);

    expect(issues).toEqual([{ severity: 'error', index: 1, number: 2, text: 'boom', extraLines: 0 }]);
  });

  it('collapses a run of adjacent lines of the same severity into one entry', () => {
    const issues = buildIssueIndex([issue(0, 'warning', 'first'), issue(1, 'warning', 'second')]);

    expect(issues).toEqual([{ severity: 'warning', index: 0, number: 1, text: 'first', extraLines: 1 }]);
  });

  it('starts a new entry where the severity changes', () => {
    const issues = buildIssueIndex([issue(0, 'warning', 'warned'), issue(1, 'error', 'failed')]);

    expect(issues.map((i) => [i.severity, i.number, i.extraLines])).toEqual([
      ['warning', 1, 0],
      ['error', 2, 0],
    ]);
  });

  it('does not join lines of the same severity separated by an unclassified line', () => {
    const issues = buildIssueIndex([
      issue(0, 'error', 'first'),
      line(1, { text: 'noise' }),
      issue(2, 'error', 'second'),
    ]);

    expect(issues.map((i) => i.number)).toEqual([1, 3]);
  });

  it('names the command section a line belongs to', () => {
    const issues = buildIssueIndex([
      line(0, { text: '+ go build ./...', command: 'go build ./...' }),
      issue(1, 'error', 'boom'),
    ]);

    expect(issues[0].section).toBe('go build ./...');
  });

  it('leaves lines before the first command without a section', () => {
    const issues = buildIssueIndex([issue(0, 'warning', 'clone was shallow')]);

    expect(issues[0].section).toBeUndefined();
  });

  it('never lists a command trace, which renders as a header rather than a tinted line', () => {
    const issues = buildIssueIndex([
      line(0, { type: 'error', text: '+ docker pull acme/error-parser', command: 'docker pull acme/error-parser' }),
    ]);

    expect(issues).toEqual([]);
  });

  it('strips the ANSI markup the log view renders, so the preview stays plain text', () => {
    const issues = buildIssueIndex([issue(0, 'error', '<span class="ansi-red-fg">Error:</span> boom\n')]);

    expect(issues[0].text).toBe('Error: boom');
  });

  it('skips a classified line that holds nothing but markup', () => {
    expect(buildIssueIndex([issue(0, 'warning', '<span class="ansi-red-fg"></span>\n')])).toEqual([]);
  });

  it('counts a run from its first line even when later lines carry the longer text', () => {
    const issues = buildIssueIndex([
      issue(0, 'warning', 'WARN skipped'),
      issue(1, 'warning', "Posit's sysreqs API doesn't serve this distribution"),
      issue(2, 'warning', 'Packages may fail to compile'),
    ]);

    expect(issues).toEqual([{ severity: 'warning', index: 0, number: 1, text: 'WARN skipped', extraLines: 2 }]);
  });
});

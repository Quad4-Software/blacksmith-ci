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
  it('ignores command traces emitted by Crow on stdout', () => {
    expect(classifyLogLine('+ echo warning', false)).toBeNull();
  });

  it.each(['"warning"', '> warning', '+ warning'])(
    'does not interpret quoted text or console prompts on stderr: %s',
    (text) => {
      expect(classifyLogLine(text, true)).toBe('warning');
    },
  );

  it.each([
    '0 errors ✔ | 0 warnings ✔ | 0 notes ✔',
    '0 warnings',
    '\x1B[32m0\x1B[0m errors ✔ | \x1B[32m0\x1B[0m warnings ✔',
    '0 errors, 0 failures, 0 failed',
  ])('does not classify zero counts: %s', (text) => {
    expect(classifyLogLine(text, false)).toBeNull();
    expect(classifyLogLine(text, true)).toBeNull();
  });

  it.each([
    '10 warnings',
    '0 errors | 2 warnings',
    '0 warnings, Warning: unable to access index for repository',
    'Warning: unable to access index for repository',
    '* checking package dependencies ...Warning: unable to access index for repository',
    'Warning: argument must be "warning"',
  ])('preserves real warnings: %s', (text) => {
    expect(classifyLogLine(text, true)).toBe('warning');
  });

  it('preserves an error alongside a zero warning count', () => {
    expect(classifyLogLine('Error: compilation failed, 0 warnings', true)).toBe('error');
  });

  it('marks a line announcing a warning', () => {
    expect(classifyLogLine('WARN System dependency check skipped', false)).toBe('warning');
  });

  it('marks a stderr line announcing an error', () => {
    expect(classifyLogLine('Error: failed to build xml2', true)).toBe('error');
  });

  it.each([
    '#34 14.43 .svelte-kit/output/server/chunks/error-banner.js 0.87 kB | gzip: 0.50 kB',
    'compiling src/errors/handler.go',
    'copying assets/warning-icon.svg',
    'loaded C:\\app\\fatal\\dump.log',
    'tests/test_exception.py::test_parse PASSED',
    'npm http fetch GET 200 https://registry.npmjs.org/@sentry/error-utils 12ms',
    'Downloading error_stack_parser-2.1.4.tar.gz (10 kB)',
    'go: downloading github.com/pkg/errors v0.9.1',
    'Compiling panic-halt v0.2.0',
    'at java.lang.NullPointerException.<init>(NullPointerException.java:59)',
  ])('ignores keywords that are part of a file, module or package name: %s', (text) => {
    expect(classifyLogLine(text, true)).toBeNull();
  });

  it.each([
    'Build error.',
    'panic: runtime error: index out of range',
    'error: cannot open /tmp/error.log',
    'error[E0308]: mismatched types',
    'time=2026-10-11T20:00:00Z level=error msg="connection refused"',
    'npm ERR! code ELIFECYCLE ... failed',
    'FAILED tests/test_parse.py::test_empty',
  ])('still marks a standalone keyword: %s', (text) => {
    expect(classifyLogLine(text, true)).toBe('error');
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

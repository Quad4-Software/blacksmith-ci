/**
 * Classifies step log lines as warnings or errors and indexes them.
 *
 * The log view tints classified lines in place, which only helps once you have
 * already scrolled to them. The index turns the same classification into a list
 * you can jump from, so a warning buried at line 3800 is one click away.
 *
 * Classification is content based rather than stream based: plenty of tools
 * write progress and success notices to stderr, so treating every stderr line
 * as an error would tint most of a healthy log.
 */

const WARNING_PATTERNS_REGEX = /\bwarn(?:ing)?s?\b|\[warn(?:ing)?\]/i;
const SUCCESS_PATTERNS_REGEX =
  /^\s*[v✓✔☑⋅·•›>+⟶→]\s|done\b|success|complete[ds]?\b|installed\b|built\b|packaged\b|updated\b|finished\b|passed\b|ok\b|\bgot\b/i;
const INFO_PATTERNS_REGEX = /^\s*[i>+\-*•·]\s|^\s*\d+\s*(?:pkg|packages?|deps?)/i;
const ERROR_PATTERNS_REGEX =
  /\berror\b|\bfailed\b|\bfailure\b|\bfatal\b|\bcritical\b|\bexception\b|\bpanic\b|\baborted?\b/i;
const HTML_TAG_REGEX = /<[^>]*>/g;

export type LogIssueSeverity = 'error' | 'warning';

export interface IssueLogLine {
  index: number;
  /** 1-based position in the step's log, used as the gutter number and anchor. */
  number: number;
  /** Rendered line, so ANSI markup rather than the raw text the agent sent. */
  text?: string;
  type: LogIssueSeverity | null;
  /** Set when this line is the trace of a command, to the command it traced. */
  command?: string;
}

export interface LogIssue {
  severity: LogIssueSeverity;
  index: number;
  /** Line number to jump to: the first line of the run. */
  number: number;
  /** Plain-text preview, taken from the first line of the run. */
  text: string;
  /** Command whose section the run sits in, unset for lines before the first. */
  section?: string;
  /** Lines folded into this entry beyond the first, so 0 for a lone line. */
  extraLines: number;
}

/**
 * Classifies a log line for highlighting, or returns null to leave it plain.
 */
export function classifyLogLine(text: string, isStderr: boolean): LogIssueSeverity | null {
  // Checked first so a line that hedges an error as a warning stays a warning.
  if (WARNING_PATTERNS_REGEX.test(text)) {
    return 'warning';
  }

  // Checkmarks, completion notices and progress counters are routinely written
  // to stderr; none of them are errors however error-shaped their wording.
  if (SUCCESS_PATTERNS_REGEX.test(text) || INFO_PATTERNS_REGEX.test(text)) {
    return null;
  }

  if (isStderr && ERROR_PATTERNS_REGEX.test(text)) {
    return 'error';
  }

  return null;
}

/**
 * Collapses the classified lines of a step into a jumpable list of issues.
 *
 * A tool that reports one problem over several lines would otherwise produce an
 * entry per line, so an unbroken run of one severity becomes a single entry
 * anchored at its first line. Command traces never appear: the log view renders
 * them as section headers, which carry no highlight to jump to.
 */
export function buildIssueIndex(lines: readonly IssueLogLine[]): LogIssue[] {
  const issues: LogIssue[] = [];
  let section: string | undefined;
  let run: LogIssue | null = null;

  for (const line of lines) {
    if (line.command !== undefined) {
      section = line.command;
      run = null;
      continue;
    }

    if (line.type === null) {
      run = null;
      continue;
    }

    if (run !== null && run.severity === line.type) {
      run.extraLines += 1;
      continue;
    }

    const text = plainText(line.text);
    if (text.length === 0) {
      run = null;
      continue;
    }

    run = { severity: line.type, index: line.index, number: line.number, text, section, extraLines: 0 };
    issues.push(run);
  }

  return issues;
}

/** Strips the markup the log view renders so a preview reads as plain text. */
function plainText(text?: string): string {
  return (text ?? '').replace(HTML_TAG_REGEX, '').trim();
}

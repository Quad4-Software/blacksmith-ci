/**
 * Groups step logs into foldable sections, one per traced command.
 *
 * Every backend prefixes each command it runs with an echoed `+ <command>`
 * line on stdout before running it, so the log stream already carries the
 * boundaries between commands. This module turns that flat stream into a
 * list of rows the log view can render, without needing any extra markers
 * from the agent — which means it also works on logs stored long ago.
 */

/** A trace line is `+ ` followed by the command, and nothing before it. */
const COMMAND_TRACE_REGEX = /^\+ (?!\s)(.+)$/;
const TRAILING_NEWLINE_REGEX = /\r?\n$/;

export interface FoldableLogLine {
  index: number;
  time?: number;
  /** Set when this line is the trace of a command, to the command it traced. */
  command?: string;
}

export type LogRow<T extends FoldableLogLine> =
  | { kind: 'header'; id: number; command: string; time?: number; duration?: number; lineCount: number }
  | { kind: 'line'; line: T; sectionId: number | null };

/**
 * Returns the command a raw log line traced, or undefined if it traced none.
 *
 * Only stdout qualifies: `set -x` writes its own traces to stderr, and folding
 * on those would split a single command into a section per subshell line.
 */
export function commandTrace(text: string, isStderr: boolean): string | undefined {
  if (isStderr) {
    return undefined;
  }
  return COMMAND_TRACE_REGEX.exec(text.replace(TRAILING_NEWLINE_REGEX, ''))?.[1];
}

/**
 * Flattens lines into renderable rows, dropping the lines of collapsed sections.
 *
 * Lines emitted before the first command — clone output, plugin banners — stay
 * ungrouped so they are never hidden behind a header.
 */
export function groupLogLines<T extends FoldableLogLine>(lines: T[], collapsed: ReadonlySet<number>): LogRow<T>[] {
  const rows: LogRow<T>[] = [];
  const headers: Extract<LogRow<T>, { kind: 'header' }>[] = [];
  let sectionId: number | null = null;
  let lastTime: number | undefined;

  for (const line of lines) {
    lastTime = line.time ?? lastTime;

    if (line.command !== undefined) {
      // The previous section ran until this one started.
      setDuration(headers.at(-1), lastTime);
      sectionId = line.index;
      const header = {
        kind: 'header' as const,
        id: line.index,
        command: line.command,
        time: lastTime,
        duration: undefined as number | undefined,
        lineCount: 0,
      };
      headers.push(header);
      rows.push(header);
    }

    if (sectionId !== null) {
      headers.at(-1)!.lineCount += 1;
      if (collapsed.has(sectionId)) {
        continue;
      }
    }

    rows.push({ kind: 'line', line, sectionId });
  }

  // The last section has no successor, so the newest line is all we know.
  setDuration(headers.at(-1), lastTime);

  return rows;
}

function setDuration<T extends FoldableLogLine>(
  header: Extract<LogRow<T>, { kind: 'header' }> | undefined,
  end?: number,
) {
  if (header === undefined || header.time === undefined || end === undefined) {
    return;
  }
  header.duration = Math.max(0, end - header.time);
}

/** Returns the id of the section a line belongs to, or null if it is ungrouped. */
export function findSectionId<T extends FoldableLogLine>(lines: T[], lineIndex: number): number | null {
  let sectionId: number | null = null;

  for (const line of lines) {
    if (line.command !== undefined) {
      sectionId = line.index;
    }
    if (line.index === lineIndex) {
      return sectionId;
    }
  }

  return null;
}

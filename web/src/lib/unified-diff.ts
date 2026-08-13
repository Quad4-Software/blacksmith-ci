export interface DiffFile {
  path: string;
  lines: DiffLine[];
}

export interface DiffLine {
  content: string;
  type: 'addition' | 'deletion' | 'hunk' | 'metadata' | 'context';
}

const DIFF_HEADER = 'diff --git ';

export function parseUnifiedDiff(diff: string): DiffFile[] {
  return diff
    .split(`\n${DIFF_HEADER}`)
    .filter((section) => section.trim() !== '')
    .map((section, index) => {
      const normalized = index === 0 && section.startsWith(DIFF_HEADER) ? section : `${DIFF_HEADER}${section}`;
      const lines = normalized.split('\n');
      const newPath = lines.find((line) => line.startsWith('+++ '))?.slice(4);
      const headerPath = lines[0]?.match(/^diff --git .* b\/(.+)$/)?.[1];
      const path =
        newPath !== undefined && newPath !== '' && newPath !== '/dev/null'
          ? newPath.replace(/^b\//, '')
          : (headerPath ?? 'unknown');

      return {
        path,
        lines: lines.map((content) => ({ content, type: lineType(content) })),
      };
    });
}

function lineType(content: string): DiffLine['type'] {
  if (content.startsWith('@@')) return 'hunk';
  if (content.startsWith('+') && !content.startsWith('+++')) return 'addition';
  if (content.startsWith('-') && !content.startsWith('---')) return 'deletion';
  if (
    content.startsWith('diff ') ||
    content.startsWith('index ') ||
    content.startsWith('--- ') ||
    content.startsWith('+++ ')
  ) {
    return 'metadata';
  }
  return 'context';
}

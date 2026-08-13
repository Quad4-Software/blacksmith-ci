import { describe, expect, it } from 'vitest';
import { parseUnifiedDiff } from './unified-diff';

describe('parseUnifiedDiff', () => {
  it('splits a unified diff into files and classifies lines', () => {
    const diff = `diff --git a/one.txt b/one.txt
index 111..222 100644
--- a/one.txt
+++ b/one.txt
@@ -1 +1 @@
-old
+new
diff --git a/src/two.ts b/src/two.ts
new file mode 100644
--- /dev/null
+++ b/src/two.ts
@@ -0,0 +1 @@
+export {};
`;

    const files = parseUnifiedDiff(diff);

    expect(files.map((file) => file.path)).toEqual(['one.txt', 'src/two.ts']);
    expect(files[0]?.lines.some((line) => line.type === 'deletion' && line.content === '-old')).toBe(true);
    expect(files[0]?.lines.some((line) => line.type === 'addition' && line.content === '+new')).toBe(true);
    expect(files[1]?.lines.some((line) => line.type === 'hunk')).toBe(true);
  });

  it('returns no files for an empty diff', () => {
    expect(parseUnifiedDiff('')).toEqual([]);
  });
});

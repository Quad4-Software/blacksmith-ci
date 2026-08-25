import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import process from 'node:process';
import { describe, expect, it } from 'vitest';

// Resolved from the working directory rather than `import.meta.url`, which
// vitest rewrites to a non-file URL.
const stylePath = ['src/style.css', 'web/src/style.css']
  .map((candidate) => resolve(process.cwd(), candidate))
  .find((candidate) => existsSync(candidate));

if (stylePath === undefined) throw new Error('could not locate style.css');

const styleCss = readFileSync(stylePath, 'utf8');

const COMMENT_RE = /\/\*[\s\S]*?\*\//g;

// Strips the body of every `@layer` block so what remains is only the rules that
// sit outside any cascade layer. Comments are removed first so an `@layer`
// mentioned in prose cannot be mistaken for a real block.
function unlayeredCss(source: string): string {
  const css = source.replace(COMMENT_RE, '');
  let out = '';
  let index = 0;

  const layerAt = (from: number) => css.indexOf('@layer', from);

  for (let start = layerAt(0); start !== -1; start = layerAt(index)) {
    out += css.slice(index, start);

    const blockStart = css.indexOf('{', start);
    if (blockStart === -1) break;

    let depth = 0;
    let cursor = blockStart;
    for (; cursor < css.length; cursor += 1) {
      if (css[cursor] === '{') depth += 1;
      else if (css[cursor] === '}') {
        depth -= 1;
        if (depth === 0) break;
      }
    }

    index = cursor + 1;
  }

  return out + css.slice(index);
}

describe('style.css', () => {
  // Unlayered rules beat every layered rule regardless of specificity, and
  // Tailwind emits its utilities into `@layer utilities`. An unlayered global
  // reset here silently kills whole utility families app-wide: a `* { min-width: 0 }`
  // outside a layer once disabled every `min-w-*` class in the app, which
  // collapsed the manual-pipeline branch picker to its content width.
  it('keeps the global `*` reset inside a cascade layer', () => {
    const unlayered = unlayeredCss(styleCss);

    expect(unlayered).not.toMatch(/(^|[},;])\s*\*\s*\{/);
  });

  it('still applies the `min-width` reset that flex children rely on', () => {
    expect(styleCss).toMatch(/@layer\s+base\s*\{[\s\S]*\*\s*\{[\s\S]*min-width:\s*0[\s\S]*\}/);
  });
});

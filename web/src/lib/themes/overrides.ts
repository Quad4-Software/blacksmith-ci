/**
 * User-provided theme overrides
 *
 * Users can paste CSS custom properties in the shadcn/tweakcn format to
 * override individual colors of the selected light and dark themes:
 *
 *   :root { --primary: oklch(0.6 0.2 40); }
 *   .dark { --primary: oklch(0.7 0.2 40); }
 *
 * Only the theme variables listed in CSS_VAR_MAP are applied; everything else
 * is reported back so the user can see what was ignored.
 */

import { CSS_VAR_KEYS, CSS_VAR_MAP } from './css-vars';
import type { ThemeColorVariables, ThemeDefinition } from './types';

export type ThemeMode = 'light' | 'dark';

export interface ThemeOverrides {
  light: Partial<ThemeColorVariables>;
  dark: Partial<ThemeColorVariables>;
}

export interface ThemeOverridesParseResult {
  overrides: ThemeOverrides;
  /** Number of variables that will be applied, across both modes */
  applied: number;
  /** Selectors that are neither a light nor a dark block */
  ignoredSelectors: string[];
  /** Properties that are not a known theme variable */
  ignoredProperties: string[];
  /** Theme variables whose value is not a valid color */
  invalidValues: string[];
}

const COMMENT_RE = /\/\*[\s\S]*?\*\//g;
// Innermost `selector { declarations }` blocks, so wrappers such as
// `@layer base { ... }` are skipped and their contents still parsed.
const RULE_RE = /([^{}]*)\{([^{}]*)\}/g;
const IMPORTANT_RE = /\s*!important\s*$/i;
const WHITESPACE_RE = /\s+/g;
const QUOTE_RE = /["']/g;

const DARK_SELECTORS = new Set(['.dark', ':root.dark', 'html.dark', '[data-theme=dark]', ':root[data-theme=dark]']);
const LIGHT_SELECTORS = new Set([
  ':root',
  'html',
  '.light',
  ':root.light',
  'html.light',
  '[data-theme=light]',
  ':root[data-theme=light]',
]);

function selectorMode(selector: string): ThemeMode | null {
  const normalized = selector.replace(WHITESPACE_RE, '').replace(QUOTE_RE, '');
  if (normalized === '') {
    return null;
  }
  const parts = normalized.split(',');
  if (parts.every((part) => DARK_SELECTORS.has(part))) {
    return 'dark';
  }
  if (parts.every((part) => LIGHT_SELECTORS.has(part))) {
    return 'light';
  }
  return null;
}

/** Whether the browser accepts value as a CSS color (always true without CSSOM) */
export function isValidColor(value: string): boolean {
  if (typeof CSS === 'undefined' || typeof CSS.supports !== 'function') {
    return true;
  }
  return CSS.supports('color', value);
}

function pushUnique(list: string[], value: string) {
  if (!list.includes(value)) {
    list.push(value);
  }
}

/**
 * Parse user-provided CSS into per-mode theme overrides.
 *
 * Declarations in a `:root` (or `.light`) block override the light theme,
 * declarations in a `.dark` block override the dark theme.
 */
export function parseThemeOverrides(
  css: string,
  validColor: (value: string) => boolean = isValidColor,
): ThemeOverridesParseResult {
  const result: ThemeOverridesParseResult = {
    overrides: { light: {}, dark: {} },
    applied: 0,
    ignoredSelectors: [],
    ignoredProperties: [],
    invalidValues: [],
  };

  const source = css.replace(COMMENT_RE, '');
  for (const [, rawSelector, body] of source.matchAll(RULE_RE)) {
    const selector = rawSelector.trim();
    const mode = selectorMode(selector);
    if (mode === null) {
      if (selector !== '') {
        pushUnique(result.ignoredSelectors, selector);
      }
      continue;
    }

    for (const declaration of body.split(';')) {
      const colon = declaration.indexOf(':');
      if (colon === -1) {
        continue;
      }
      const property = declaration.slice(0, colon).trim();
      const value = declaration
        .slice(colon + 1)
        .replace(IMPORTANT_RE, '')
        .trim();
      if (property === '' || value === '') {
        continue;
      }

      const key = CSS_VAR_KEYS.get(property);
      if (key === undefined) {
        pushUnique(result.ignoredProperties, property);
        continue;
      }
      if (!validColor(value)) {
        pushUnique(result.invalidValues, `${property}: ${value}`);
        continue;
      }

      if (result.overrides[mode][key] === undefined) {
        result.applied++;
      }
      result.overrides[mode][key] = value;
    }
  }

  return result;
}

function colorsToCss(selector: string, colors: ThemeColorVariables): string {
  const lines = Object.entries(CSS_VAR_MAP).map(
    ([key, cssVar]) => `  ${cssVar}: ${colors[key as keyof ThemeColorVariables]};`,
  );
  return `${selector} {\n${lines.join('\n')}\n}`;
}

/**
 * Serialize a light and a dark theme as override CSS, as a starting point for
 * editing.
 */
export function themesToCss(light: ThemeDefinition, dark: ThemeDefinition): string {
  return `${colorsToCss(':root', light.light)}\n\n${colorsToCss('.dark', dark.dark)}\n`;
}

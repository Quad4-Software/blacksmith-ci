import { describe, expect, it } from 'vitest';

import { blacksmithTheme } from './blacksmith';
import { parseThemeOverrides, themesToCss } from './overrides';

const anyColor = () => true;

describe('parseThemeOverrides', () => {
  it('maps :root to light and .dark to dark', () => {
    const result = parseThemeOverrides(
      `:root { --primary: #ff0000; --sidebar-border: #eeeeee; }
        .dark { --primary: #00ff00; }`,
      anyColor,
    );

    expect(result.overrides.light).toEqual({ primary: '#ff0000', sidebarBorder: '#eeeeee' });
    expect(result.overrides.dark).toEqual({ primary: '#00ff00' });
    expect(result.applied).toBe(3);
    expect(result.ignoredSelectors).toEqual([]);
    expect(result.ignoredProperties).toEqual([]);
  });

  it('returns empty overrides for empty input', () => {
    const result = parseThemeOverrides('', anyColor);

    expect(result.overrides).toEqual({ light: {}, dark: {} });
    expect(result.applied).toBe(0);
  });

  it('accepts tweakcn output wrapped in @layer with comments and !important', () => {
    const result = parseThemeOverrides(
      `/* generated */
        @layer base {
          :root { --background: oklch(1 0 0) !important; /* page */ }
          [data-theme="dark"] { --background: oklch(0.2 0 0); }
        }
        @theme inline { --color-background: var(--background); }`,
      anyColor,
    );

    expect(result.overrides.light).toEqual({ background: 'oklch(1 0 0)' });
    expect(result.overrides.dark).toEqual({ background: 'oklch(0.2 0 0)' });
    expect(result.ignoredSelectors).toEqual(['@theme inline']);
  });

  it('reports unknown properties and selectors', () => {
    const result = parseThemeOverrides(
      `:root { --radius: 0.5rem; --font-sans: Inter; color: red; --primary: red; }
        .sidebar { --primary: blue; }`,
      anyColor,
    );

    expect(result.overrides.light).toEqual({ primary: 'red' });
    expect(result.ignoredProperties).toEqual(['--radius', '--font-sans', 'color']);
    expect(result.ignoredSelectors).toEqual(['.sidebar']);
  });

  it('skips values that are not valid colors', () => {
    const result = parseThemeOverrides(':root { --primary: not-a-color; --ring: #123456; }', (value) =>
      value.startsWith('#'),
    );

    expect(result.overrides.light).toEqual({ ring: '#123456' });
    expect(result.invalidValues).toEqual(['--primary: not-a-color']);
  });

  it('lets later declarations win without double counting', () => {
    const result = parseThemeOverrides(':root { --primary: red; } :root { --primary: blue; }', anyColor);

    expect(result.overrides.light).toEqual({ primary: 'blue' });
    expect(result.applied).toBe(1);
  });

  it('treats a selector list as one mode only when all parts agree', () => {
    const result = parseThemeOverrides(
      `:root, .light { --primary: red; }
        :root, .dark { --primary: blue; }`,
      anyColor,
    );

    expect(result.overrides.light).toEqual({ primary: 'red' });
    expect(result.overrides.dark).toEqual({});
    expect(result.ignoredSelectors).toEqual([':root, .dark']);
  });
});

describe('themesToCss', () => {
  it('round-trips through parseThemeOverrides', () => {
    const result = parseThemeOverrides(themesToCss(blacksmithTheme, blacksmithTheme), anyColor);

    expect(result.overrides.light).toEqual(blacksmithTheme.light);
    expect(result.overrides.dark).toEqual(blacksmithTheme.dark);
    expect(result.ignoredProperties).toEqual([]);
  });
});

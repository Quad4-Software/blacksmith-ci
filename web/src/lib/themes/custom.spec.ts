import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import process from 'node:process';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { claudeTheme } from './claude';
import { blacksmithTheme } from './blacksmith';
import { CSS_VAR_MAP } from './css-vars';
import { mergeCustomThemes } from './custom';

const THEME_VARS_BLOCK_RE = /themeVars = map\[string\]bool\{([\s\S]*?)\n\t\}/;
const GO_MAP_KEY_RE = /"([a-z0-9-]+)": true/g;

const anyColor = () => true;
const bundled = [blacksmithTheme, claudeTheme];

describe('mergeCustomThemes', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns the bundled themes unchanged without custom themes', () => {
    expect(mergeCustomThemes(bundled, [], 'blacksmith', anyColor)).toEqual(bundled);
  });

  it('appends a new theme on top of the default theme', () => {
    const themes = mergeCustomThemes(
      bundled,
      [{ id: 'acme', name: 'ACME', light: { primary: '#7c3aed', 'sidebar-primary': '#6d28d9' } }],
      'blacksmith',
      anyColor,
    );

    expect(themes.map((theme) => theme.id)).toEqual(['blacksmith', 'claude', 'acme']);
    const acme = themes[2];
    expect(acme.name).toBe('ACME');
    expect(acme.custom).toBe(true);
    expect(acme.light).toEqual({ ...blacksmithTheme.light, primary: '#7c3aed', sidebarPrimary: '#6d28d9' });
    expect(acme.dark).toEqual(blacksmithTheme.dark);
  });

  it('inherits from the given base theme', () => {
    const [, , acme] = mergeCustomThemes(
      bundled,
      [{ id: 'acme', base: 'claude', dark: { primary: '#a78bfa' } }],
      'blacksmith',
      anyColor,
    );

    expect(acme.name).toBe('acme');
    expect(acme.light).toEqual(claudeTheme.light);
    expect(acme.dark).toEqual({ ...claudeTheme.dark, primary: '#a78bfa' });
  });

  it('replaces a bundled theme in place and inherits from it', () => {
    const themes = mergeCustomThemes(bundled, [{ id: 'blacksmith', light: { primary: '#7c3aed' } }], 'blacksmith', anyColor);

    expect(themes.map((theme) => theme.id)).toEqual(['blacksmith', 'claude']);
    expect(themes[0].name).toBe(blacksmithTheme.name);
    expect(themes[0].custom).toBe(true);
    expect(themes[0].light).toEqual({ ...blacksmithTheme.light, primary: '#7c3aed' });
  });

  it('falls back to the default theme for an unknown base', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});

    const [, , acme] = mergeCustomThemes(
      bundled,
      [{ id: 'acme', base: 'missing', light: { primary: 'red' } }],
      'blacksmith',
      anyColor,
    );

    expect(acme.dark).toEqual(blacksmithTheme.dark);
    expect(warn).toHaveBeenCalledOnce();
  });

  it('skips unknown variables silently and invalid colors with a warning', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});

    const [, , acme] = mergeCustomThemes(
      bundled,
      [{ id: 'acme', light: { 'font-sans': 'Inter', radius: '1rem', primary: 'nope', ring: '#123456' } }],
      'blacksmith',
      (value) => value.startsWith('#'),
    );

    expect(acme.light).toEqual({ ...blacksmithTheme.light, ring: '#123456' });
    expect(warn).toHaveBeenCalledOnce();
  });
});

// The server drops variables it does not know before passing themes to the UI,
// so its list has to match the variables the UI applies.
describe('server theme variables', () => {
  it('match CSS_VAR_MAP', () => {
    const themeGoPath = ['../server/theme/theme.go', 'server/theme/theme.go']
      .map((candidate) => resolve(process.cwd(), candidate))
      .find((candidate) => existsSync(candidate));
    if (themeGoPath === undefined) throw new Error('could not locate server/theme/theme.go');

    const source = readFileSync(themeGoPath, 'utf8');
    const block = THEME_VARS_BLOCK_RE.exec(source)?.[1] ?? '';
    const serverVars = [...block.matchAll(GO_MAP_KEY_RE)].map((match) => `--${match[1]}`);

    expect(serverVars.toSorted()).toEqual(Object.values(CSS_VAR_MAP).toSorted());
  });
});

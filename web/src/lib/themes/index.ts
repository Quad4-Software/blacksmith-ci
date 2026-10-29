/**
 * Theme Registry
 *
 * Central registry for all available themes.
 * To add a new theme:
 * 1. Create a theme file (e.g., my-theme.ts) implementing ThemeDefinition
 * 2. Import and add it to the BUNDLED_THEMES array below
 *
 * Instance admins can add or replace themes via CROW_CUSTOM_THEMES,
 * which are merged in from window.CROW_CUSTOM_THEMES.
 */

import type { ThemeDefinition, ThemeOption } from './types';
import { blacksmithTheme } from './blacksmith';
import { catppuccinTheme } from './catppuccin';
import { claudeTheme } from './claude';
import { mergeCustomThemes } from './custom';
import { doom64Theme } from './doom-64';
import { modernMinimalTheme } from './modern-minimal';
import { monoTheme } from './mono';
import { oceanBreezeTheme } from './ocean-breeze';
import { t3ChatTheme } from './t3-chat';
import { vercelTheme } from './vercel';

/** Default theme ID when no preference is stored */
export const DEFAULT_THEME_ID = 'blacksmith';

/** Themes shipped with Blacksmith */
const BUNDLED_THEMES: ThemeDefinition[] = [
  blacksmithTheme,
  catppuccinTheme,
  claudeTheme,
  doom64Theme,
  modernMinimalTheme,
  monoTheme,
  oceanBreezeTheme,
  t3ChatTheme,
  vercelTheme,
];

/** All available themes, including admin-provided ones */
export const THEMES: ThemeDefinition[] = mergeCustomThemes(
  BUNDLED_THEMES,
  globalThis.window?.CROW_CUSTOM_THEMES ?? [],
  DEFAULT_THEME_ID,
);

/** Theme lookup map for O(1) access */
export const THEME_MAP = new Map<string, ThemeDefinition>(THEMES.map((theme) => [theme.id, theme]));

/** Theme options for UI display (without full color data) */
export const THEME_OPTIONS: ThemeOption[] = THEMES.map(({ id, name, description, custom }) => ({
  id,
  name,
  description,
  custom,
}));

/**
 * Get a theme by ID, falling back to default if not found.
 * Throws if neither the requested theme nor the default exists (should never happen).
 */
export function getTheme(id: string): ThemeDefinition {
  const theme = THEME_MAP.get(id) ?? THEME_MAP.get(DEFAULT_THEME_ID);
  if (!theme) {
    throw new Error(`Theme not found: ${id}, and default theme '${DEFAULT_THEME_ID}' is missing`);
  }
  return theme;
}

export { CSS_VAR_MAP } from './css-vars';
export type { CustomThemeConfig } from './custom';
export { parseThemeOverrides, themesToCss } from './overrides';

// Re-export types
export type { ThemeMode, ThemeOverrides, ThemeOverridesParseResult } from './overrides';
export type { ThemeColorVariables, ThemeDefinition, ThemeOption } from './types';

/**
 * Admin-provided themes
 *
 * The server passes themes from CROW_CUSTOM_THEMES to the UI as
 * `window.CROW_CUSTOM_THEMES`. Each theme only needs the variables it changes;
 * the rest come from its base theme. A theme whose id matches a bundled theme
 * replaces that theme, so admins can also restyle the default.
 */

import { CSS_VAR_KEYS } from './css-vars';
import { isValidColor } from './overrides';
import type { ThemeColorVariables, ThemeDefinition } from './types';

/** A theme as delivered by the server; variable names have no leading `--` */
export interface CustomThemeConfig {
  id: string;
  name?: string;
  description?: string;
  base?: string;
  light?: Record<string, string>;
  dark?: Record<string, string>;
}

function toColorVariables(
  themeId: string,
  vars: Record<string, string> | undefined,
  validColor: (value: string) => boolean,
): Partial<ThemeColorVariables> {
  const colors: Partial<ThemeColorVariables> = {};
  for (const [name, value] of Object.entries(vars ?? {})) {
    const key = CSS_VAR_KEYS.get(`--${name}`);
    // Generators such as tweakcn also emit fonts, radius and shadows, which
    // themes do not cover, so unknown names are skipped silently.
    if (key === undefined) {
      continue;
    }
    if (!validColor(value)) {
      console.warn(`custom theme '${themeId}': ignoring invalid color for --${name}: ${value}`);
      continue;
    }
    colors[key] = value;
  }
  return colors;
}

/**
 * Merge admin-provided themes into the bundled ones.
 *
 * A custom theme with a bundled id replaces it in place and inherits from it by
 * default; other custom themes are appended and inherit from `defaultId`.
 */
export function mergeCustomThemes(
  bundled: ThemeDefinition[],
  custom: CustomThemeConfig[],
  defaultId: string,
  validColor: (value: string) => boolean = isValidColor,
): ThemeDefinition[] {
  const bundledMap = new Map(bundled.map((theme) => [theme.id, theme]));
  const result = [...bundled];

  for (const config of custom) {
    const replaced = bundledMap.get(config.id);
    const baseId = config.base ?? (replaced ? config.id : defaultId);
    let base = bundledMap.get(baseId);
    if (base === undefined) {
      console.warn(`custom theme '${config.id}': unknown base theme '${baseId}', using '${defaultId}'`);
      base = bundledMap.get(defaultId);
    }
    if (base === undefined) {
      continue;
    }

    const theme: ThemeDefinition = {
      id: config.id,
      name: config.name ?? replaced?.name ?? config.id,
      description: config.description ?? replaced?.description,
      light: { ...base.light, ...toColorVariables(config.id, config.light, validColor) },
      dark: { ...base.dark, ...toColorVariables(config.id, config.dark, validColor) },
      custom: true,
    };

    const index = result.findIndex((existing) => existing.id === config.id);
    if (index === -1) {
      result.push(theme);
    } else {
      result[index] = theme;
    }
  }

  return result;
}

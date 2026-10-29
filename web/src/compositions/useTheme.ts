import { useColorMode, useStorage } from '@vueuse/core';
import { computed, watch } from 'vue';

import type { ThemeColorVariables, ThemeOption } from '~/lib/themes';
import { CSS_VAR_MAP, DEFAULT_THEME_ID, getTheme, parseThemeOverrides, THEME_OPTIONS } from '~/lib/themes';

// Color mode (light/dark/auto)
const {
  store: storeTheme,
  state: resolvedTheme,
  system: systemTheme,
} = useColorMode({
  storageKey: 'crow:theme',
  initialValue: 'dark',
});

// Migration: check for old single theme preference and migrate to new split keys
const OLD_THEME_KEY = 'crow:theme-name';
const LIGHT_THEME_KEY = 'crow:theme-name-light';
const DARK_THEME_KEY = 'crow:theme-name-dark';

function parseStoredValue(value: string | null): string | null {
  if (value === null) {
    return null;
  }
  // Try to parse as JSON (useStorage stores values JSON-encoded)
  try {
    return JSON.parse(value) as string;
  } catch {
    // If not valid JSON, return as-is (plain string)
    return value;
  }
}

function getInitialTheme(key: string): string {
  // Check if new key already exists
  const newValue = parseStoredValue(localStorage.getItem(key));
  if (newValue !== null) {
    return newValue;
  }
  // Migrate from old single theme key if it exists
  const oldValue = parseStoredValue(localStorage.getItem(OLD_THEME_KEY));
  if (oldValue !== null) {
    return oldValue;
  }
  return DEFAULT_THEME_ID;
}

// Separate theme names for light and dark modes
const lightThemeName = useStorage<string>(LIGHT_THEME_KEY, getInitialTheme(LIGHT_THEME_KEY));
const darkThemeName = useStorage<string>(DARK_THEME_KEY, getInitialTheme(DARK_THEME_KEY));

// Font size options
export const FONT_SIZE_OPTIONS = [
  { value: 12, label: '12px' },
  { value: 13, label: '13px' },
  { value: 14, label: '14px (Default)' },
  { value: 15, label: '15px' },
  { value: 16, label: '16px' },
] as const;

// Base font size (default: 14px)
const fontSize = useStorage<number>('crow:font-size', 14);

// Current theme definition (computed from mode-specific theme name)
const currentTheme = computed(() => {
  const mode = resolvedTheme.value;
  const themeName = mode === 'light' ? lightThemeName.value : darkThemeName.value;
  return getTheme(themeName);
});

// User-provided CSS overriding individual theme variables (empty = no overrides)
const themeOverridesCss = useStorage<string>('crow:theme-overrides', '');

const themeOverrides = computed(() => parseThemeOverrides(themeOverridesCss.value).overrides);

// Available themes for UI
const availableThemes = computed<ThemeOption[]>(() => THEME_OPTIONS);

/**
 * Apply theme CSS variables to document root
 */
function applyThemeVariables(colors: ThemeColorVariables) {
  const root = document.documentElement;
  for (const [key, cssVar] of Object.entries(CSS_VAR_MAP)) {
    const value = colors[key as keyof ThemeColorVariables];
    if (value) {
      root.style.setProperty(cssVar, value);
    }
  }
}

/**
 * Apply font size to document root
 */
function applyFontSize(size: number) {
  document.documentElement.style.setProperty('font-size', `${size}px`);
}

/**
 * Update theme classes, attributes, and CSS variables
 */
function updateTheme() {
  const mode = resolvedTheme.value;
  const theme = currentTheme.value;

  // Update document classes and data-theme attribute
  if (mode === 'dark') {
    document.documentElement.classList.remove('light');
    document.documentElement.classList.add('dark');
    document.documentElement.setAttribute('data-theme', 'dark');
    document.querySelector('meta[name=theme-color]')?.setAttribute('content', '#0a0a0b');
  } else {
    document.documentElement.classList.remove('dark');
    document.documentElement.classList.add('light');
    document.documentElement.setAttribute('data-theme', 'light');
    document.querySelector('meta[name=theme-color]')?.setAttribute('content', '#fafafa');
  }

  // Apply theme-specific CSS variables, with the user's overrides on top
  const isDark = mode === 'dark';
  const colors = isDark ? theme.dark : theme.light;
  const overrides = isDark ? themeOverrides.value.dark : themeOverrides.value.light;
  applyThemeVariables({ ...colors, ...overrides });

  // Apply font size
  applyFontSize(fontSize.value);
}

// Watch for theme or mode changes
watch([storeTheme, systemTheme, lightThemeName, darkThemeName, themeOverrides], updateTheme, { immediate: true });

// Watch for font size changes
watch(fontSize, applyFontSize, { immediate: true });

/**
 * Theme composable for managing color scheme and theme selection
 */
export function useTheme() {
  return {
    /** Resolved theme mode ('light' | 'dark') */
    theme: resolvedTheme,

    /** Stored theme mode preference ('light' | 'dark' | 'auto') */
    storeTheme,

    /** Selected theme name for light mode */
    lightThemeName,

    /** Selected theme name for dark mode */
    darkThemeName,

    /** Currently active theme definition */
    currentTheme,

    /** User-provided CSS overriding variables of the selected themes */
    themeOverridesCss,

    /** Available themes for UI selection */
    availableThemes,

    /** Base font size in pixels */
    fontSize,
  };
}

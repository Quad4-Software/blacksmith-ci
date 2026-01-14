import { useColorMode, useStorage } from '@vueuse/core';
import { computed, watch } from 'vue';

import type { ThemeColorVariables, ThemeOption } from '~/lib/themes';
import { DEFAULT_THEME_ID, getTheme, THEME_OPTIONS } from '~/lib/themes';

// Color mode (light/dark/auto)
const {
  store: storeTheme,
  state: resolvedTheme,
  system: systemTheme,
} = useColorMode({
  storageKey: 'crow:theme',
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
  const mode = resolvedTheme.value as 'light' | 'dark';
  const themeName = mode === 'light' ? lightThemeName.value : darkThemeName.value;
  return getTheme(themeName);
});

// Available themes for UI
const availableThemes = computed<ThemeOption[]>(() => THEME_OPTIONS);

/**
 * Maps ThemeColorVariables keys to CSS custom property names
 */
const CSS_VAR_MAP: Record<keyof ThemeColorVariables, string> = {
  background: '--background',
  foreground: '--foreground',
  card: '--card',
  cardForeground: '--card-foreground',
  popover: '--popover',
  popoverForeground: '--popover-foreground',
  primary: '--primary',
  primaryForeground: '--primary-foreground',
  secondary: '--secondary',
  secondaryForeground: '--secondary-foreground',
  muted: '--muted',
  mutedForeground: '--muted-foreground',
  accent: '--accent',
  accentForeground: '--accent-foreground',
  destructive: '--destructive',
  destructiveForeground: '--destructive-foreground',
  border: '--border',
  input: '--input',
  ring: '--ring',
  chart1: '--chart-1',
  chart2: '--chart-2',
  chart3: '--chart-3',
  chart4: '--chart-4',
  chart5: '--chart-5',
  sidebar: '--sidebar',
  sidebarForeground: '--sidebar-foreground',
  sidebarPrimary: '--sidebar-primary',
  sidebarPrimaryForeground: '--sidebar-primary-foreground',
  sidebarAccent: '--sidebar-accent',
  sidebarAccentForeground: '--sidebar-accent-foreground',
  sidebarBorder: '--sidebar-border',
  sidebarRing: '--sidebar-ring',
};

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
  const mode = resolvedTheme.value as 'light' | 'dark';
  const theme = currentTheme.value;

  // Update document classes and data-theme attribute
  if (mode === 'dark') {
    document.documentElement.classList.remove('light');
    document.documentElement.classList.add('dark');
    document.documentElement.setAttribute('data-theme', 'dark');
    document.querySelector('meta[name=theme-color]')?.setAttribute('content', '#2A2E3A');
  } else {
    document.documentElement.classList.remove('dark');
    document.documentElement.classList.add('light');
    document.documentElement.setAttribute('data-theme', 'light');
    document.querySelector('meta[name=theme-color]')?.setAttribute('content', '#369943');
  }

  // Apply theme-specific CSS variables
  const colors = mode === 'dark' ? theme.dark : theme.light;
  applyThemeVariables(colors);

  // Apply font size
  applyFontSize(fontSize.value);
}

// Watch for theme or mode changes
watch([storeTheme, systemTheme, lightThemeName, darkThemeName], updateTheme, { immediate: true });

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

    /** Available themes for UI selection */
    availableThemes,

    /** Base font size in pixels */
    fontSize,
  };
}

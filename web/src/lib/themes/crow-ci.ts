import type { ThemeDefinition } from './types';

/**
 * Crow CI default theme
 *
 * Aligned with the codefloe-tide theme on codefloe.com so Crow and the forge
 * feel like one product: orange primary colors on cool slate (light) and
 * gray (dark) neutrals.
 *
 * Light: Burnt orange primary (#c2410c) on slate neutrals (#f1f5f9 page, white surfaces)
 * Dark: Bright orange primary (#fb923c) on gray neutrals (#111827 page, #1f2937 surfaces)
 */
export const crowCiTheme: ThemeDefinition = {
  id: 'crow-ci',
  name: 'Crow CI',
  description: 'Default Crow CI theme with orange accents, matching codefloe-tide',

  light: {
    // Backgrounds - slate-100 page with slate-900 text
    background: '#f1f5f9',
    foreground: '#0f172a',

    // Card - white surfaces on the slate page
    card: '#ffffff',
    cardForeground: '#0f172a',

    // Popover - same as card
    popover: '#ffffff',
    popoverForeground: '#0f172a',

    // Primary - burnt orange #c2410c
    primary: '#c2410c',
    primaryForeground: '#ffffff',

    // Secondary - slate-200
    secondary: '#e2e8f0',
    secondaryForeground: '#0f172a',

    // Muted - one step below the page so muted fills stay visible on it, slate-600 text
    muted: '#e9eef4',
    mutedForeground: '#475569',

    // Accent - neutral slate hover/active state
    accent: '#e2e8f0',
    accentForeground: '#0f172a',

    // Destructive - red-600
    destructive: '#dc2626',
    destructiveForeground: '#ffffff',

    // Borders - slate-300, inputs and outline buttons slate-400 so both stand out on the page
    border: '#cbd5e1',
    input: '#94a3b8',
    ring: '#c2410c',

    // Charts - codefloe-tide accent palette
    chart1: '#c2410c',
    chart2: '#0d9488',
    chart3: '#2563eb',
    chart4: '#ca8a04',
    chart5: '#9333ea',

    // Sidebar - white, like the codefloe navbar
    sidebar: '#ffffff',
    sidebarForeground: '#0f172a',
    sidebarPrimary: '#c2410c',
    sidebarPrimaryForeground: '#ffffff',
    sidebarAccent: '#e2e8f0',
    sidebarAccentForeground: '#0f172a',
    sidebarBorder: '#cbd5e1',
    sidebarRing: '#c2410c',
  },

  dark: {
    // Backgrounds - gray-900 page with gray-50 text
    background: '#111827',
    foreground: '#f9fafb',

    // Card - gray-800 surfaces
    card: '#1f2937',
    cardForeground: '#f9fafb',

    // Popover - same as card
    popover: '#1f2937',
    popoverForeground: '#f9fafb',

    // Primary - bright orange #fb923c
    primary: '#fb923c',
    primaryForeground: '#111827',

    // Secondary - gray-700
    secondary: '#374151',
    secondaryForeground: '#f9fafb',

    // Muted - codefloe box header, gray-400 text
    muted: '#273445',
    mutedForeground: '#9ca3af',

    // Accent - neutral gray hover/active state
    accent: '#374151',
    accentForeground: '#f9fafb',

    // Destructive - red-500, readable on the dark page
    destructive: '#ef4444',
    destructiveForeground: '#ffffff',

    // Borders - gray-600 so they stand out on the page; inputs share it, since their fill is a tint of it
    border: '#4b5563',
    input: '#4b5563',
    ring: '#fb923c',

    // Charts - codefloe-tide accent palette, lighter for dark mode
    chart1: '#fb923c',
    chart2: '#14b8a6',
    chart3: '#3b82f6',
    chart4: '#eab308',
    chart5: '#a855f7',

    // Sidebar - gray-800, like the codefloe navbar
    sidebar: '#1f2937',
    sidebarForeground: '#f9fafb',
    sidebarPrimary: '#fb923c',
    sidebarPrimaryForeground: '#111827',
    sidebarAccent: '#374151',
    sidebarAccentForeground: '#f9fafb',
    sidebarBorder: '#4b5563',
    sidebarRing: '#fb923c',
  },
};

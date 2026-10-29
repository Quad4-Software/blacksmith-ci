import type { ThemeDefinition } from './types';

/**
 * Blacksmith CI default theme
 *
 * Quad4 "void" identity: monochrome, dark-first, minimal.
 * Dark is the canonical variant, light is its paper inverse.
 *
 * Light: Void-on-paper primary (#0a0a0b) on zinc neutrals (#fafafa page, white surfaces)
 * Dark: Paper-on-void primary (#fafafa) on void neutrals (#0a0a0b page, #16161a surfaces)
 */
export const blacksmithTheme: ThemeDefinition = {
  id: 'blacksmith',
  name: 'Blacksmith',
  description: 'Default Blacksmith CI theme, Quad4 void monochrome',

  light: {
    // Backgrounds - paper page with void text
    background: '#fafafa',
    foreground: '#0a0a0b',

    // Card - white surfaces on the paper page
    card: '#ffffff',
    cardForeground: '#0a0a0b',

    // Popover - same as card
    popover: '#ffffff',
    popoverForeground: '#0a0a0b',

    // Primary - void on paper
    primary: '#0a0a0b',
    primaryForeground: '#fafafa',

    // Secondary - zinc-200
    secondary: '#e4e4e7',
    secondaryForeground: '#0a0a0b',

    // Muted - zinc-100 fill, zinc-600 text
    muted: '#f4f4f5',
    mutedForeground: '#52525b',

    // Accent - neutral zinc hover/active state
    accent: '#e4e4e7',
    accentForeground: '#0a0a0b',

    // Destructive - red-600
    destructive: '#dc2626',
    destructiveForeground: '#fafafa',

    // Borders - zinc-200, inputs zinc-300 so they stand out on the page
    border: '#e4e4e7',
    input: '#d4d4d8',
    ring: '#52525b',

    // Charts - monochrome zinc ramp
    chart1: '#27272a',
    chart2: '#3f3f46',
    chart3: '#52525b',
    chart4: '#71717a',
    chart5: '#a1a1aa',

    // Sidebar - zinc-100
    sidebar: '#f4f4f5',
    sidebarForeground: '#0a0a0b',
    sidebarPrimary: '#0a0a0b',
    sidebarPrimaryForeground: '#fafafa',
    sidebarAccent: '#e4e4e7',
    sidebarAccentForeground: '#0a0a0b',
    sidebarBorder: '#e4e4e7',
    sidebarRing: '#52525b',
  },

  dark: {
    // Backgrounds - void page with paper text
    background: '#0a0a0b',
    foreground: '#fafafa',

    // Card - raised surfaces
    card: '#16161a',
    cardForeground: '#fafafa',

    // Popover - same as card
    popover: '#16161a',
    popoverForeground: '#fafafa',

    // Primary - paper on void
    primary: '#fafafa',
    primaryForeground: '#0a0a0b',

    // Secondary - zinc-800
    secondary: '#27272a',
    secondaryForeground: '#fafafa',

    // Muted - raised surface, muted zinc text
    muted: '#16161a',
    mutedForeground: '#a1a1aa',

    // Accent - neutral zinc hover/active state
    accent: '#27272a',
    accentForeground: '#fafafa',

    // Destructive - red-500, readable on the void page
    destructive: '#ef4444',
    destructiveForeground: '#fafafa',

    // Borders - neutral zinc, inputs share it
    border: '#2a2a30',
    input: '#2a2a30',
    ring: '#52525b',

    // Charts - monochrome zinc ramp, lighter for dark mode
    chart1: '#d4d4d8',
    chart2: '#a1a1aa',
    chart3: '#71717a',
    chart4: '#52525b',
    chart5: '#3f3f46',

    // Sidebar - void, like the page
    sidebar: '#0a0a0b',
    sidebarForeground: '#fafafa',
    sidebarPrimary: '#fafafa',
    sidebarPrimaryForeground: '#0a0a0b',
    sidebarAccent: '#16161a',
    sidebarAccentForeground: '#fafafa',
    sidebarBorder: '#2a2a30',
    sidebarRing: '#52525b',
  },
};

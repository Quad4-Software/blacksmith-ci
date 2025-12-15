import type { ThemeDefinition } from './types';

/**
 * Crow CI default theme
 *
 * A warm, earthy theme featuring orange primary colors
 * with cream/stone neutrals for a professional yet approachable look.
 *
 * Light: Burnt orange primary (#c2410c) with warm cream secondary (#e9e6dc)
 * Dark: Bright orange primary (#fb923c) with off-white secondary (#faf9f5)
 */
export const crowCiTheme: ThemeDefinition = {
  id: 'crow-ci',
  name: 'Crow CI',
  description: 'Default Crow CI theme with warm orange accents',

  light: {
    // Backgrounds - warm cream tones
    background: 'oklch(0.985 0.005 90)',
    foreground: 'oklch(0.25 0.015 50)',

    // Card - slightly lighter than background
    card: 'oklch(0.995 0.003 90)',
    cardForeground: 'oklch(0.25 0.015 50)',

    // Popover - same as card
    popover: 'oklch(0.995 0.003 90)',
    popoverForeground: 'oklch(0.25 0.015 50)',

    // Primary - burnt orange #c2410c
    primary: 'oklch(0.52 0.17 35)',
    primaryForeground: 'oklch(0.98 0.005 90)',

    // Secondary - warm cream #e9e6dc
    secondary: 'oklch(0.92 0.01 85)',
    secondaryForeground: 'oklch(0.35 0.02 50)',

    // Muted - subtle warm gray
    muted: 'oklch(0.94 0.008 85)',
    mutedForeground: 'oklch(0.50 0.015 50)',

    // Accent - lighter orange tint for hover states
    accent: 'oklch(0.94 0.03 60)',
    accentForeground: 'oklch(0.30 0.02 50)',

    // Destructive - warm red
    destructive: 'oklch(0.55 0.22 25)',
    destructiveForeground: 'oklch(0.98 0.005 90)',

    // Borders - warm stone
    border: 'oklch(0.88 0.012 80)',
    input: 'oklch(0.88 0.012 80)',
    ring: 'oklch(0.52 0.17 35)',

    // Charts - varied warm palette
    chart1: 'oklch(0.52 0.17 35)',
    chart2: 'oklch(0.65 0.15 160)',
    chart3: 'oklch(0.55 0.12 250)',
    chart4: 'oklch(0.75 0.15 80)',
    chart5: 'oklch(0.60 0.18 330)',

    // Sidebar - slightly darker than background
    sidebar: 'oklch(0.96 0.008 85)',
    sidebarForeground: 'oklch(0.25 0.015 50)',
    sidebarPrimary: 'oklch(0.52 0.17 35)',
    sidebarPrimaryForeground: 'oklch(0.98 0.005 90)',
    sidebarAccent: 'oklch(0.92 0.02 70)',
    sidebarAccentForeground: 'oklch(0.30 0.02 50)',
    sidebarBorder: 'oklch(0.88 0.012 80)',
    sidebarRing: 'oklch(0.52 0.17 35)',
  },

  dark: {
    // Backgrounds - neutral dark #171717
    background: 'oklch(0.145 0 0)',
    foreground: 'oklch(0.92 0.008 85)',

    // Card - #212121
    card: '#212121',
    cardForeground: 'oklch(0.92 0.008 85)',

    // Popover - same as card
    popover: '#212121',
    popoverForeground: 'oklch(0.92 0.008 85)',

    // Primary - bright orange #fb923c
    primary: '#fb923c',
    primaryForeground: 'oklch(0.145 0 0)',

    // Secondary - off-white #faf9f5
    secondary: 'oklch(0.98 0.005 90)',
    secondaryForeground: 'oklch(0.20 0 0)',

    // Muted - neutral dark gray
    muted: 'oklch(0.25 0 0)',
    mutedForeground: 'oklch(0.65 0 0)',

    // Accent - subtle orange tint for hover states
    accent: '#3d2a1a',
    accentForeground: 'oklch(0.92 0.008 85)',

    // Destructive - warm red
    destructive: 'oklch(0.60 0.20 25)',
    destructiveForeground: 'oklch(0.98 0.005 90)',

    // Borders and inputs
    border: '#404040',
    input: '#212121',
    ring: '#fb923c',

    // Charts - vibrant palette for dark mode
    chart1: '#fb923c',
    chart2: 'oklch(0.70 0.14 170)',
    chart3: 'oklch(0.65 0.15 260)',
    chart4: 'oklch(0.80 0.12 90)',
    chart5: 'oklch(0.70 0.16 340)',

    // Sidebar - slightly darker than background
    sidebar: 'oklch(0.12 0 0)',
    sidebarForeground: 'oklch(0.92 0.008 85)',
    sidebarPrimary: '#fb923c',
    sidebarPrimaryForeground: 'oklch(0.145 0 0)',
    sidebarAccent: '#3d2a1a',
    sidebarAccentForeground: 'oklch(0.92 0.008 85)',
    sidebarBorder: 'oklch(0.28 0 0)',
    sidebarRing: '#fb923c',
  },
};

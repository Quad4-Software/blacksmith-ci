/**
 * Theme type definitions for Crow CI
 *
 * The theme system uses shadcn/ui CSS variables for UI components.
 * Custom Crow CI variables (--wp-*) for pipeline states, code blocks, etc.
 * are kept unchanged across all themes.
 */

/**
 * All CSS variables that define a theme's appearance.
 * These map directly to shadcn/ui CSS custom properties.
 */
export interface ThemeColorVariables {
  // Core backgrounds
  background: string;
  foreground: string;

  // Card component
  card: string;
  cardForeground: string;

  // Popover/dropdown
  popover: string;
  popoverForeground: string;

  // Primary action color
  primary: string;
  primaryForeground: string;

  // Secondary action color
  secondary: string;
  secondaryForeground: string;

  // Muted/subtle elements
  muted: string;
  mutedForeground: string;

  // Accent highlights
  accent: string;
  accentForeground: string;

  // Destructive/error actions
  destructive: string;
  destructiveForeground: string;

  // Borders and inputs
  border: string;
  input: string;
  ring: string;

  // Chart colors for data visualization
  chart1: string;
  chart2: string;
  chart3: string;
  chart4: string;
  chart5: string;

  // Sidebar specific
  sidebar: string;
  sidebarForeground: string;
  sidebarPrimary: string;
  sidebarPrimaryForeground: string;
  sidebarAccent: string;
  sidebarAccentForeground: string;
  sidebarBorder: string;
  sidebarRing: string;
}

/**
 * A complete theme definition with both light and dark variants.
 */
export interface ThemeDefinition {
  /** Unique identifier for the theme (used in localStorage) */
  id: string;

  /** Display name shown in the UI */
  name: string;

  /** Optional description of the theme */
  description?: string;

  /** Optional author/source attribution */
  author?: string;

  /** Light mode color variables */
  light: ThemeColorVariables;

  /** Dark mode color variables */
  dark: ThemeColorVariables;
}

/**
 * Theme option for UI display (subset of ThemeDefinition)
 */
export interface ThemeOption {
  id: string;
  name: string;
  description?: string;
}

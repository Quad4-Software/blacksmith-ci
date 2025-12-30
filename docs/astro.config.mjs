// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightThemeNova from 'starlight-theme-nova';
import tailwindcss from '@tailwindcss/vite';
import markdoc from '@astrojs/markdoc';
import starlightLinksValidator from 'starlight-links-validator';
import starlightUtils from '@lorenzo_lewis/starlight-utils';
import starlightScrollToTop from 'starlight-scroll-to-top';
import starlightVersions from 'starlight-versions';
import starlightImageZoom from 'starlight-image-zoom';
import rehypeMermaid from 'rehype-mermaid';
import rehypeFigure from '@microflash/rehype-figure';

// https://astro.build/config
export default defineConfig({
  site: 'https://crowci.dev',
  output: 'static',
  trailingSlash: 'ignore',
  integrations: [
    markdoc(),
    starlight({
      title: 'Crow CI',
      favicon: '/favicon-light.svg',
      customCss: ['./src/styles/base.css', './src/styles/starlight-mappings.css', './src/styles/custom.css'],
      head: [
        // Theme-aware favicons: dark icon for light mode, light icon for dark mode
        // Note: favicon-light = dark icon (for light backgrounds), favicon-dark = light icon (for dark backgrounds)
        {
          tag: 'link',
          attrs: {
            rel: 'icon',
            href: '/favicon-light.svg',
            type: 'image/svg+xml',
            media: '(prefers-color-scheme: light)',
          },
        },
        {
          tag: 'link',
          attrs: {
            rel: 'icon',
            href: '/favicon-dark.svg',
            type: 'image/svg+xml',
            media: '(prefers-color-scheme: dark)',
          },
        },
        // PNG fallback for browsers that don't support SVG favicons
        {
          tag: 'link',
          attrs: {
            rel: 'icon',
            href: '/favicon-light.png',
            type: 'image/png',
            media: '(prefers-color-scheme: light)',
          },
        },
        {
          tag: 'link',
          attrs: {
            rel: 'icon',
            href: '/favicon-dark.png',
            type: 'image/png',
            media: '(prefers-color-scheme: dark)',
          },
        },
        {
          tag: 'script',
          attrs: {
            type: 'module',
          },
          content: `
            import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs';
            mermaid.initialize({ startOnLoad: true, theme: 'neutral' });
          `,
        },
      ],
      components: {
        SiteTitle: './src/components/SiteTitle.astro',
        // Override Banner and PageTitle to remove version outdated warnings
        Banner: './src/components/Banner.astro',
        PageTitle: './src/components/PageTitle.astro',
        // Custom theme switcher
        ThemeProvider: './src/components/ThemeProvider.astro',
        ThemeSelect: './src/components/ThemeSelect.astro',
        // Add Liberapay donation link
        SocialIcons: './src/components/SocialIcons.astro',
      },
      expressiveCode: {
        themes: ['catppuccin-mocha', 'github-light'],
        useStarlightDarkModeSwitch: true,
        useStarlightUiThemeColors: false,
        styleOverrides: { borderRadius: '0', codePaddingBlock: '0.8rem' },
      },
      plugins: [
        starlightImageZoom(),
        starlightVersions({
          current: { label: 'v5.0 (latest)', redirect: 'root' },
          versions: [
            { slug: 'v5-0', label: 'v5.0' },
            { slug: 'v4-6', label: 'v4.6' },
            { slug: 'v4-5', label: 'v4.5' },
            { slug: 'dev', label: 'dev' },
          ],
        }),
        starlightThemeNova(),
        starlightLinksValidator(),
        starlightUtils({
          multiSidebar: {
            switcherStyle: 'horizontalList',
          },
        }),
        starlightScrollToTop(),
      ],
      social: [
        {
          icon: 'codeberg',
          label: 'Codeberg',
          href: 'https://codeberg.org/crowci/crow',
        },
      ],
      editLink: {
        baseUrl: 'https://codeberg.org/crowci/crow/src/branch/main/docs/src/content/docs/',
      },
      // Sidebar is empty at root level - versioned docs use their own sidebar configs
      sidebar: [],
    }),
  ],

  markdown: {
    rehypePlugins: [[rehypeMermaid, { strategy: 'pre-mermaid' }], rehypeFigure],
  },

  vite: {
    plugins: [tailwindcss()],
  },
});

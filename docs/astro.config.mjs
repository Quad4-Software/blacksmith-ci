// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightCelestiaTheme from 'starlight-theme-celestia';
import markdoc from '@astrojs/markdoc';
import starlightLinksValidator from 'starlight-links-validator';
import starlightUtils from '@lorenzo_lewis/starlight-utils';
import starlightScrollToTop from 'starlight-scroll-to-top';
import starlightVersions from 'starlight-versions';
import starlightImageZoom from 'starlight-image-zoom';
import rehypeMermaid from 'rehype-mermaid';
import rehypeFigure from '@microflash/rehype-figure';
// Astro 7 defaults to its native Sätteri Markdown pipeline; opt back into the
// remark/rehype (unified) processor so the rehype plugins below keep working.
import { unified } from '@astrojs/markdown-remark';

// https://astro.build/config
export default defineConfig({
  site: 'https://crowci.dev',
  output: 'static',
  trailingSlash: 'ignore',
  integrations: [
    markdoc(),
    starlight({
      title: 'Crow CI',
      logo: {
        src: './public/logo.svg',
        replacesTitle: true,
      },
      favicon: '/favicon-light.svg',
      customCss: ['./src/styles/custom.css'],
      head: [
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
        // Override Banner and PageTitle to suppress version outdated warnings
        Banner: './src/components/Banner.astro',
        PageTitle: './src/components/PageTitle.astro',
        // Override Search to scope results to the current docs version
        // (forked from Celestia's Search; see file header for context)
        Search: './src/components/Search.astro',
        // Compose celestia's MarkdownContent (code copy button) with image-zoom's,
        // which otherwise clobber each other depending on plugin order.
        MarkdownContent: './src/components/MarkdownContent.astro',
      },
      plugins: [
        starlightImageZoom(),
        starlightCelestiaTheme({
          stylingSystem: 'css',
          nav: [
            { label: 'CodeFloe', href: 'https://codefloe.com/crowci/crow' },
            { label: 'Donate', href: 'https://liberapay.com/crowci' },
          ],
        }),
        starlightVersions({
          current: { label: 'v6.5 (latest)', redirect: 'root' },
          versions: [
            { slug: 'v6-5', label: 'v6.5' },
            { slug: 'v6-4', label: 'v6.4' },
            { slug: 'v6-3', label: 'v6.3' },
            { slug: 'v6-2', label: 'v6.2' },
            { slug: 'v5-13', label: 'v5.13' },
            { slug: 'dev', label: 'dev' },
          ],
        }),
        starlightLinksValidator(),
        starlightUtils({
          multiSidebar: {
            switcherStyle: 'horizontalList',
          },
        }),
        starlightScrollToTop(),
      ],
      social: [],
      editLink: {
        baseUrl: 'https://codefloe.com/crowci/crow/src/branch/main/docs/src/content/docs/',
      },
      sidebar: [],
    }),
  ],

  markdown: {
    processor: unified(),
    // Astro 6.4.2 regressed the default for `gfm` from true to false;
    // see https://github.com/withastro/astro/issues/16971
    gfm: true,
    rehypePlugins: [[rehypeMermaid, { strategy: 'pre-mermaid' }], rehypeFigure],
  },

});

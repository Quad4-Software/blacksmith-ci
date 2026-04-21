// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightCelestiaTheme from 'starlight-celestia-theme';
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
          current: { label: 'v5.10 (latest)', redirect: 'root' },
          versions: [
            { slug: 'v5-10', label: 'v5.10' },
            { slug: 'v5-9', label: 'v5.9' },
            { slug: 'v5-8', label: 'v5.8' },
            { slug: 'v5-7', label: 'v5.7' },
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
    rehypePlugins: [[rehypeMermaid, { strategy: 'pre-mermaid' }], rehypeFigure],
  },

});

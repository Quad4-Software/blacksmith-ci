import { readdirSync } from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import VueI18nPlugin from '@intlify/unplugin-vue-i18n/vite';
import vue from '@vitejs/plugin-vue';
import type { Plugin } from 'vite';
import prismjs from 'vite-plugin-prismjs';
import svgLoader from 'vite-svg-loader';
import type { ViteUserConfig } from 'vitest/config';
import { defineConfig } from 'vitest/config';
import tailwindcss from '@tailwindcss/vite';

function woodpeckerInfoPlugin(): Plugin {
  return {
    name: 'woodpecker-info',
    configureServer() {
      const info = 'Access the Crow UI at http://localhost:8000/\n\n';
      console.log(info);
    },
  };
}

function externalCSSPlugin(): Plugin {
  return {
    name: 'external-css',
    transformIndexHtml: {
      order: 'post',
      handler() {
        return [
          {
            tag: 'link',
            attrs: { rel: 'stylesheet', type: 'text/css', href: '/assets/custom.css' },
            injectTo: 'head',
          },
        ];
      },
    },
  };
}

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    tailwindcss(),
    vue(),
    VueI18nPlugin({
      include: path.resolve(__dirname, 'src/assets/locales/**'),
    }),
    (() => {
      const virtualModuleId = 'virtual:vue-i18n-supported-locales';
      const resolvedVirtualModuleId = `\0${virtualModuleId}`;

      const filenames = readdirSync('src/assets/locales/').map((filename) => filename.replace('.json', ''));

      return {
        name: 'vue-i18n-supported-locales',

        resolveId(id) {
          if (id === virtualModuleId) {
            return resolvedVirtualModuleId;
          }
        },

        load(id) {
          if (id === resolvedVirtualModuleId) {
            return `export const SUPPORTED_LOCALES = ${JSON.stringify(filenames)}`;
          }
        },
      };
    })(),
    svgLoader(),
    externalCSSPlugin(),
    woodpeckerInfoPlugin(),
    prismjs({
      languages: ['yaml'],
    }),
  ],
  resolve: {
    alias: {
      '~/': `${path.resolve(__dirname, 'src')}/`,
    },
  },
  logLevel: 'warn',
  build: {
    chunkSizeWarningLimit: 1000,
    rollupOptions: {
      output: {
        manualChunks: {
          vue: ['vue', 'vue-router', 'pinia'],
          'vue-i18n': ['vue-i18n'],
          vueuse: ['@vueuse/core'],
          icons: ['@iconify/vue', '@mdi/js', 'simple-icons'],
          flow: ['@vue-flow/core', '@vue-flow/background', '@vue-flow/controls'],
        },
      },
    },
  },
  server: {
    allowedHosts: true,
    host: process.env.VITE_DEV_SERVER_HOST ?? '127.0.0.1',
    port: 8010,
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./test-setup.ts'],
    pool: 'forks',
    poolOptions: {
      forks: {
        singleFork: true,
      },
    },
  },
} as ViteUserConfig);

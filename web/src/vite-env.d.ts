/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue';
  // eslint-disable-next-line ts/no-empty-object-type
  const component: DefineComponent<{}, {}, any>;
  export default component;
}

declare module 'virtual:vue-i18n-supported-locales' {
  export const SUPPORTED_LOCALES: string[];
}

declare module '@dagrejs/dagre' {
  const dagre: any;
  export default dagre;
}

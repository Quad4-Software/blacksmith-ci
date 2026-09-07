import { config } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';

// happy-dom leaves `localStorage` undefined here, because node's own
// experimental global shadows it and is inert without --localstorage-file.
// Anything reading stored settings — `useStorage`, `useTheme` at module load —
// therefore throws on import, so give the suite a working in-memory Storage.
if (typeof window !== 'undefined' && window.localStorage === undefined) {
  const store = new Map<string, string>();
  const localStorageShim: Storage = {
    get length() {
      return store.size;
    },
    key: (index: number) => [...store.keys()][index] ?? null,
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, String(value)),
    removeItem: (key: string) => void store.delete(key),
    clear: () => store.clear(),
  };
  // Writable, because specs that predate this shim install their own by
  // assigning `globalThis.localStorage`, which a read-only property rejects.
  const descriptor = { value: localStorageShim, configurable: true, writable: true };
  Object.defineProperty(window, 'localStorage', descriptor);
  Object.defineProperty(globalThis, 'localStorage', descriptor);
}

// Create a simple i18n instance for testing
const i18n = createI18n({
  legacy: false,
  locale: 'en',
  missingWarn: false,
  fallbackWarn: false,
  messages: {
    en: {},
  },
});

// Configure Vue Test Utils global settings
config.global.plugins = [i18n];
config.global.stubs = {
  // Stub router-link as a simple anchor tag
  RouterLink: {
    template: '<a><slot /></a>',
    props: ['to'],
  },
};

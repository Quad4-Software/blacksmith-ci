import { useStorage } from '@vueuse/core';
import { nextTick, ref } from 'vue';
import { createI18n } from 'vue-i18n';

import { useDate } from './useDate';

export function getUserLanguage(): string {
  const browserLocale = navigator.language.split('-')[0];
  const selectedLocale = useStorage('crow:locale', browserLocale).value;

  return selectedLocale;
}

const { setDateLocale } = useDate();
const userLanguage = getUserLanguage();
const fallbackLocale = 'en';

// Track when i18n is ready (initial locale loaded)
export const i18nReady = ref(false);

export const i18n = createI18n({
  locale: userLanguage,
  legacy: false,
  globalInjection: true,
  fallbackLocale,
  messages: {}, // Start with empty messages, load dynamically
  missingWarn: false,
  fallbackWarn: false,
});

const loadLocaleMessages = async (locale: string) => {
  // Skip if already loaded
  if ((i18n.global.availableLocales as readonly string[]).includes(locale)) {
    return;
  }

  const messages = (await import(`~/assets/locales/${locale}.json`)) as { default: any };

  i18n.global.setLocaleMessage(locale, messages.default);

  return nextTick();
};

export const setI18nLanguage = async (lang: string): Promise<void> => {
  if (!(i18n.global.availableLocales as readonly string[]).includes(lang)) {
    await loadLocaleMessages(lang);
  }
  i18n.global.locale.value = lang;
  await setDateLocale(lang);
};

// Load locales and mark as ready when done
async function initializeI18n() {
  const minDisplayTime = 300; // Minimum ms to show loading screen (avoid flash)
  const startTime = Date.now();

  try {
    // Load English first (fallback)
    await loadLocaleMessages(fallbackLocale);
    // Then load user language if different
    if (userLanguage !== fallbackLocale) {
      await loadLocaleMessages(userLanguage);
    }
    await setDateLocale(userLanguage);
  } catch (error) {
    console.error('Failed to load i18n messages:', error);
  } finally {
    // Ensure minimum display time to avoid jarring flash
    const elapsed = Date.now() - startTime;
    if (elapsed < minDisplayTime) {
      await new Promise((resolve) => setTimeout(resolve, minDisplayTime - elapsed));
    }
    // Mark as ready even if there was an error (fallback to keys)
    i18nReady.value = true;
  }
}

void initializeI18n();

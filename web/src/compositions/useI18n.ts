import { useStorage } from '@vueuse/core';
import { nextTick } from 'vue';
import { createI18n } from 'vue-i18n';

import enMessages from '~/assets/locales/en.json';
import { useDate } from './useDate';

export function getUserLanguage(): string {
  const browserLocale = navigator.language.split('-')[0];
  const selectedLocale = useStorage('crow:locale', browserLocale).value;

  return selectedLocale;
}

const { setDateLocale } = useDate();
const userLanguage = getUserLanguage();
const fallbackLocale = 'en';
export const i18n = createI18n({
  locale: userLanguage,
  legacy: false,
  globalInjection: true,
  fallbackLocale,
  messages: {
    en: enMessages,
  },
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
  // @ts-expect-error - vue-i18n types are too strict for dynamic locale values
  i18n.global.locale.value = lang;
  await setDateLocale(lang);
};

// Only load user language if it's different from English (which is already loaded)
if (userLanguage !== fallbackLocale) {
  loadLocaleMessages(userLanguage).catch(console.error);
}
setDateLocale(userLanguage).catch(console.error);

import { config } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';

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

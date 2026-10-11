import type { SwitchAccount, User } from '~/lib/api/types';
import type { CustomThemeConfig } from '~/lib/themes/custom';

export interface AuthConfig {
  oidc: { enabled: boolean; name: string };
  forge_login: boolean;
  forge_count: number | null;
}

declare global {
  interface Window {
    CROW_USER: User | undefined;
    CROW_ACCOUNTS: SwitchAccount[] | undefined;
    CROW_VERSION: string | undefined;
    CROW_BUILD_DATE: string | undefined;
    CROW_SKIP_VERSION_CHECK: boolean | undefined;
    CROW_CSRF: string | undefined;
    CROW_ROOT_PATH: string | undefined;
    CROW_ENABLE_SWAGGER: boolean | undefined;
    CROW_USER_REGISTERED_AGENTS: boolean | undefined;
    CROW_SCHEDULING_STRATEGY: string | undefined;
    CROW_AUTH: AuthConfig | undefined;
    CROW_CUSTOM_THEMES: CustomThemeConfig[] | null | undefined;
  }
}

export default () => ({
  user: window.CROW_USER ?? null,
  accounts: window.CROW_ACCOUNTS ?? [],
  version: window.CROW_VERSION,
  buildDate: window.CROW_BUILD_DATE,
  skipVersionCheck: window.CROW_SKIP_VERSION_CHECK === true || false,
  csrf: window.CROW_CSRF ?? null,
  rootPath: window.CROW_ROOT_PATH ?? '',
  enableSwagger: window.CROW_ENABLE_SWAGGER === true || false,
  userRegisteredAgents: window.CROW_USER_REGISTERED_AGENTS || false,
  schedulingStrategy: window.CROW_SCHEDULING_STRATEGY ?? '',
  auth: window.CROW_AUTH ?? { oidc: { enabled: false, name: '' }, forge_login: true, forge_count: null },
});

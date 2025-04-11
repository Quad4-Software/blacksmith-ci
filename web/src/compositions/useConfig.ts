import type { User } from '~/lib/api/types';

declare global {
  interface Window {
    CROW_USER: User | undefined;
    CROW_VERSION: string | undefined;
    CROW_SKIP_VERSION_CHECK: boolean | undefined;
    CROW_CSRF: string | undefined;
    CROW_ROOT_PATH: string | undefined;
    CROW_ENABLE_SWAGGER: boolean | undefined;
    CROW_USER_REGISTERED_AGENTS: boolean | undefined;
  }
}

export default () => ({
  user: window.CROW_USER ?? null,
  version: window.CROW_VERSION,
  skipVersionCheck: window.CROW_SKIP_VERSION_CHECK === true || false,
  csrf: window.CROW_CSRF ?? null,
  rootPath: window.CROW_ROOT_PATH ?? '',
  enableSwagger: window.CROW_ENABLE_SWAGGER === true || false,
  userRegisteredAgents: window.CROW_USER_REGISTERED_AGENTS || false,
});

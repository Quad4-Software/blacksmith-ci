import type { SwitchAccount, User } from '~/lib/api/types';
import type { CustomThemeConfig } from '~/lib/themes/custom';

// AgentProtocol is the agent protocol range of the server: the version it
// speaks and the oldest agent version it accepts.
export interface AgentProtocol {
  version: number;
  min_agent: number;
}

// AgentAutoRestart is how the server rolls restarts out to agents that accept them.
export interface AgentAutoRestart {
  policy: '' | 'off' | 'outdated' | 'release';
  max_parallel: number;
}

export interface AuthConfig {
  oidc: { enabled: boolean; name: string };
  ldap: { enabled: boolean; name: string };
  // Roles in managed_roles are set by an identity provider and cannot be assigned in the UI.
  managed_roles: string[];
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
    CROW_SMTP_CONFIGURED: boolean | undefined;
    CROW_CUSTOM_THEMES: CustomThemeConfig[] | null | undefined;
    CROW_AGENT_PROTOCOL: AgentProtocol | undefined;
    CROW_AGENT_AUTO_RESTART: AgentAutoRestart | undefined;
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
  agentProtocol: window.CROW_AGENT_PROTOCOL ?? null,
  agentAutoRestart: window.CROW_AGENT_AUTO_RESTART ?? null,
  smtpConfigured: window.CROW_SMTP_CONFIGURED === true,
  auth: window.CROW_AUTH ?? {
    oidc: { enabled: false, name: '' },
    ldap: { enabled: false, name: '' },
    managed_roles: [],
    forge_login: true,
    forge_count: null,
  },
});

// Integration types for external secret store integrations

import { WebhookEvents } from './webhook';

// Events an integration serves secrets to when no event filter is set (mirrors model.DefaultIntegrationEvents)
export const defaultIntegrationEvents: WebhookEvents[] = [
  WebhookEvents.Push,
  WebhookEvents.Tag,
  WebhookEvents.Release,
  WebhookEvents.Deploy,
  WebhookEvents.Cron,
  WebhookEvents.Manual,
];

export type IntegrationType = 'vault' | 'azure_key_vault' | 'infisical';

export interface Integration {
  id: number;
  user_id: number;
  name: string;
  type: IntegrationType;
  description: string;
  config: string;
  allowed_user_ids: number[];
  allowed_org_ids: number[];
  allowed_repo_ids: number[];
  // Pipeline events allowed to use secrets of this integration; empty means the default set without pull requests
  events: WebhookEvents[];
  // Plugin images allowed to use secrets of this integration; empty means any step
  images: string[];
  is_active: boolean;
  created_at: number;
  updated_at: number;
}

export interface IntegrationTypeInfo {
  type: IntegrationType;
  name: string;
}

// Vault-specific config structure (for form handling)
export interface VaultConfig {
  address: string;
  role_id: string;
  secret_id: string;
  auth_path?: string;
  mount_path?: string;
  namespace?: string;
  tls_skip_verify?: boolean;
  ca_certificate?: string;
}

// Azure Key Vault-specific config structure (for form handling)
export interface AzureKeyVaultConfig {
  vault_url: string;
  tenant_id: string;
  client_id: string;
  client_secret: string;
}

// Infisical-specific config structure (for form handling)
export interface InfisicalConfig {
  site_url: string;
  client_id: string;
  client_secret: string;
  project_id: string;
  environment: string;
  secret_path?: string;
}

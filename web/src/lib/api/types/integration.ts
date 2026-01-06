// Integration types for external secret store integrations

export type IntegrationType = 'vault';

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

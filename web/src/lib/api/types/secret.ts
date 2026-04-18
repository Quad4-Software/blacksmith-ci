import type { WebhookEvents } from './webhook';

export interface Secret {
  id: string;
  repo_id: number;
  org_id: number;
  name: string;
  value: string;
  events: WebhookEvents[];
  images: string[];
  source?: string;
  created_at: number;
  updated_at: number;
}

export interface SecretWithOrg extends Secret {
  org_name: string;
  repo_name?: string;
  scope_type: 'user' | 'org' | 'repo' | 'global';
  forge_id?: number;
  forge_name?: string;
  forge_icon?: string;
  forge_type?: string;
  is_shared?: boolean;
  first_owner_name?: string;
}

export interface SecretTarget {
  id: number;
  secret_id: number;
  org_id: number;
  repo_id: number;
  created_at: number;
}

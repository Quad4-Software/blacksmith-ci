// Access token scope definitions
export type AccessTokenScope =
  // User scopes
  | 'repo:read'
  | 'repo:write'
  | 'repo:admin'
  | 'user:read'
  | 'user:write'
  // Admin-only scopes
  | 'admin:read'
  | 'admin:write';

// All user scopes
export const USER_SCOPES: AccessTokenScope[] = ['repo:read', 'repo:write', 'repo:admin', 'user:read', 'user:write'];

// Admin-only scopes
export const ADMIN_SCOPES: AccessTokenScope[] = ['admin:read', 'admin:write'];

// Scope descriptions for UI
export const SCOPE_DESCRIPTIONS: Record<AccessTokenScope, string> = {
  'repo:read': 'Read pipeline status, logs, and repository information',
  'repo:write': 'Trigger, cancel, and approve pipelines',
  'repo:admin': 'Manage repository settings, secrets, and crons',
  'user:read': 'Read user profile and settings',
  'user:write': 'Manage user secrets and registries',
  'admin:read': 'View admin settings, queue, and all users',
  'admin:write': 'Manage users, global secrets, agents, and autoscalers',
};

export interface AccessToken {
  id: number;
  user_id: number;
  org_id: number;
  repo_id: number;
  name: string;
  scopes: string[];
  expires_at: number;
  last_used: number;
  created_at: number;
  updated_at: number;
}

// Response when creating a token - includes the plain token once
export interface AccessTokenWithSecret extends AccessToken {
  token: string;
}

// Request body for creating a token
export interface CreateAccessTokenRequest {
  name: string;
  scopes: AccessTokenScope[];
  org_id?: number;
  repo_id?: number;
  expires_at?: number;
}

// Request body for updating a token
export interface UpdateAccessTokenRequest {
  name?: string;
  scopes?: AccessTokenScope[];
}

// Available scopes response from API
export interface AvailableScopesResponse {
  user: AccessTokenScope[];
  admin?: AccessTokenScope[];
}

// The user account.
export interface User {
  id: number;
  // The unique identifier for the account.

  login: string;
  // The login name for the account.

  email: string;
  // The email address for the account.

  avatar_url: string;
  // The url for the avatar image.

  admin: boolean;
  // Whether the account has administrative privileges.

  active: boolean;
  // Whether the account is currently active.

  org_id: number;
  // The ID of the org assigned to the user.

  forge_id?: number;
  // The ID of the forge the user is using for authentication.

  description?: string;
  // A short bio or description for the user.

  timezone?: string;
  // The user's timezone (e.g., "America/New_York", "Europe/Berlin").

  roles: string[];
  // Instance roles besides admin.

  permissions: Permission[];
  // Effective instance permissions computed by the server.
}

// A remembered account available for quick-switching without a full re-login.
export interface SwitchAccount {
  id: number;
  // The unique identifier for the account.

  login: string;
  // The login name for the account.

  avatar_url: string;
  // The url for the avatar image.

  active: boolean;
  // Whether this is the currently active account.
}

// An instance permission, see the role table in the docs.
export type Permission =
  | 'agents:read'
  | 'agents:register'
  | 'agents:write'
  | 'maintenance:read'
  | 'maintenance:write'
  | 'metrics:all'
  | 'orgs:delete'
  | 'orgs:read'
  | 'queue:read'
  | 'queue:write'
  | 'repos:read'
  | 'repos:repair'
  | 'secrets:write'
  | 'server:admin'
  | 'users:read'
  | 'users:write';

// The instance roles an admin can assign besides admin.
export const instanceRoles = ['operator', 'secrets-manager', 'auditor'] as const;

export type InstanceRole = (typeof instanceRoles)[number];

// UserForgeStatus represents the connection status of a linked forge
export type UserForgeStatus = 'active' | 'expired';

// UserForge represents a user's connection to a forge (multi-forge support)
export interface UserForge {
  id: number;
  forge_id: number;
  forge_name?: string;
  forge_type?: string;
  forge_icon?: string; // custom icon name or URL
  is_primary: boolean;
  forge_login: string;
  forge_email?: string;
  forge_avatar?: string;
  linked_at: number;
  last_used?: number;
  status: UserForgeStatus;
  expiry?: number;
}

// AvailableForge represents a forge that can potentially be linked
export interface AvailableForge {
  id: number;
  url: string;
  type: string;
  is_linked: boolean;
  is_primary?: boolean;
  linked_as?: string;
}

// UnlinkForgeResponse is the response when unlinking a forge
export interface UnlinkForgeResponse {
  message: string;
  disabled_repos: number;
}

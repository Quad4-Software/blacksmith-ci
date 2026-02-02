export interface Registry {
  id: string;
  repo_id: number;
  org_id: number;
  address: string;
  username: string;
  password: string;
  readonly: boolean;
  created_at: number;
  updated_at: number;
}

export interface RegistryWithOrg extends Registry {
  org_name: string;
  scope_type: 'user' | 'org' | 'global';
  forge_id?: number;
  forge_name?: string;
  forge_icon?: string;
  forge_type?: string;
}

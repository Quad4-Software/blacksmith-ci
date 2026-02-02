// A version control organization.
export interface Org {
  // The name of the organization.
  id: number;
  name: string;
  is_user: boolean;
  forge_id: number;
}

export interface OrgPermissions {
  member: boolean;
  admin: boolean;
}

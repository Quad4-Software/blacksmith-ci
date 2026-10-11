// UserIdentity is an identity provider account linked to the current user.
export interface UserIdentity {
  id: number;
  provider: string;
  issuer: string;
  subject: string;
  email?: string;
  username?: string;
  groups: string[] | null;
  linked_at: number;
  last_login?: number;
}

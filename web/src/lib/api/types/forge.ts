export type ForgeType = 'github' | 'gitlab' | 'gitea' | 'forgejo' | 'bitbucket' | 'bitbucket-dc' | 'addon';

export interface Forge {
  id: number;
  type: ForgeType;
  url: string;
  client?: string;
  client_secret?: string;
  skip_verify?: boolean;
  oauth_host?: string;
  icon?: string; // custom icon name (e.g., "codeberg", "github") or URL to custom icon
  additional_options?: Record<string, unknown>;
}

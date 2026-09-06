import type { Secret, SecretWithOrg } from '~/lib/api/types';

/**
 * The page a secret list is rendered on. The all-secrets view (`user`) shows
 * secrets from every scope, while the settings views each show one scope and
 * only allow editing secrets owned by that scope.
 */
export type SecretScopeContext = 'user' | 'repo' | 'org' | 'admin';

export type SecretScopeType = SecretWithOrg['scope_type'];

interface ScopeNames {
  repoName?: string;
  orgName: string;
}

/** Derive the scope of a secret from the owner ids returned by the API. */
export function secretScopeType(secret: Pick<Secret, 'repo_id' | 'org_id'>): Exclude<SecretScopeType, 'user'> {
  if (secret.repo_id !== 0) {
    return 'repo';
  }
  if (secret.org_id !== 0) {
    return 'org';
  }
  return 'global';
}

/**
 * Annotate scope-specific secrets with the fields the secrets table renders
 * (scope type and owner names), which only the all-secrets endpoint returns.
 */
export function toScopedSecrets(secrets: Secret[], names: ScopeNames): SecretWithOrg[] {
  return secrets.map((secret) => {
    const scopeType = secretScopeType(secret);
    return {
      ...secret,
      scope_type: scopeType,
      org_name: names.orgName,
      ...(scopeType === 'repo' && names.repoName !== undefined && names.repoName !== ''
        ? { repo_name: names.repoName }
        : {}),
    };
  });
}

const levelOrder: Record<SecretScopeType, number> = {
  repo: 0,
  user: 1,
  org: 1,
  global: 2,
};

/**
 * Merge the repo, org and global secrets shown on a repo's settings page: a
 * secret defined on a more specific level shadows one of the same name on a
 * broader level, and the remaining secrets are ordered most specific first.
 */
export function mergeRepoSecrets(secrets: Secret[], names: ScopeNames): SecretWithOrg[] {
  const byName = new Map<string, SecretWithOrg>();

  for (const secret of toScopedSecrets(secrets, names)) {
    const existing = byName.get(secret.name);
    if (!existing || levelOrder[secret.scope_type] < levelOrder[existing.scope_type]) {
      byName.set(secret.name, secret);
    }
  }

  return [...byName.values()]
    .toSorted((a, b) => a.name.localeCompare(b.name))
    .toSorted((a, b) => levelOrder[a.scope_type] - levelOrder[b.scope_type]);
}

/**
 * Whether the current user may edit a secret from the page it is listed on.
 * Secrets inherited from a broader scope are read-only on the narrower page,
 * because saving them there would write to the wrong scope.
 */
export function canEditSecret(secret: SecretWithOrg, context: SecretScopeContext, isAdmin: boolean): boolean {
  switch (context) {
    case 'repo':
      return secret.scope_type === 'repo';
    case 'org':
      return secret.scope_type === 'org';
    case 'admin':
      return secret.scope_type === 'global' && isAdmin;
    case 'user':
      return secret.scope_type === 'global' ? isAdmin : true;
  }
}

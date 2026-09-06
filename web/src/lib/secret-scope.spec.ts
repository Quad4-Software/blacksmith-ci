import { describe, expect, it } from 'vitest';

import { canEditSecret, mergeRepoSecrets, secretScopeType, toScopedSecrets } from '~/lib/secret-scope';
import type { Secret, SecretWithOrg } from '~/lib/api/types';
import { WebhookEvents } from '~/lib/api/types';

function secret(overrides: Partial<Secret>): Secret {
  return {
    id: '1',
    repo_id: 0,
    org_id: 0,
    name: 'SECRET',
    value: '',
    events: [WebhookEvents.Push],
    images: [],
    created_at: 0,
    updated_at: 0,
    ...overrides,
  };
}

function scoped(overrides: Partial<SecretWithOrg>): SecretWithOrg {
  return {
    ...secret(overrides),
    org_name: '',
    scope_type: 'repo',
    ...overrides,
  };
}

describe('secretScopeType', () => {
  it('classifies a secret with a repo as repo-scoped', () => {
    expect(secretScopeType(secret({ repo_id: 7, org_id: 0 }))).toBe('repo');
  });

  it('classifies a secret with only an org as org-scoped', () => {
    expect(secretScopeType(secret({ repo_id: 0, org_id: 3 }))).toBe('org');
  });

  it('classifies a secret without repo or org as global', () => {
    expect(secretScopeType(secret({ repo_id: 0, org_id: 0 }))).toBe('global');
  });
});

describe('toScopedSecrets', () => {
  it('annotates secrets with scope type and owner names', () => {
    const result = toScopedSecrets([secret({ id: '1', repo_id: 7, org_id: 3 })], {
      repoName: 'octocat/hello',
      orgName: 'octocat',
    });

    expect(result).toEqual([
      expect.objectContaining({ id: '1', scope_type: 'repo', repo_name: 'octocat/hello', org_name: 'octocat' }),
    ]);
  });

  it('leaves the repo name unset for non-repo secrets', () => {
    const result = toScopedSecrets([secret({ id: '1', org_id: 3 })], { repoName: 'octocat/hello', orgName: 'octocat' });

    expect(result[0].scope_type).toBe('org');
    expect(result[0].repo_name).toBeUndefined();
  });

  it('returns an empty list for no secrets', () => {
    expect(toScopedSecrets([], { orgName: 'octocat' })).toEqual([]);
  });
});

describe('mergeRepoSecrets', () => {
  const context = { repoName: 'octocat/hello', orgName: 'octocat' };

  it('keeps the most specific secret when a name exists on several levels', () => {
    const result = mergeRepoSecrets(
      [
        secret({ id: '1', name: 'TOKEN', repo_id: 0, org_id: 0 }),
        secret({ id: '2', name: 'TOKEN', repo_id: 0, org_id: 3 }),
        secret({ id: '3', name: 'TOKEN', repo_id: 7, org_id: 0 }),
      ],
      context,
    );

    expect(result).toHaveLength(1);
    expect(result[0]).toMatchObject({ id: '3', scope_type: 'repo' });
  });

  it('prefers the org secret over the global one when there is no repo secret', () => {
    const result = mergeRepoSecrets(
      [
        secret({ id: '1', name: 'TOKEN', repo_id: 0, org_id: 0 }),
        secret({ id: '2', name: 'TOKEN', repo_id: 0, org_id: 3 }),
      ],
      context,
    );

    expect(result).toHaveLength(1);
    expect(result[0]).toMatchObject({ id: '2', scope_type: 'org' });
  });

  it('orders repo secrets before org secrets before global ones, each sorted by name', () => {
    const result = mergeRepoSecrets(
      [
        secret({ id: '1', name: 'GLOBAL_B', repo_id: 0, org_id: 0 }),
        secret({ id: '2', name: 'GLOBAL_A', repo_id: 0, org_id: 0 }),
        secret({ id: '3', name: 'ORG', repo_id: 0, org_id: 3 }),
        secret({ id: '4', name: 'REPO_B', repo_id: 7, org_id: 0 }),
        secret({ id: '5', name: 'REPO_A', repo_id: 7, org_id: 0 }),
      ],
      context,
    );

    expect(result.map((s) => s.name)).toEqual(['REPO_A', 'REPO_B', 'ORG', 'GLOBAL_A', 'GLOBAL_B']);
  });
});

describe('canEditSecret', () => {
  it('only allows editing repo secrets in a repo context', () => {
    expect(canEditSecret(scoped({ scope_type: 'repo' }), 'repo', false)).toBe(true);
    expect(canEditSecret(scoped({ scope_type: 'org' }), 'repo', false)).toBe(false);
    expect(canEditSecret(scoped({ scope_type: 'global' }), 'repo', true)).toBe(false);
  });

  it('only allows editing org secrets in an org context', () => {
    expect(canEditSecret(scoped({ scope_type: 'org' }), 'org', false)).toBe(true);
    expect(canEditSecret(scoped({ scope_type: 'repo' }), 'org', false)).toBe(false);
  });

  it('requires an admin to edit global secrets in an admin context', () => {
    expect(canEditSecret(scoped({ scope_type: 'global' }), 'admin', true)).toBe(true);
    expect(canEditSecret(scoped({ scope_type: 'global' }), 'admin', false)).toBe(false);
  });

  it('keeps the all-secrets rules in a user context', () => {
    expect(canEditSecret(scoped({ scope_type: 'user' }), 'user', false)).toBe(true);
    expect(canEditSecret(scoped({ scope_type: 'org' }), 'user', false)).toBe(true);
    expect(canEditSecret(scoped({ scope_type: 'repo' }), 'user', false)).toBe(true);
    expect(canEditSecret(scoped({ scope_type: 'global' }), 'user', false)).toBe(false);
    expect(canEditSecret(scoped({ scope_type: 'global' }), 'user', true)).toBe(true);
  });
});

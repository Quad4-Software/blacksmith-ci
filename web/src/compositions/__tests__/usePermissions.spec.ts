import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it } from 'vitest';

import usePermissions, {
  adminAreaPermissions,
  adminPages,
  adminScopesFor,
  firstAdminRoute,
} from '~/compositions/usePermissions';

// The auth store reads the signed-in user from window.CROW_USER when it is created,
// so the user is set there before each composition call.
function signIn(user: unknown) {
  window.CROW_USER = user as never;
}

describe('usePermissions', () => {
  beforeEach(() => {
    window.CROW_USER = undefined;
    setActivePinia(createPinia());
  });

  it('denies everything when signed out', () => {
    const { can, canAny, isAdmin } = usePermissions();
    expect(can('agents:write')).toBe(false);
    expect(canAny(adminAreaPermissions)).toBe(false);
    expect(isAdmin.value).toBe(false);
  });

  it('follows the permissions from the server', () => {
    signIn({ login: 'aud', admin: false, roles: ['auditor'], permissions: ['agents:read', 'users:read'] });
    const { can, canAny, isAdmin } = usePermissions();
    expect(can('agents:read')).toBe(true);
    expect(can('agents:write')).toBe(false);
    expect(canAny(['agents:write', 'users:read'])).toBe(true);
    expect(canAny(['agents:write', 'queue:write'])).toBe(false);
    expect(isAdmin.value).toBe(false);
  });

  it('treats a user without a permissions field as having none', () => {
    signIn({ login: 'old', admin: false });
    const { can } = usePermissions();
    expect(can('users:read')).toBe(false);
  });

  it('reports admins', () => {
    signIn({ login: 'root', admin: true, roles: [], permissions: ['server:admin'] });
    expect(usePermissions().isAdmin.value).toBe(true);
  });

  describe('firstAdminRoute', () => {
    it('lands on maintenance when the user can read it', () => {
      expect(firstAdminRoute((p) => p === 'maintenance:read' || p === 'users:read')).toBe('admin-settings-maintenance');
    });

    it('skips pages the user cannot see', () => {
      expect(firstAdminRoute((p) => p === 'queue:read')).toBe('admin-settings-queue');
      expect(firstAdminRoute((p) => p === 'orgs:read')).toBe('admin-settings-entities');
      expect(firstAdminRoute((p) => p === 'server:admin')).toBe('admin-settings-notifications');
    });

    it('returns undefined without any admin page', () => {
      expect(firstAdminRoute(() => false)).toBeUndefined();
    });
  });

  it('opens the admin area exactly for the permissions of its pages', () => {
    const pagePermissions = new Set(adminPages.flatMap((page) => page.permissions));
    expect(new Set(adminAreaPermissions)).toStrictEqual(pagePermissions);
  });

  it('tells which admin pages the user can see', () => {
    signIn({ login: 'sec', admin: false, roles: ['secrets-manager'], permissions: ['secrets:write'] });
    const { canSee } = usePermissions();
    expect(canSee('admin-settings-secrets')).toBe(true);
    expect(canSee('admin-settings-maintenance')).toBe(false);
  });

  // Mirrors GetAvailableScopes in server/api/access_token.go.
  describe('adminScopesFor', () => {
    const holding =
      (...ps: string[]) =>
      (p: string) =>
        ps.includes(p);

    it('offers no admin scope without an admin permission', () => {
      expect(adminScopesFor(holding())).toStrictEqual([]);
    });

    it('offers admin:read for read permissions only', () => {
      expect(adminScopesFor(holding('users:read', 'metrics:all'))).toStrictEqual(['admin:read']);
    });

    it('offers both scopes for a write permission', () => {
      expect(adminScopesFor(holding('secrets:write'))).toStrictEqual(['admin:read', 'admin:write']);
      expect(adminScopesFor(holding('queue:read', 'queue:write'))).toStrictEqual(['admin:read', 'admin:write']);
    });
  });
});

import { computed } from 'vue';

import type { AccessTokenScope } from '~/lib/api/types/accessToken';
import type { Permission } from '~/lib/api/types/user';
import useAuthentication from '~/compositions/useAuthentication';

// The admin pages in sidebar order, each with the permissions that show it.
export const adminPages: { route: string; permissions: Permission[] }[] = [
  { route: 'admin-settings-maintenance', permissions: ['maintenance:read'] },
  { route: 'admin-settings-secrets', permissions: ['secrets:write'] },
  { route: 'admin-settings-registries', permissions: ['secrets:write'] },
  { route: 'admin-settings-repos', permissions: ['repos:read'] },
  { route: 'admin-settings-entities', permissions: ['users:read', 'orgs:read'] },
  { route: 'admin-settings-queue', permissions: ['queue:read'] },
  { route: 'admin-settings-notifications', permissions: ['server:admin'] },
];

// Any of these permissions opens the admin area, because it shows at least one admin page.
export const adminAreaPermissions: Permission[] = [...new Set(adminPages.flatMap((page) => page.permissions))];

// Permissions that only grant visibility; every other permission changes state.
// Keep in sync with readPermissions in server/model/role.go.
export const readPermissions: Permission[] = [
  'users:read',
  'orgs:read',
  'repos:read',
  'queue:read',
  'agents:read',
  'maintenance:read',
  'metrics:all',
];

// Every permission that is not in readPermissions.
const writePermissions: Permission[] = [
  'agents:register',
  'agents:write',
  'maintenance:write',
  'orgs:delete',
  'queue:write',
  'repos:repair',
  'secrets:write',
  'server:admin',
  'users:write',
];

// The admin token scopes a user may create, the same rule as User.AdminScopes in server/model/access_token.go:
// admin:read with any admin permission, admin:write with any write permission.
export function adminScopesFor(can: (p: Permission) => boolean): AccessTokenScope[] {
  const scopes: AccessTokenScope[] = [];
  if (readPermissions.some(can) || writePermissions.some(can)) scopes.push('admin:read');
  if (writePermissions.some(can)) scopes.push('admin:write');
  return scopes;
}

// The first admin page the user can see, used as the target of the admin area's default route.
export function firstAdminRoute(can: (p: Permission) => boolean): string | undefined {
  return adminPages.find((page) => page.permissions.some((p) => can(p)))?.route;
}

export default () => {
  // Through useAuthentication, so specs that mock it to avoid Pinia keep working.
  const auth = useAuthentication();
  const permissions = computed(() => new Set<Permission>(auth.user?.permissions ?? []));
  const can = (p: Permission) => permissions.value.has(p);

  return {
    can,
    canAny: (ps: Permission[]) => ps.some((p) => permissions.value.has(p)),
    // Whether the admin page with this route name is visible to the user.
    canSee: (route: string) => adminPages.some((page) => page.route === route && page.permissions.some(can)),
    isAdmin: computed(() => !!auth.user?.admin),
  } as const;
};

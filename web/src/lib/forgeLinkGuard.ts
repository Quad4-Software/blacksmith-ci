// Routes a user without any linked forge can still reach.
const EXEMPT_PATHS = ['/user', '/admin', '/login', '/cli/auth'];

// needsForgeLink reports whether a signed-in user without forges must be sent
// to the forge linking screen before visiting `path`. Only single sign-on users
// can be without a forge, so the guard is off unless OIDC is enabled.
export function needsForgeLink(
  path: string,
  rootPath: string,
  isAuthenticated: boolean,
  forgeCount: number | null,
  oidcEnabled: boolean,
): boolean {
  if (!oidcEnabled || !isAuthenticated || forgeCount === null || forgeCount > 0) {
    return false;
  }
  const relative = rootPath !== '' && path.startsWith(rootPath) ? path.slice(rootPath.length) : path;
  return !EXEMPT_PATHS.some((p) => relative === p || relative.startsWith(`${p}/`));
}

// Pages that must never be the target of a post-login redirect: returning to
// them would loop back into the sign-in flow or sign the user out again.
const authPagePattern = /^\/(?:login|logout|authorize)(?:[/?#]|$)/i;
// eslint-disable-next-line no-control-regex
const controlCharPattern = /[\u0000-\u001F\u007F]/;

/**
 * Returns raw when it is an in-app path that is safe to navigate to after a
 * sign-in, and undefined otherwise. Accepted paths start with a single "/",
 * contain no backslash or control character, live below rootPath and do not
 * point at the login, logout or authorize pages.
 */
export function safeRedirectPath(raw: unknown, rootPath = ''): string | undefined {
  if (typeof raw !== 'string' || raw === '') {
    return undefined;
  }
  if (!raw.startsWith('/') || raw.startsWith('//') || raw.includes('\\') || controlCharPattern.test(raw)) {
    return undefined;
  }

  let appPath = raw;
  if (rootPath !== '') {
    if (raw !== rootPath && !raw.startsWith(`${rootPath}/`)) {
      return undefined;
    }
    appPath = raw.slice(rootPath.length) || '/';
  }
  if (appPath.startsWith('//') || authPagePattern.test(appPath)) {
    return undefined;
  }
  return raw;
}

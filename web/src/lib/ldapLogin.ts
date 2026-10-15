export type LdapLoginResult = { ok: true } | { ok: false; code: string; retryAfter?: number };

// ldapLogin posts directory credentials to the server, which sets the session
// cookies on success (204) and answers failures with `{"error": "<code>"}`.
export async function ldapLogin(rootPath: string, username: string, password: string): Promise<LdapLoginResult> {
  let res: Response;
  try {
    res = await fetch(`${rootPath}/authorize/ldap`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
      credentials: 'same-origin',
    });
  } catch {
    return { ok: false, code: 'internal_error' };
  }
  if (res.status === 204) {
    return { ok: true };
  }

  let code = 'internal_error';
  try {
    const body = (await res.json()) as { error?: unknown };
    if (typeof body.error === 'string' && body.error !== '') {
      code = body.error;
    }
  } catch {
    // not JSON (e.g. a proxy error page): keep the generic code
  }
  const retryAfter = Number.parseInt(res.headers.get('Retry-After') ?? '', 10);
  return Number.isNaN(retryAfter) ? { ok: false, code } : { ok: false, code, retryAfter };
}

type Translate = (key: string, named?: Record<string, unknown>, plural?: number) => string;

// ldapErrorMessage maps a server error code to a translated message.
export function ldapErrorMessage(t: Translate, code: string, retryAfter?: number): string {
  switch (code) {
    case 'invalid_credentials':
      return t('ldap.invalid_credentials');
    case 'ldap_access_denied':
      return t('ldap.ldap_access_denied');
    case 'too_many_attempts': {
      const minutes = Math.max(1, Math.ceil((retryAfter ?? 60) / 60));
      return t('ldap.too_many_attempts', { minutes }, minutes);
    }
    case 'ldap_unavailable':
      return t('ldap.ldap_unavailable');
    case 'registration_closed':
      return t('registration_closed');
    case 'idp_login_taken':
      return t('idp_login_taken');
    case 'idp_invalid_username':
      return t('idp_invalid_username');
    case 'idp_missing_username':
      return t('idp_missing_username');
    case 'identity_taken':
      return t('user.settings.sso.identity_taken');
    case 'identity_already_linked':
      return t('user.settings.sso.identity_already_linked');
    case 'forge_sign_in_required':
      return t('user.settings.sso.forge_sign_in_required');
    default:
      return t('internal_error');
  }
}

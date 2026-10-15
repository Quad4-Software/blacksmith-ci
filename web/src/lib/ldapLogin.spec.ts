import { afterEach, describe, expect, it, vi } from 'vitest';

import { ldapErrorMessage, ldapLogin } from './ldapLogin';

function stubFetch(res: Response) {
  const fetchMock = vi.fn().mockResolvedValue(res);
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

describe('ldapLogin', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('returns ok on 204 and posts JSON to the root path', async () => {
    const fetchMock = stubFetch(new Response(null, { status: 204 }));

    await expect(ldapLogin('/root', 'jane', 'secret')).resolves.toEqual({ ok: true });

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/root/authorize/ldap');
    expect(init.method).toBe('POST');
    expect(init.headers).toEqual({ 'Content-Type': 'application/json' });
    expect(init.credentials).toBe('same-origin');
    expect(JSON.parse(init.body as string)).toEqual({ username: 'jane', password: 'secret' });
  });

  it('returns the error code of a failure', async () => {
    stubFetch(new Response(JSON.stringify({ error: 'invalid_credentials' }), { status: 401 }));
    await expect(ldapLogin('', 'jane', 'x')).resolves.toEqual({ ok: false, code: 'invalid_credentials' });
  });

  it('reads Retry-After in seconds', async () => {
    stubFetch(
      new Response(JSON.stringify({ error: 'too_many_attempts' }), { status: 429, headers: { 'Retry-After': '120' } }),
    );
    await expect(ldapLogin('', 'jane', 'x')).resolves.toEqual({
      ok: false,
      code: 'too_many_attempts',
      retryAfter: 120,
    });
  });

  it('falls back to internal_error for a non-JSON response', async () => {
    stubFetch(new Response('<html>bad gateway</html>', { status: 502 }));
    await expect(ldapLogin('', 'jane', 'x')).resolves.toEqual({ ok: false, code: 'internal_error' });
  });

  it('falls back to internal_error when the request fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network')));
    await expect(ldapLogin('', 'jane', 'x')).resolves.toEqual({ ok: false, code: 'internal_error' });
  });
});

describe('ldapErrorMessage', () => {
  const t = (key: string, named?: Record<string, unknown>) => (named ? `${key}:${JSON.stringify(named)}` : key);

  it('maps codes to message keys', () => {
    expect(ldapErrorMessage(t, 'invalid_credentials')).toBe('ldap.invalid_credentials');
    expect(ldapErrorMessage(t, 'registration_closed')).toBe('registration_closed');
    expect(ldapErrorMessage(t, 'identity_taken')).toBe('user.settings.sso.identity_taken');
    expect(ldapErrorMessage(t, 'whatever')).toBe('internal_error');
  });

  it('rounds the wait up to whole minutes', () => {
    expect(ldapErrorMessage(t, 'too_many_attempts', 90)).toBe('ldap.too_many_attempts:{"minutes":2}');
  });
});

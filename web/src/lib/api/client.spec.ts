import { afterEach, describe, expect, it, vi } from 'vitest';

import type { useAuthStore } from '~/store/auth';

import ApiClient, { ApiRequestError } from './client';

// Huma answers a failed request with RFC 9457 problem details.
function problem(status: number, detail: string) {
  return new Response(JSON.stringify({ title: 'Unauthorized', status, detail }), {
    status,
    headers: { 'Content-Type': 'application/problem+json' },
  });
}

function setup(res: Response) {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(res));
  const authStore = {
    isAuthenticated: true,
    isSessionExpired: false,
    isRedirecting: false,
    invalidateSession: vi.fn(),
    registerAbortController: vi.fn(),
    removeAbortController: vi.fn(),
  };
  const client = new ApiClient('', null, 'csrf');
  client.setAuthStore(authStore as unknown as ReturnType<typeof useAuthStore>);
  const onerror = vi.fn();
  client.setErrorHandler(onerror);
  return { client, authStore, onerror };
}

describe('apiClient 401 handling', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('keeps the session when the directory rejects the credentials', async () => {
    const { client, authStore, onerror } = setup(problem(401, 'invalid_credentials'));

    const err: unknown = await client
      ._post('/api/user/identities/ldap', { username: 'jane', password: 'x' })
      .catch((error: unknown) => error);

    expect(err).toBeInstanceOf(ApiRequestError);
    expect((err as ApiRequestError).status).toBe(401);
    expect((err as ApiRequestError).message).toBe('invalid_credentials');
    expect(authStore.invalidateSession).not.toHaveBeenCalled();
    expect(onerror).toHaveBeenCalledOnce();
  });

  it('invalidates the session on any other 401', async () => {
    const { client, authStore } = setup(problem(401, 'session expired'));

    await expect(client._get('/api/user')).rejects.toBeInstanceOf(ApiRequestError);

    expect(authStore.invalidateSession).toHaveBeenCalledOnce();
  });

  it('skips the error handler for a silent request', async () => {
    const { client, onerror } = setup(problem(401, 'invalid_credentials'));

    await expect(client._post('/api/user/identities/ldap', {}, { silent: true })).rejects.toBeInstanceOf(
      ApiRequestError,
    );

    expect(onerror).not.toHaveBeenCalled();
  });
});

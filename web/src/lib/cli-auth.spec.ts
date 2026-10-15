import { afterEach, describe, expect, it, vi } from 'vitest';

import { cliTokenUrl, sendCliToken } from './cli-auth';

describe('cliTokenUrl', () => {
  it.each([
    ['1', 'http://localhost:1/token'],
    ['8080', 'http://localhost:8080/token'],
    ['65535', 'http://localhost:65535/token'],
    ['08080', 'http://localhost:8080/token'],
    ['80', 'http://localhost/token'],
  ])('accepts port %s on localhost', (port, expected) => {
    const url = cliTokenUrl(port);
    expect(url?.hostname).toBe('localhost');
    expect(url?.href).toBe(expected);
  });

  it.each([
    ['userinfo injection', '1@evil.com'],
    ['host injection', '80/@evil.com'],
    ['path injection', '80/../x'],
    ['zero', '0'],
    ['out of range', '65536'],
    ['too long', '008080'],
    ['negative', '-1'],
    ['empty', ''],
    ['whitespace', ' 80'],
  ])('rejects %s', (_, port) => {
    expect(cliTokenUrl(port)).toBeNull();
  });

  it.each([undefined, null, 80, ['80']])('rejects non-string %j', (port) => {
    expect(cliTokenUrl(port)).toBeNull();
  });
});

describe('sendCliToken', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  const url = new URL('http://localhost:1234/token');

  it('posts the token and reports success', async () => {
    const fetchMock = vi.fn().mockResolvedValue(Response.json({ ok: 'true' }));
    vi.stubGlobal('fetch', fetchMock);

    await expect(sendCliToken(url, 'abc')).resolves.toBe(true);
    expect(fetchMock).toHaveBeenCalledWith(url, expect.objectContaining({ method: 'POST', body: '{"token":"abc"}' }));
  });

  it('reports failure when the CLI does not acknowledge', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(Response.json({ ok: 'false' })));
    await expect(sendCliToken(url, 'abc')).resolves.toBe(false);
  });

  it('reports failure when the listener is down', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')));
    await expect(sendCliToken(url, 'abc')).resolves.toBe(false);
  });

  it('reports failure on a non-JSON answer', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('nope')));
    await expect(sendCliToken(url, 'abc')).resolves.toBe(false);
  });
});

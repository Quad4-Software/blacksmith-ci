import { describe, expect, it } from 'vitest';

import { safeRedirectPath } from './redirect';

describe('safeRedirectPath', () => {
  it.each(['/', '/repos', '/repos/1/pipeline/2?tab=logs#L3', '/user?forge_linked=true', '/login-help', '/repos/login'])(
    'accepts the in-app path %s',
    (path) => {
      expect(safeRedirectPath(path)).toBe(path);
    },
  );

  it.each([
    undefined,
    null,
    42,
    ['/repos'],
    '',
    'repos',
    '//evil.com',
    '/\\evil.com',
    '/repos\\x',
    'https://evil.com',
    'javascript:alert(1)',
    '/repos\n/x',
    '/\t/evil.com',
    '/login',
    '/login?url=/repos',
    '/LOGIN',
    '/logout',
    '/logout?all=true',
    '/authorize',
    '/authorize?forge_id=1',
    '/authorize/oidc',
  ])('rejects %j', (value) => {
    expect(safeRedirectPath(value)).toBeUndefined();
  });

  describe('with a root path', () => {
    it.each(['/crow', '/crow/', '/crow/repos/1'])('accepts %s', (path) => {
      expect(safeRedirectPath(path, '/crow')).toBe(path);
    });

    it.each(['/repos', '/crowbar', '/crow/login', '/crow/logout', '/crow/authorize?forge_id=2', '/crow//evil.com'])(
      'rejects %s',
      (path) => {
        expect(safeRedirectPath(path, '/crow')).toBeUndefined();
      },
    );
  });
});

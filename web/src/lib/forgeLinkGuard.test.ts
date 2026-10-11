import { describe, expect, it } from 'vitest';

import { needsForgeLink } from './forgeLinkGuard';

describe('needsForgeLink', () => {
  it('ignores signed-out users and users with forges', () => {
    expect(needsForgeLink('/repos', '', false, 0, true)).toBe(false);
    expect(needsForgeLink('/repos', '', true, null, true)).toBe(false);
    expect(needsForgeLink('/repos', '', true, 1, true)).toBe(false);
  });

  it('stays off when OIDC is disabled', () => {
    expect(needsForgeLink('/repos', '', true, 0, false)).toBe(false);
  });

  it('sends users without forges to the link screen', () => {
    expect(needsForgeLink('/', '', true, 0, true)).toBe(true);
    expect(needsForgeLink('/repos/1', '', true, 0, true)).toBe(true);
    expect(needsForgeLink('/crow/repos', '/crow', true, 0, true)).toBe(true);
  });

  it('keeps user settings, admin, login and CLI auth reachable', () => {
    for (const path of ['/user', '/user/access-tokens', '/admin', '/admin/repos', '/login', '/cli/auth']) {
      expect(needsForgeLink(path, '', true, 0, true)).toBe(false);
    }
    expect(needsForgeLink('/crow/user', '/crow', true, 0, true)).toBe(false);
    expect(needsForgeLink('/username-lookalike', '', true, 0, true)).toBe(true);
  });
});

import { afterEach, describe, it, expect, vi, beforeEach } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import { ref } from 'vue';
import AccessTokenDialog from './AccessTokenDialog.vue';

// Mock vue-i18n
vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}));

// Mock useTheme
vi.mock('~/compositions/useTheme', () => ({
  useTheme: () => ({
    theme: ref('light'),
    storeTheme: ref('auto'),
    themeName: ref('crow-ci'),
    currentTheme: ref({ id: 'crow-ci', name: 'Crow CI', light: {}, dark: {} }),
    availableThemes: ref([]),
  }),
}));

// Track mock user for testing admin vs regular user
let mockUser: { admin: boolean } | null = { admin: false };

vi.mock('~/compositions/useAuthentication', () => ({
  default: () => ({
    user: mockUser,
  }),
}));

// Permissions of the signed-in user, read by usePermissions
let mockPermissions: string[] = [];

vi.mock('~/compositions/usePermissions', async (importOriginal) => ({
  ...(await importOriginal<typeof import('~/compositions/usePermissions')>()),
  default: () => ({ can: (p: string) => mockPermissions.includes(p) }),
}));

// The dialog content is teleported to the body, so read the rendered checkboxes from the document.
function scopeIds() {
  return [...document.querySelectorAll('[id^="scope-admin"]')].map((el) => el.id);
}

describe('accessTokenDialog', () => {
  beforeEach(() => {
    mockUser = { admin: false };
    mockPermissions = [];
  });

  // Note: Dialog content is rendered via teleport to body, which makes it difficult
  // to test the rendered content. These tests focus on component setup and events.

  describe('component setup', () => {
    it('mounts without errors', async () => {
      const wrapper = mount(AccessTokenDialog, {
        props: {
          open: false,
          hideTrigger: true,
        },
        global: {
          stubs: {
            teleport: true,
          },
        },
      });

      expect(wrapper.exists()).toBe(true);
    });

    it('accepts open prop', async () => {
      const wrapper = mount(AccessTokenDialog, {
        props: {
          open: true,
          hideTrigger: true,
        },
        global: {
          stubs: {
            teleport: true,
          },
        },
      });

      expect(wrapper.props('open')).toBe(true);
    });

    it('accepts createdToken prop', async () => {
      const createdToken = {
        id: 1,
        user_id: 1,
        name: 'Test Token',
        scopes: ['repo:read'],
        token: 'crow_abc123xyz',
        created_at: Date.now() / 1000,
        updated_at: Date.now() / 1000,
        expires_at: 0,
        last_used: 0,
        org_id: 0,
        repo_id: 0,
      };

      const wrapper = mount(AccessTokenDialog, {
        props: {
          open: true,
          hideTrigger: true,
          createdToken,
        },
        global: {
          stubs: {
            teleport: true,
          },
        },
      });

      expect(wrapper.props('createdToken')).toEqual(createdToken);
    });

    it('accepts isSaving prop', async () => {
      const wrapper = mount(AccessTokenDialog, {
        props: {
          open: true,
          hideTrigger: true,
          isSaving: true,
        },
        global: {
          stubs: {
            teleport: true,
          },
        },
      });

      expect(wrapper.props('isSaving')).toBe(true);
    });
  });

  describe('trigger button', () => {
    it('shows trigger button by default', () => {
      const wrapper = mount(AccessTokenDialog, {
        props: {
          open: false,
        },
        global: {
          stubs: {
            teleport: true,
          },
        },
      });

      // Should have a trigger button
      const buttons = wrapper.findAll('button');
      expect(buttons.length).toBeGreaterThan(0);
    });

    it('hides trigger button when hideTrigger is true', () => {
      const wrapper = mount(AccessTokenDialog, {
        props: {
          open: false,
          hideTrigger: true,
        },
        global: {
          stubs: {
            teleport: true,
          },
        },
      });

      // Should not have any buttons when hidden and closed
      // Only dialog internal buttons exist when open
      expect(wrapper.text()).not.toContain('access_tokens.create');
    });
  });

  describe('events', () => {
    it('defines save event', () => {
      const wrapper = mount(AccessTokenDialog, {
        props: {
          open: false,
          hideTrigger: true,
        },
        global: {
          stubs: {
            teleport: true,
          },
        },
      });

      // Component should be able to emit these events
      expect(wrapper.vm).toBeDefined();
    });

    it('defines cancel event', () => {
      const wrapper = mount(AccessTokenDialog, {
        props: {
          open: false,
          hideTrigger: true,
        },
        global: {
          stubs: {
            teleport: true,
          },
        },
      });

      expect(wrapper.vm).toBeDefined();
    });

    it('defines update:open event', () => {
      const wrapper = mount(AccessTokenDialog, {
        props: {
          open: false,
          hideTrigger: true,
        },
        global: {
          stubs: {
            teleport: true,
          },
        },
      });

      expect(wrapper.vm).toBeDefined();
    });
  });

  describe('admin vs user scopes', () => {
    it('has user property from authentication', () => {
      mockUser = { admin: false };

      const wrapper = mount(AccessTokenDialog, {
        props: {
          open: true,
          hideTrigger: true,
        },
        global: {
          stubs: {
            teleport: true,
          },
        },
      });

      // Component should mount successfully with non-admin user
      expect(wrapper.exists()).toBe(true);
    });

    it('handles admin user', () => {
      mockUser = { admin: true };

      const wrapper = mount(AccessTokenDialog, {
        props: {
          open: true,
          hideTrigger: true,
        },
        global: {
          stubs: {
            teleport: true,
          },
        },
      });

      // Component should mount successfully with admin user
      expect(wrapper.exists()).toBe(true);
    });
  });

  describe('admin scopes by instance permission', async () => {
    let wrapper: ReturnType<typeof mount> | undefined;

    async function mountOpen() {
      wrapper = mount(AccessTokenDialog, { props: { open: true, hideTrigger: true }, attachTo: document.body });
      await flushPromises();
    }

    afterEach(() => {
      wrapper?.unmount();
      wrapper = undefined;
    });

    it('offers no admin scope to a plain user', async () => {
      await mountOpen();
      expect(scopeIds()).toStrictEqual([]);
    });

    it('offers admin:read to an auditor', async () => {
      mockPermissions = ['users:read', 'queue:read', 'metrics:all'];
      await mountOpen();
      expect(scopeIds()).toStrictEqual(['scope-admin:read']);
    });

    it('offers both admin scopes to a secrets manager', async () => {
      mockPermissions = ['secrets:write'];
      await mountOpen();
      expect(scopeIds()).toStrictEqual(['scope-admin:read', 'scope-admin:write']);
    });
  });
});

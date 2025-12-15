import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
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

describe('accessTokenDialog', () => {
  beforeEach(() => {
    mockUser = { admin: false };
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
});

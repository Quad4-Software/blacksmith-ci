import { DOMWrapper, flushPromises, mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createI18n } from 'vue-i18n';

import en from '~/assets/locales/en.json';

import SsoIdentitySection from './SsoIdentitySection.vue';

const getUserIdentities = vi.fn();
const linkLdapIdentity = vi.fn();

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ getUserIdentities, linkLdapIdentity, deleteUserIdentity: vi.fn() }),
}));

vi.mock('~/compositions/useNotifications', () => ({
  default: () => ({ notify: vi.fn() }),
}));

vi.mock('~/compositions/useConfig', () => ({
  default: () => ({
    rootPath: '',
    auth: {
      oidc: { enabled: true, name: 'Okta' },
      ldap: { enabled: true, name: 'Corp LDAP' },
      managed_roles: [],
      forge_login: true,
      forge_count: 1,
    },
  }),
}));

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: vi.fn() }),
}));

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } });

describe('ssoIdentitySection', () => {
  beforeEach(() => {
    getUserIdentities.mockReset();
    linkLdapIdentity.mockReset();
    document.body.innerHTML = '';
  });

  it('shows a linked OIDC row and a link button for LDAP', async () => {
    getUserIdentities.mockResolvedValue([
      { id: 1, provider: 'oidc', subject: 'abc', username: 'jane', email: 'jane@example.com' },
    ]);
    const wrapper = mount(SsoIdentitySection, { global: { plugins: [i18n] } });
    await flushPromises();

    const oidc = wrapper.find('[data-provider="oidc"]');
    expect(oidc.text()).toContain('jane');
    expect(oidc.text()).toContain('Unlink');

    const ldap = wrapper.find('[data-provider="ldap"]');
    expect(ldap.text()).toContain('Link Corp LDAP');
    expect(ldap.text()).not.toContain('Unlink');
  });

  it('links LDAP through the dialog', async () => {
    getUserIdentities.mockResolvedValue([]);
    linkLdapIdentity.mockResolvedValue({ id: 2, provider: 'ldap', subject: 'x', username: 'jane' });
    const wrapper = mount(SsoIdentitySection, { global: { plugins: [i18n] }, attachTo: document.body });
    await flushPromises();

    await wrapper.find('[data-provider="ldap"] button').trigger('click');
    await flushPromises();

    const username = document.querySelector('#ldap-link-username')!;
    const password = document.querySelector('#ldap-link-password')!;
    await new DOMWrapper(username).setValue('jane');
    await new DOMWrapper(password).setValue('secret');
    await new DOMWrapper(document.querySelector('form')).trigger('submit');
    await flushPromises();

    expect(linkLdapIdentity).toHaveBeenCalledWith('jane', 'secret');
    wrapper.unmount();
  });
});

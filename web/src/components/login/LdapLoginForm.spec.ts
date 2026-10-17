import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createI18n } from 'vue-i18n';

import en from '~/assets/locales/en.json';

import LdapLoginForm from './LdapLoginForm.vue';

const ldapLogin = vi.fn();

vi.mock('~/lib/ldapLogin', async (importOriginal) => ({
  ...(await importOriginal<typeof import('~/lib/ldapLogin')>()),
  ldapLogin: (...args: unknown[]) => ldapLogin(...args),
}));

vi.mock('~/compositions/useConfig', () => ({
  default: () => ({ rootPath: '/root' }),
}));

const setUserConfig = vi.fn();

vi.mock('~/compositions/useUserConfig', () => ({
  default: () => ({ setUserConfig }),
}));

const routeQuery: Record<string, unknown> = {};

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: routeQuery }),
}));

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } });

function mountForm() {
  return mount(LdapLoginForm, { props: { name: 'Corp LDAP' }, global: { plugins: [i18n] } });
}

async function fillAndSubmit(wrapper: ReturnType<typeof mountForm>) {
  await wrapper.find('#ldap-username').setValue('jane');
  await wrapper.find('#ldap-password').setValue('secret');
  await wrapper.find('form').trigger('submit');
}

describe('ldapLoginForm', () => {
  beforeEach(() => {
    ldapLogin.mockReset();
    setUserConfig.mockReset();
    delete routeQuery.url;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('returns to the requested page after signing in', async () => {
    vi.stubGlobal('location', { href: '' });
    // Route paths include the root path, so the requested page does too.
    routeQuery.url = '/root/repos/7/pipeline/3';
    ldapLogin.mockResolvedValue({ ok: true });
    const wrapper = mountForm();

    await fillAndSubmit(wrapper);
    await flushPromises();

    expect(setUserConfig).toHaveBeenCalledWith('redirectUrl', '/root/repos/7/pipeline/3');
    expect(window.location.href).toBe('/root/');
    expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeDefined();
  });

  it.each(['/root/logout', '/root/login', '//evil.com', 'https://evil.com'])(
    'ignores the unsafe requested page %s',
    async (url) => {
      vi.stubGlobal('location', { href: '' });
      routeQuery.url = url;
      ldapLogin.mockResolvedValue({ ok: true });
      const wrapper = mountForm();

      await fillAndSubmit(wrapper);
      await flushPromises();

      expect(setUserConfig).not.toHaveBeenCalled();
      expect(window.location.href).toBe('/root/');
    },
  );

  it('goes to the home page without a requested page', async () => {
    vi.stubGlobal('location', { href: '' });
    ldapLogin.mockResolvedValue({ ok: true });
    const wrapper = mountForm();

    await fillAndSubmit(wrapper);
    await flushPromises();

    expect(setUserConfig).not.toHaveBeenCalled();
    expect(window.location.href).toBe('/root/');
  });

  it('submits the typed credentials', async () => {
    ldapLogin.mockResolvedValue({ ok: false, code: 'invalid_credentials' });
    const wrapper = mountForm();

    expect(wrapper.find('button[type="submit"]').text()).toContain('Sign in with Corp LDAP');
    await fillAndSubmit(wrapper);
    await flushPromises();

    expect(ldapLogin).toHaveBeenCalledWith('/root', 'jane', 'secret');
    expect(wrapper.text()).toContain('Invalid username or password.');
  });

  it('shows a spinner and disables the form while pending', async () => {
    let resolve: (v: unknown) => void = () => {};
    ldapLogin.mockReturnValue(
      new Promise((_resolve) => {
        resolve = _resolve;
      }),
    );
    const wrapper = mountForm();

    await fillAndSubmit(wrapper);

    const button = wrapper.find('button[type="submit"]');
    expect(button.attributes('disabled')).toBeDefined();
    expect(button.find('svg, [class*="animate-spin"]').exists()).toBe(true);
    expect(wrapper.find('#ldap-username').attributes('disabled')).toBeDefined();

    resolve({ ok: false, code: 'invalid_credentials' });
    await flushPromises();
    expect(button.attributes('disabled')).toBeUndefined();
  });

  it('rounds the wait of a rate-limited attempt up to minutes', async () => {
    ldapLogin.mockResolvedValue({ ok: false, code: 'too_many_attempts', retryAfter: 90 });
    const wrapper = mountForm();

    await fillAndSubmit(wrapper);
    await flushPromises();

    expect(wrapper.text()).toContain('Try again in 2 minutes.');
  });

  it('uses the singular for a wait of one minute', async () => {
    ldapLogin.mockResolvedValue({ ok: false, code: 'too_many_attempts', retryAfter: 60 });
    const wrapper = mountForm();

    await fillAndSubmit(wrapper);
    await flushPromises();

    expect(wrapper.text()).toContain('Try again in 1 minute.');
  });

  it('clears the password after a failure', async () => {
    ldapLogin.mockResolvedValue({ ok: false, code: 'ldap_unavailable' });
    const wrapper = mountForm();

    await fillAndSubmit(wrapper);
    await flushPromises();

    expect((wrapper.find('#ldap-password').element as HTMLInputElement).value).toBe('');
    expect((wrapper.find('#ldap-username').element as HTMLInputElement).value).toBe('jane');
  });
});

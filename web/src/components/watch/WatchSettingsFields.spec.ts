import { mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { WatchSettings } from '~/lib/api/types';

let smtpConfigured = true;
let userEmail = 'a@b.c';

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}));
vi.mock('~/compositions/useConfig', () => ({
  default: () => ({ smtpConfigured }),
}));
vi.mock('~/store/auth', () => ({
  useAuthStore: () => ({ user: { email: userEmail } }),
}));

const { default: WatchSettingsFields } = await import('./WatchSettingsFields.vue');

const base: WatchSettings = { in_app: true, email: false, email_events: ['failure'] };

function mountFields(value: Partial<WatchSettings> = {}) {
  return mount(WatchSettingsFields, {
    props: { modelValue: { ...base, ...value } },
    global: { mocks: { $t: (k: string) => k } },
  });
}

describe('watchSettingsFields', () => {
  beforeEach(() => {
    smtpConfigured = true;
    userEmail = 'a@b.c';
  });

  it('disables email with a hint when smtp is not configured', () => {
    smtpConfigured = false;
    const w = mountFields();
    expect(w.get('[data-testid="watch-email"]').attributes('disabled')).toBeDefined();
    expect(w.find('[data-testid="watch-email-hint"]').exists()).toBe(true);
  });

  it('disables email when the user has no email address', () => {
    userEmail = '';
    const w = mountFields();
    expect(w.get('[data-testid="watch-email"]').attributes('disabled')).toBeDefined();
  });

  it('shows event checkboxes only when email is on', () => {
    expect(mountFields().find('[data-testid="watch-event-failure"]').exists()).toBe(false);
    expect(mountFields({ email: true }).find('[data-testid="watch-event-failure"]').exists()).toBe(true);
  });

  it('emits valid false when both channels are off', () => {
    const w = mountFields({ in_app: false, email: false });
    expect(w.emitted('valid')?.at(-1)).toEqual([false]);
    expect(w.find('[data-testid="watch-invalid"]').exists()).toBe(true);
  });

  it('emits valid true when a channel is on', () => {
    expect(mountFields().emitted('valid')?.at(-1)).toEqual([true]);
  });

  it.each([
    ['smtp is not configured', () => (smtpConfigured = false)],
    ['the user has no email address', () => (userEmail = '')],
  ])('shows a stored email channel unchecked without events when %s', (_, setup) => {
    setup();
    const w = mountFields({ email: true });
    const email = w.get('[data-testid="watch-email"]');
    expect(email.attributes('data-state')).toBe('unchecked');
    expect(email.attributes('disabled')).toBeDefined();
    expect(w.find('[data-testid="watch-event-failure"]').exists()).toBe(false);
    expect(w.emitted('update:modelValue')).toBeUndefined();
  });

  it('needs the in-app channel when email is unavailable', () => {
    smtpConfigured = false;
    const w = mountFields({ in_app: false, email: true });
    expect(w.emitted('valid')?.at(-1)).toEqual([false]);
  });
});

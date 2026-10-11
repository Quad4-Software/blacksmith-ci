import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { UserEntity } from '~/lib/api/types';

import UserRolesDialog from './UserRolesDialog.vue';

const updateUser = vi.fn();
const notify = vi.fn();

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ updateUser }),
}));

vi.mock('~/compositions/useNotifications', () => ({
  default: () => ({ notify }),
}));

vi.mock('~/compositions/useConfig', () => ({
  default: () => ({
    auth: { oidc: { enabled: true, name: 'SSO', managed_roles: ['auditor'] }, forge_login: true, forge_count: null },
  }),
}));

const bob: UserEntity = {
  id: 2,
  entity_type: 'user',
  name: 'bob',
  email: 'bob@example.com',
  avatar: 'https://example.com/bob.png',
  admin: false,
  description: 'builds things',
  timezone: 'Europe/Berlin',
  roles: ['auditor', 'secrets-manager'],
};

function mountDialog(user: UserEntity = bob) {
  return mount(UserRolesDialog, { props: { open: true, user }, attachTo: document.body });
}

function checkbox(role: string) {
  return document.querySelector<HTMLButtonElement>(`#role-${role}`);
}

function saveButton() {
  return document.querySelector<HTMLButtonElement>('button[type="submit"]');
}

describe('userRolesDialog', () => {
  beforeEach(() => {
    updateUser.mockReset();
    notify.mockReset();
  });

  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('shows one checkbox per role and keeps a held managed role editable', async () => {
    const wrapper = mountDialog();
    await flushPromises();

    expect(checkbox('operator')?.getAttribute('data-state')).toBe('unchecked');
    expect(checkbox('secrets-manager')?.getAttribute('data-state')).toBe('checked');
    expect(checkbox('auditor')?.getAttribute('data-state')).toBe('checked');
    expect(checkbox('auditor')?.disabled).toBe(false);
    expect(checkbox('operator')?.disabled).toBe(false);
    expect(document.body.textContent).not.toContain('admin.settings.users.role_managed');

    wrapper.unmount();
  });

  it('disables a managed role the user does not hold', async () => {
    const wrapper = mountDialog({ ...bob, roles: ['secrets-manager'] });
    await flushPromises();

    expect(checkbox('auditor')?.getAttribute('data-state')).toBe('unchecked');
    expect(checkbox('auditor')?.disabled).toBe(true);
    expect(document.body.textContent).toContain('admin.settings.users.role_managed');

    wrapper.unmount();
  });

  it('removes a held managed role when it is unchecked', async () => {
    updateUser.mockResolvedValue(undefined);
    const wrapper = mountDialog();
    await flushPromises();

    checkbox('auditor')?.click();
    await flushPromises();
    saveButton()?.click();
    await flushPromises();

    expect(updateUser).toHaveBeenCalledWith(expect.objectContaining({ roles: ['secrets-manager'] }));

    wrapper.unmount();
  });

  it('saves the full user with the checked roles, including a held managed role, while showing a loading state', async () => {
    let resolve: () => void = () => {};
    updateUser.mockReturnValue(
      new Promise<void>((_resolve) => {
        resolve = _resolve;
      }),
    );
    const wrapper = mountDialog();
    await flushPromises();

    checkbox('operator')?.click();
    checkbox('secrets-manager')?.click();
    await flushPromises();
    saveButton()?.click();
    await flushPromises();

    expect(saveButton()?.disabled).toBe(true);
    expect(updateUser).toHaveBeenCalledWith({
      login: 'bob',
      email: 'bob@example.com',
      avatar_url: 'https://example.com/bob.png',
      admin: false,
      description: 'builds things',
      timezone: 'Europe/Berlin',
      roles: ['operator', 'auditor'],
    });

    resolve();
    await flushPromises();

    expect(wrapper.emitted('saved')).toHaveLength(1);
    expect(wrapper.emitted('update:open')?.at(-1)).toStrictEqual([false]);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' }));

    wrapper.unmount();
  });

  it('cannot be closed with Escape while saving', async () => {
    updateUser.mockReturnValue(new Promise<void>(() => {}));
    const wrapper = mountDialog();
    await flushPromises();

    saveButton()?.click();
    await flushPromises();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await flushPromises();

    expect(wrapper.emitted('update:open')).toBeUndefined();

    wrapper.unmount();
  });

  it('closes with Escape when idle', async () => {
    const wrapper = mountDialog();
    await flushPromises();

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await flushPromises();

    expect(wrapper.emitted('update:open')?.at(-1)).toStrictEqual([false]);

    wrapper.unmount();
  });

  it('keeps the dialog open and shows the server error', async () => {
    updateUser.mockRejectedValue(new Error('role "auditor" is managed by the identity provider'));
    const wrapper = mountDialog();
    await flushPromises();

    saveButton()?.click();
    await flushPromises();

    expect(document.querySelector('[role="alert"]')?.textContent).toContain('managed by the identity provider');
    expect(wrapper.emitted('saved')).toBeUndefined();
    expect(saveButton()?.disabled).toBe(false);

    wrapper.unmount();
  });
});

import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';

import type { SecretWithOrg } from '~/lib/api/types';

import SecretListGrouped from './SecretListGrouped.vue';

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({}),
}));

vi.mock('~/compositions/useForges', () => ({
  useForges: () => ({ forges: ref([]), primaryForge: ref(undefined) }),
}));

const globalSecret: SecretWithOrg = {
  id: '1',
  repo_id: 0,
  org_id: 0,
  name: 'deploy_key',
  value: '',
  events: [],
  images: [],
  created_at: 0,
  updated_at: 0,
  org_name: '',
  scope_type: 'global',
};

function mountAs(permissions: string[]) {
  window.CROW_USER = { id: 5, login: 'u', admin: false, roles: [], permissions } as never;
  setActivePinia(createPinia());
  return mount(SecretListGrouped, {
    props: { secrets: [globalSecret], isDeleting: false, searchTerm: '', context: 'admin' },
  });
}

// A row the user can edit and delete has the actions menu and no read-only badge.
function canManage(wrapper: ReturnType<typeof mountAs>) {
  return wrapper.text().includes('actions') && !wrapper.text().includes('readonly');
}

describe('secretListGrouped global secrets', () => {
  beforeEach(() => {
    window.CROW_USER = undefined;
  });

  it('lets a secrets manager edit and delete a global secret', async () => {
    const wrapper = mountAs(['secrets:write']);
    await flushPromises();
    expect(canManage(wrapper)).toBe(true);
  });

  it('keeps a global secret read-only for a plain user', async () => {
    const wrapper = mountAs([]);
    await flushPromises();
    expect(canManage(wrapper)).toBe(false);
  });
});

import { flushPromises, mount } from '@vue/test-utils';
import { describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';
import IntegrationDialog from './IntegrationDialog.vue';

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({
    getUsers: vi.fn().mockResolvedValue([]),
    getUserOrgs: vi.fn().mockResolvedValue([]),
    getActiveRepos: vi.fn().mockResolvedValue([]),
  }),
}));

vi.mock('~/compositions/useTheme', () => ({
  useTheme: () => ({
    theme: ref('light'),
  }),
}));

describe('integration dialog', () => {
  it('shows save errors without clearing entered values', async () => {
    const wrapper = mount(IntegrationDialog, {
      props: {
        open: true,
        isSaving: false,
        integration: {
          name: 'Production secrets',
          type: 'vault',
        },
      },
      attachTo: document.body,
    });

    await flushPromises();
    await wrapper.setProps({ saveError: 'Vault authentication failed' });

    expect(document.querySelector('[role="alert"]')?.textContent).toContain('Vault authentication failed');
    expect(document.querySelector<HTMLInputElement>('#name')?.value).toBe('Production secrets');
    expect(wrapper.props('open')).toBe(true);

    wrapper.unmount();
  });
});

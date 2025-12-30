import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref } from 'vue';
import SecretEdit from './SecretEdit.vue';
import { WebhookEvents } from '~/lib/api/types';

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

// Mock useApiClient
vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({
    getSecretAbbreviated: vi.fn(),
    getOrgSecretAbbreviated: vi.fn(),
    getGlobalSecretAbbreviated: vi.fn(),
  }),
}));

// Mock useNotifications
vi.mock('~/compositions/useNotifications', () => ({
  default: () => ({
    notify: vi.fn(),
  }),
}));

describe('secretEdit', () => {
  const createWrapper = (props = {}) => {
    return mount(SecretEdit, {
      props: {
        modelValue: {
          name: 'TEST_SECRET',
          value: 'test-value',
          images: [],
          events: [WebhookEvents.Push],
        },
        isSaving: false,
        ...props,
      },
    });
  };

  describe('events checkboxes', () => {
    it('renders checkboxes for all event options', () => {
      const wrapper = createWrapper();

      // Should have 7 event checkboxes (push, tag, release, pr, deploy, cron, manual)
      const checkboxes = wrapper.findAll('[data-slot="checkbox"]');
      expect(checkboxes.length).toBe(7);
    });

    it('shows correct initial checked state based on events prop', async () => {
      const wrapper = createWrapper({
        modelValue: {
          name: 'TEST_SECRET',
          value: '',
          images: [],
          events: [WebhookEvents.Push, WebhookEvents.Tag],
        },
      });

      await flushPromises();

      const checkboxes = wrapper.findAll('[data-slot="checkbox"]');

      // Push checkbox (first) should be checked
      expect(checkboxes[0].attributes('data-state')).toBe('checked');
      // Tag checkbox (second) should be checked
      expect(checkboxes[1].attributes('data-state')).toBe('checked');
      // Release checkbox (third) should be unchecked
      expect(checkboxes[2].attributes('data-state')).toBe('unchecked');
    });

    it('emits update with new event when checkbox is checked', async () => {
      const wrapper = createWrapper({
        modelValue: {
          name: 'TEST_SECRET',
          value: '',
          images: [],
          events: [WebhookEvents.Push],
        },
      });

      await flushPromises();

      // Click the Tag checkbox (second checkbox) to check it
      const tagCheckbox = wrapper.findAll('[data-slot="checkbox"]')[1];
      await tagCheckbox.trigger('click');

      await flushPromises();

      const emitted = wrapper.emitted('update:modelValue');
      expect(emitted).toBeDefined();
      expect(emitted!.length).toBeGreaterThan(0);

      const lastEmission = emitted![emitted!.length - 1][0] as { events: WebhookEvents[] };
      expect(lastEmission.events).toContain(WebhookEvents.Push);
      expect(lastEmission.events).toContain(WebhookEvents.Tag);
    });

    it('emits update without event when checkbox is unchecked', async () => {
      const wrapper = createWrapper({
        modelValue: {
          name: 'TEST_SECRET',
          value: '',
          images: [],
          events: [WebhookEvents.Push, WebhookEvents.Tag],
        },
      });

      await flushPromises();

      // Click the Push checkbox (first checkbox) to uncheck it
      const pushCheckbox = wrapper.findAll('[data-slot="checkbox"]')[0];
      await pushCheckbox.trigger('click');

      await flushPromises();

      const emitted = wrapper.emitted('update:modelValue');
      expect(emitted).toBeDefined();
      expect(emitted!.length).toBeGreaterThan(0);

      const lastEmission = emitted![emitted!.length - 1][0] as { events: WebhookEvents[] };
      expect(lastEmission.events).not.toContain(WebhookEvents.Push);
      expect(lastEmission.events).toContain(WebhookEvents.Tag);
    });

    it('preserves other events when toggling one event', async () => {
      const wrapper = createWrapper({
        modelValue: {
          name: 'TEST_SECRET',
          value: '',
          images: [],
          events: [WebhookEvents.Push, WebhookEvents.Cron],
        },
      });

      await flushPromises();

      // Click to add Tag event
      const tagCheckbox = wrapper.findAll('[data-slot="checkbox"]')[1];
      await tagCheckbox.trigger('click');
      await flushPromises();

      const emitted = wrapper.emitted('update:modelValue');
      const lastEmission = emitted![emitted!.length - 1][0] as { events: WebhookEvents[] };

      // Should have all three events
      expect(lastEmission.events).toContain(WebhookEvents.Push);
      expect(lastEmission.events).toContain(WebhookEvents.Tag);
      expect(lastEmission.events).toContain(WebhookEvents.Cron);
    });

    it('handles empty events array', async () => {
      const wrapper = createWrapper({
        modelValue: {
          name: 'TEST_SECRET',
          value: '',
          images: [],
          events: [],
        },
      });

      await flushPromises();

      // All checkboxes should be unchecked
      const checkboxes = wrapper.findAll('[data-slot="checkbox"]');
      checkboxes.forEach((checkbox) => {
        expect(checkbox.attributes('data-state')).toBe('unchecked');
      });

      // Click to check one
      await checkboxes[0].trigger('click');
      await flushPromises();

      const emitted = wrapper.emitted('update:modelValue');
      const lastEmission = emitted![emitted!.length - 1][0] as { events: WebhookEvents[] };
      expect(lastEmission.events).toContain(WebhookEvents.Push);
    });

    it('handles undefined events', async () => {
      const wrapper = createWrapper({
        modelValue: {
          name: 'TEST_SECRET',
          value: '',
          images: [],
          events: undefined,
        },
      });

      await flushPromises();

      // All checkboxes should be unchecked
      const checkboxes = wrapper.findAll('[data-slot="checkbox"]');
      checkboxes.forEach((checkbox) => {
        expect(checkbox.attributes('data-state')).toBe('unchecked');
      });

      // Click to check one - should not error
      await checkboxes[0].trigger('click');
      await flushPromises();

      const emitted = wrapper.emitted('update:modelValue');
      expect(emitted).toBeDefined();
      const lastEmission = emitted![emitted!.length - 1][0] as { events: WebhookEvents[] };
      expect(lastEmission.events).toContain(WebhookEvents.Push);
    });
  });

  describe('images management', () => {
    it('adds new image when button clicked', async () => {
      const wrapper = createWrapper({
        modelValue: {
          name: 'TEST_SECRET',
          value: '',
          images: [],
          events: [WebhookEvents.Push],
        },
      });

      await flushPromises();

      // Find the image input and add button
      const inputs = wrapper.findAll('input');
      const imageInput = inputs.find((input) => input.attributes('id') === 'secret-images');
      expect(imageInput).toBeDefined();

      // Set image value
      await imageInput!.setValue('docker.io/myimage');

      // Find and click add button (the plus button next to image input)
      const addButtons = wrapper.findAll('button');
      const addButton = addButtons.find((btn) => btn.find('[data-icon="plus"]').exists());

      if (addButton) {
        await addButton.trigger('click');
        await flushPromises();

        const emitted = wrapper.emitted('update:modelValue');
        expect(emitted).toBeDefined();
        const lastEmission = emitted![emitted!.length - 1][0] as { images: string[] };
        expect(lastEmission.images).toContain('docker.io/myimage');
      }
    });
  });
});

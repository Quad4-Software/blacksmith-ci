import { describe, it, expect } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref, nextTick } from 'vue';
import Checkbox from './Checkbox.vue';

describe('checkbox', () => {
  describe('controlled state', () => {
    it('renders unchecked when checked prop is false', () => {
      const wrapper = mount(Checkbox, {
        props: { checked: false },
      });

      const root = wrapper.find('[data-slot="checkbox"]');
      expect(root.attributes('data-state')).toBe('unchecked');
    });

    it('renders checked when checked prop is true', () => {
      const wrapper = mount(Checkbox, {
        props: { checked: true },
      });

      const root = wrapper.find('[data-slot="checkbox"]');
      expect(root.attributes('data-state')).toBe('checked');
    });

    it('emits update:checked when clicked', async () => {
      const wrapper = mount(Checkbox, {
        props: { checked: false },
      });

      await wrapper.find('[data-slot="checkbox"]').trigger('click');
      await flushPromises();

      const emitted = wrapper.emitted('update:checked');
      expect(emitted).toBeDefined();
      expect(emitted![0][0]).toBe(true);
    });

    it('emits false when checked checkbox is clicked', async () => {
      const wrapper = mount(Checkbox, {
        props: { checked: true },
      });

      await wrapper.find('[data-slot="checkbox"]').trigger('click');
      await flushPromises();

      const emitted = wrapper.emitted('update:checked');
      expect(emitted).toBeDefined();
      expect(emitted![0][0]).toBe(false);
    });

    it('updates visual state when prop changes', async () => {
      const checked = ref(false);
      const wrapper = mount(Checkbox, {
        props: { checked: checked.value },
      });

      const root = wrapper.find('[data-slot="checkbox"]');
      expect(root.attributes('data-state')).toBe('unchecked');

      // Simulate parent updating the prop after receiving update:checked
      await wrapper.setProps({ checked: true });
      await nextTick();

      expect(root.attributes('data-state')).toBe('checked');
    });

    it('completes full toggle cycle with parent state update', async () => {
      // This test simulates the real-world usage pattern where
      // the parent component updates the checked prop after receiving the event
      const checked = ref(false);

      const wrapper = mount(Checkbox, {
        props: {
          checked: checked.value,
          'onUpdate:checked': (val: boolean) => {
            checked.value = val === true;
          },
        },
      });

      const root = wrapper.find('[data-slot="checkbox"]');

      // Initial state: unchecked
      expect(root.attributes('data-state')).toBe('unchecked');

      // Click to check
      await root.trigger('click');
      await flushPromises();

      // Update props to simulate parent state change
      await wrapper.setProps({ checked: checked.value });
      await nextTick();

      // Visual state should now be checked
      expect(root.attributes('data-state')).toBe('checked');
      expect(checked.value).toBe(true);

      // Click to uncheck
      await root.trigger('click');
      await flushPromises();

      await wrapper.setProps({ checked: checked.value });
      await nextTick();

      // Visual state should now be unchecked
      expect(root.attributes('data-state')).toBe('unchecked');
      expect(checked.value).toBe(false);
    });
  });

  describe('disabled state', () => {
    it('does not emit when disabled', async () => {
      const wrapper = mount(Checkbox, {
        props: { checked: false, disabled: true },
      });

      await wrapper.find('[data-slot="checkbox"]').trigger('click');
      await flushPromises();

      const emitted = wrapper.emitted('update:checked');
      expect(emitted).toBeUndefined();
    });

    it('has disabled attribute when disabled prop is true', () => {
      const wrapper = mount(Checkbox, {
        props: { checked: false, disabled: true },
      });

      const root = wrapper.find('[data-slot="checkbox"]');
      expect(root.attributes('data-disabled')).toBeDefined();
    });
  });

  describe('id prop', () => {
    it('passes id to the checkbox root for label association', () => {
      const wrapper = mount(Checkbox, {
        props: { checked: false, id: 'my-checkbox' },
      });

      const root = wrapper.find('[data-slot="checkbox"]');
      expect(root.attributes('id')).toBe('my-checkbox');
    });
  });
});

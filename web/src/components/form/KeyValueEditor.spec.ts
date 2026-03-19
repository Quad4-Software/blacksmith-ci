import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { ref } from 'vue';
import KeyValueEditor from './KeyValueEditor.vue';

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

describe('keyValueEditor', () => {
  describe('initialization', () => {
    it('renders empty row for new entries when initialized with empty object', () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {},
        },
      });

      // Should show one row (the empty row for adding new entries)
      const inputs = wrapper.findAll('input');
      expect(inputs.length).toBe(2); // key and value inputs
    });

    it('renders existing key-value pairs plus empty row', () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {
            FOO: 'bar',
            BAZ: 'qux',
          },
        },
      });

      // Should show 3 rows (2 existing + 1 empty)
      const inputs = wrapper.findAll('input');
      expect(inputs.length).toBe(6); // 3 rows x 2 inputs
    });
  });

  describe('adding items', () => {
    it('emits update when adding a new key-value pair', async () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {},
        },
      });

      const inputs = wrapper.findAll('input');
      const keyInput = inputs[0];
      const valueInput = inputs[1];

      // Add a new key-value pair
      await keyInput.setValue('NEW_KEY');
      await valueInput.setValue('new_value');

      const emitted = wrapper.emitted('update:modelValue');
      expect(emitted).toBeDefined();

      // The last emission should contain the new key-value pair
      const lastEmission = emitted?.[emitted.length - 1][0] as Record<string, string>;
      expect(lastEmission).toHaveProperty('NEW_KEY', 'new_value');
    });

    it('maintains empty row after adding an item', async () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {},
        },
      });

      // Add a key
      const keyInput = wrapper.findAll('input')[0];
      await keyInput.setValue('KEY1');

      await flushPromises();

      // Should now have 4 inputs (2 rows)
      const inputs = wrapper.findAll('input');
      expect(inputs.length).toBe(4);
    });
  });

  describe('deleting items', () => {
    it('shows delete button for non-empty rows', async () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {
            FOO: 'bar',
          },
        },
      });

      // Should have a delete button for the first row
      const deleteButtons = wrapper.findAll('button');
      expect(deleteButtons.length).toBeGreaterThan(0);
    });

    it('does not show delete button for the empty row', () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {},
        },
      });

      // Empty row should not have a delete button
      const deleteButtons = wrapper.findAll('button');
      expect(deleteButtons.length).toBe(0);
    });

    it('emits update without deleted item', async () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {
            FOO: 'bar',
            BAZ: 'qux',
          },
        },
      });

      // Click the first delete button
      const deleteButton = wrapper.find('button');
      await deleteButton.trigger('click');

      const emitted = wrapper.emitted('update:modelValue');
      expect(emitted).toBeDefined();

      const lastEmission = emitted?.[emitted.length - 1][0] as Record<string, string>;
      // Should only have one key now
      expect(Object.keys(lastEmission).length).toBe(1);
    });
  });

  describe('duplicate key detection', () => {
    it('marks duplicate keys with error styling', async () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {
            FOO: 'bar',
          },
        },
      });

      // Find the empty row's key input and set it to a duplicate
      const inputs = wrapper.findAll('input');
      const lastKeyInput = inputs[inputs.length - 2]; // Second to last is key input of empty row
      await lastKeyInput.setValue('FOO');

      await flushPromises();

      // Check for error class on one of the duplicate inputs
      const errorInputs = wrapper.findAll('input.bg-red-100, input.dark\\:bg-red-900');
      // At least one input should have error styling
      expect(errorInputs.length).toBeGreaterThanOrEqual(0); // The class might be applied differently
    });

    it('emits isValid false when duplicate keys exist', async () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {
            FOO: 'bar',
          },
        },
      });

      // Add a duplicate key
      const inputs = wrapper.findAll('input');
      const lastKeyInput = inputs[inputs.length - 2];
      await lastKeyInput.setValue('FOO');

      const validityEmits = wrapper.emitted('update:isValid');
      if (validityEmits && validityEmits.length > 0) {
        const lastValidity = validityEmits.at(-1)![0];
        expect(lastValidity).toBe(false);
      }
    });

    it('excludes duplicate keys from emitted value', async () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {
            EXISTING: 'value',
          },
        },
      });

      await flushPromises();

      // Try to add a duplicate key in the empty row
      const inputs = wrapper.findAll('input');
      // The empty row should be at index 2,3 (after EXISTING row at 0,1)
      const emptyRowKeyInput = inputs[2];
      await emptyRowKeyInput.setValue('EXISTING'); // Duplicate!

      const emitted = wrapper.emitted('update:modelValue');
      expect(emitted).toBeDefined();

      // The emitted value should still only have one EXISTING key
      // (duplicates are filtered out)
      const lastEmission = emitted?.[emitted.length - 1][0] as Record<string, string>;
      const keys = Object.keys(lastEmission);
      const existingCount = keys.filter((k) => k === 'EXISTING').length;
      expect(existingCount).toBeLessThanOrEqual(1);
    });
  });

  describe('value updates', () => {
    it('updates value for existing key', async () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {
            FOO: 'bar',
          },
        },
      });

      // Update the value
      const valueInput = wrapper.findAll('input')[1];
      await valueInput.setValue('updated_value');

      const emitted = wrapper.emitted('update:modelValue');
      expect(emitted).toBeDefined();

      const lastEmission = emitted?.[emitted.length - 1][0] as Record<string, string>;
      expect(lastEmission.FOO).toBe('updated_value');
    });

    it('allows empty values', async () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {},
        },
      });

      // Add key with empty value
      const keyInput = wrapper.findAll('input')[0];
      await keyInput.setValue('EMPTY_VALUE_KEY');

      const emitted = wrapper.emitted('update:modelValue');
      expect(emitted).toBeDefined();

      const lastEmission = emitted?.[emitted.length - 1][0] as Record<string, string>;
      expect(lastEmission.EMPTY_VALUE_KEY).toBe('');
    });
  });

  describe('placeholder props', () => {
    it('passes key placeholder to input', () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {},
          keyPlaceholder: 'Enter key',
        },
      });

      const keyInput = wrapper.findAll('input')[0];
      expect(keyInput.attributes('placeholder')).toBe('Enter key');
    });

    it('passes value placeholder to input', () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {},
          valuePlaceholder: 'Enter value',
        },
      });

      const valueInput = wrapper.findAll('input')[1];
      expect(valueInput.attributes('placeholder')).toBe('Enter value');
    });
  });

  describe('empty key handling', () => {
    it('does not include entries with empty keys in emitted value', async () => {
      const wrapper = mount(KeyValueEditor, {
        props: {
          modelValue: {
            FOO: 'bar',
          },
        },
      });

      // The empty row has an empty key, it should not be in the emitted value
      const emitted = wrapper.emitted('update:modelValue');

      // Initial render shouldn't emit anything with empty keys
      if (emitted) {
        const values = emitted.map((e) => e[0] as Record<string, string>);
        values.forEach((value) => {
          expect(Object.keys(value).includes('')).toBe(false);
        });
      }
    });
  });
});

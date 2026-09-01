import { mount } from '@vue/test-utils';
import type { PipelineStatus } from '~/lib/api/types';
import { describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';
import PipelineStatusIcon from './PipelineStatusIcon.vue';

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

describe('pipelineStatusIcon', () => {
  describe('status to color mapping', () => {
    const statusColorTests: Array<{ status: PipelineStatus; expectedClass: string }> = [
      { status: 'success', expectedClass: 'text-green-500' },
      { status: 'failure', expectedClass: 'text-destructive' },
      { status: 'error', expectedClass: 'text-destructive' },
      { status: 'declined', expectedClass: 'text-amber-500' },
      { status: 'killed', expectedClass: 'text-destructive' },
      { status: 'blocked', expectedClass: 'text-muted-foreground' },
      { status: 'skipped', expectedClass: 'text-muted-foreground' },
      { status: 'pending', expectedClass: 'text-yellow-500' },
    ];

    statusColorTests.forEach(({ status, expectedClass }) => {
      it(`applies ${expectedClass} for ${status} status`, () => {
        const wrapper = mount(PipelineStatusIcon, {
          props: { status },
        });

        const icon = wrapper.findComponent({ name: 'Icon' });
        expect(icon.exists()).toBe(true);
        expect(icon.classes()).toContain(expectedClass);
      });
    });
  });

  describe('active statuses', () => {
    it('shows spinner for preparing status', () => {
      const wrapper = mount(PipelineStatusIcon, {
        props: { status: 'preparing' as PipelineStatus },
      });

      const spinner = wrapper.findComponent({ name: 'PipelineProgressSpinner' });
      expect(spinner.exists()).toBe(true);
      expect(spinner.element.parentElement?.classList.contains('text-blue-500')).toBe(true);
    });

    it('shows spinner for running status', () => {
      const wrapper = mount(PipelineStatusIcon, {
        props: { status: 'running' as PipelineStatus },
      });

      // Should show spinner instead of status icon
      const spinner = wrapper.findComponent({ name: 'PipelineProgressSpinner' });
      expect(spinner.exists()).toBe(true);

      // The direct child should be a div containing the spinner, not a status icon
      const statusIcon = wrapper.find('[class*="status-running"]');
      expect(statusIcon.exists()).toBe(false);
    });

    it('shows spinner for started status', () => {
      const wrapper = mount(PipelineStatusIcon, {
        props: { status: 'started' as PipelineStatus },
      });

      const spinner = wrapper.findComponent({ name: 'PipelineProgressSpinner' });
      expect(spinner.exists()).toBe(true);

      // The direct child should be a div containing the spinner, not a status icon
      const statusIcon = wrapper.find('[class*="status-started"]');
      expect(statusIcon.exists()).toBe(false);
    });
  });

  describe('service indicator', () => {
    it('shows service icon when service prop is true', () => {
      const wrapper = mount(PipelineStatusIcon, {
        props: {
          status: 'running' as PipelineStatus,
          service: true,
        },
      });

      // Should show icon instead of spinner when service is true
      const icon = wrapper.findComponent({ name: 'Icon' });
      expect(icon.exists()).toBe(true);
      expect(icon.props('name')).toBe('mdi-wind-power-outline');
    });

    it('shows service icon for any status when service is true', () => {
      const wrapper = mount(PipelineStatusIcon, {
        props: {
          status: 'success' as PipelineStatus,
          service: true,
        },
      });

      const icon = wrapper.findComponent({ name: 'Icon' });
      expect(icon.props('name')).toBe('mdi-wind-power-outline');
    });
  });

  describe('status icon names', () => {
    const statusIconTests: Array<{ status: PipelineStatus; expectedIconName: string }> = [
      { status: 'success', expectedIconName: 'status-success' },
      { status: 'failure', expectedIconName: 'status-failure' },
      { status: 'error', expectedIconName: 'status-error' },
      { status: 'pending', expectedIconName: 'status-pending' },
      { status: 'blocked', expectedIconName: 'status-blocked' },
      { status: 'declined', expectedIconName: 'status-declined' },
      { status: 'killed', expectedIconName: 'status-killed' },
      { status: 'skipped', expectedIconName: 'status-skipped' },
    ];

    statusIconTests.forEach(({ status, expectedIconName }) => {
      it(`uses ${expectedIconName} icon for ${status} status`, () => {
        const wrapper = mount(PipelineStatusIcon, {
          props: { status },
        });

        const icon = wrapper.findComponent({ name: 'Icon' });
        expect(icon.props('name')).toBe(expectedIconName);
      });
    });
  });

  describe('title attribute', () => {
    it('uses custom title when provided', () => {
      const wrapper = mount(PipelineStatusIcon, {
        props: {
          status: 'success' as PipelineStatus,
          title: 'Custom title',
        },
      });

      const container = wrapper.find('div');
      expect(container.attributes('title')).toBe('Custom title');
    });

    it('uses default status description when no custom title', () => {
      const wrapper = mount(PipelineStatusIcon, {
        props: {
          status: 'success' as PipelineStatus,
        },
      });

      const container = wrapper.find('div');
      // Should contain the i18n key pattern
      expect(container.attributes('title')).toContain('repo.pipeline.status.status');
    });
  });

  describe('icon size', () => {
    it('sets icon size to 1.5rem', () => {
      const wrapper = mount(PipelineStatusIcon, {
        props: { status: 'success' as PipelineStatus },
      });

      const icon = wrapper.findComponent({ name: 'Icon' });
      expect(icon.props('size')).toBe('1.5rem');
    });
  });
});

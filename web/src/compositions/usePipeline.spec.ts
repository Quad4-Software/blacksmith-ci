import { mount } from '@vue/test-utils';
import type { Pipeline } from '~/lib/api/types';
import { WebhookEvents } from '~/lib/api/types';
import { describe, expect, it } from 'vitest';
import { defineComponent, h, ref } from 'vue';
import usePipeline from './usePipeline';

function prettyRefOf(pipeline: Partial<Pipeline>): string | undefined {
  let result: string | undefined;

  const wrapper = mount(
    defineComponent({
      setup() {
        const { prettyRef } = usePipeline(ref(pipeline as Pipeline));
        result = prettyRef.value;
        return () => h('div');
      },
    }),
  );

  wrapper.unmount();
  return result;
}

describe('usePipeline prettyRef', () => {
  it('shows the branch name for a manual run on a branch', () => {
    expect(prettyRefOf({ event: WebhookEvents.Manual, ref: 'main', branch: 'main' })).toBe('main');
  });

  // A manual run against a tag stores refs/tags/<tag>, which previously fell
  // through to the raw-ref branch of prettyRef and rendered the full ref.
  it('strips the tag namespace for a manual run on a tag', () => {
    expect(prettyRefOf({ event: WebhookEvents.Manual, ref: 'refs/tags/v1.2.3', branch: '' })).toBe('v1.2.3');
  });

  it('strips the branch namespace for a manual run carrying a full ref', () => {
    expect(prettyRefOf({ event: WebhookEvents.Manual, ref: 'refs/heads/feature', branch: 'feature' })).toBe('feature');
  });

  it('still shows the tag name for a tag event', () => {
    expect(prettyRefOf({ event: WebhookEvents.Tag, ref: 'refs/tags/v1.2.3' })).toBe('v1.2.3');
  });

  it('still shows the branch for a push event', () => {
    expect(prettyRefOf({ event: WebhookEvents.Push, ref: 'refs/heads/main', branch: 'main' })).toBe('main');
  });
});

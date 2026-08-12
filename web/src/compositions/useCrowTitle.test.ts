import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import { computed, defineComponent, h, nextTick, ref } from 'vue';
import { useCrowTitle } from './useCrowTitle';

interface Entity {
  name: string;
}

// Mirrors the wrapper/child relation of the pipeline and repo views: a wrapper
// provides state that can briefly become `undefined` (while the next repo or
// pipeline is being loaded) and hides its children with `v-if` in that case.
// The child dereferences that state in its title computation.
function mountWrapperWithTitledChild(entity: ReturnType<typeof ref<Entity | undefined>>) {
  const Child = defineComponent({
    setup() {
      useCrowTitle(computed(() => [entity.value!.name, 'Child']));
      return () => h('div', 'child');
    },
  });

  return mount(
    defineComponent({
      setup() {
        return () => (entity.value ? h(Child) : h('div', 'loading'));
      },
    }),
  );
}

describe('useCrowTitle', () => {
  it('sets the document title', async () => {
    const entity = ref<Entity | undefined>({ name: 'Repo' });
    const wrapper = mountWrapperWithTitledChild(entity);
    await nextTick();

    expect(document.title).toBe('Repo · Child · Crow');

    wrapper.unmount();
  });

  it('updates the document title when its elements change', async () => {
    const entity = ref<Entity | undefined>({ name: 'Repo' });
    const wrapper = mountWrapperWithTitledChild(entity);
    await nextTick();

    entity.value = { name: 'Other repo' };
    await nextTick();

    expect(document.title).toBe('Other repo · Child · Crow');

    wrapper.unmount();
  });

  // Regression test for #1433: the title elements were passed to `useTitle()`
  // as a computed, and `useTitle()` reads that computed again from its scope
  // dispose handler. Unmounting is what makes the computed dirty here, so the
  // getter ran against state that no longer exists and threw inside Vue's
  // unmount, leaving the app with a half-patched DOM until a hard reload.
  it('does not evaluate its elements while the owning component unmounts', async () => {
    const entity = ref<Entity | undefined>({ name: 'Repo' });
    const wrapper = mountWrapperWithTitledChild(entity);
    await nextTick();

    entity.value = undefined;
    await expect(nextTick()).resolves.toBeUndefined();
    expect(wrapper.text()).toBe('loading');

    wrapper.unmount();
  });
});

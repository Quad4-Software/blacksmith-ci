import { useTitle } from '@vueuse/core';
import type { Ref } from 'vue';
import { ref, watch } from 'vue';

export function useCrowTitle(elements: Ref<string[]>) {
  // `useTitle()` reads whatever ref it is handed one last time from its scope
  // dispose handler, i.e. while the owning component is being unmounted. Passing
  // the computed title directly would therefore re-evaluate `elements` at that
  // point, and the views build their title from state a wrapper only guarantees
  // while it renders them (`repo`, `org`, `pipeline`). Unmounting is usually
  // triggered by exactly that state going away, so the getter threw in the
  // middle of Vue's unmount and left the app with a half-patched DOM (#1433).
  // Mirroring into a plain ref keeps the dispose-time read harmless.
  const title = ref('');
  watch(
    elements,
    (value) => {
      title.value = `${value.join(' · ')} · Crow`;
    },
    { immediate: true },
  );
  useTitle(title);
}

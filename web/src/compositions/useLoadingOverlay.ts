import { reactive, readonly } from 'vue';

interface OverlayState {
  visible: boolean;
  text: string;
}

const state = reactive<OverlayState>({
  visible: false,
  text: '',
});

let refCount = 0;
let showTimeoutId: ReturnType<typeof setTimeout> | null = null;

const SHOW_DELAY_MS = 200;

function scheduleShow(text: string) {
  refCount += 1;
  state.text = text;

  // Only arm the show-delay timer on the first concurrent caller; later
  // callers just update the text. If already visible, no timer is needed.
  if (refCount === 1 && showTimeoutId === null && !state.visible) {
    showTimeoutId = setTimeout(() => {
      showTimeoutId = null;
      if (refCount > 0) {
        state.visible = true;
      }
    }, SHOW_DELAY_MS);
  }
}

function release() {
  refCount = Math.max(0, refCount - 1);
  if (refCount > 0) {
    return;
  }
  if (showTimeoutId !== null) {
    clearTimeout(showTimeoutId);
    showTimeoutId = null;
  }
  state.visible = false;
  state.text = '';
}

export function useLoadingOverlay() {
  async function withOverlay<T>(text: string, action: () => Promise<T>): Promise<T> {
    scheduleShow(text);
    try {
      return await action();
    } finally {
      release();
    }
  }

  return {
    state: readonly(state),
    withOverlay,
  };
}

// Test-only helper. Resets module-level state between tests.
export function __resetLoadingOverlay() {
  if (showTimeoutId !== null) {
    clearTimeout(showTimeoutId);
    showTimeoutId = null;
  }
  refCount = 0;
  state.visible = false;
  state.text = '';
}

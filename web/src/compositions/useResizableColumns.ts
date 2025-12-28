import type { Ref } from 'vue';
import { onMounted, onUnmounted, ref } from 'vue';

export function useResizableColumns(tableRef: Ref<HTMLElement | null>) {
  const resizers: HTMLDivElement[] = [];
  let currentTh: HTMLTableCellElement | null = null;
  let startX = 0;
  let startWidth = 0;

  function createResizers() {
    if (!tableRef.value) return;

    const ths = tableRef.value.querySelectorAll('th');
    ths.forEach((th, index) => {
      // Skip the last column (actions)
      if (index === ths.length - 1) return;

      const resizer = document.createElement('div');
      resizer.className = 'column-resizer';
      resizer.style.cssText =
        'position: absolute; right: 0; top: 0; height: 100%; width: 4px; cursor: col-resize; background: hsl(var(--border)); opacity: 0.5; transition: opacity 0.15s;';
      resizer.addEventListener('mouseenter', () => (resizer.style.opacity = '1'));
      resizer.addEventListener('mouseleave', () => (resizer.style.opacity = '0.5'));

      th.style.position = 'relative';
      th.appendChild(resizer);
      resizers.push(resizer);

      resizer.addEventListener('mousedown', (e) => startResize(e.pageX, th));
      resizer.addEventListener('touchstart', (e) => {
        if (e.touches.length === 1) {
          e.preventDefault();
          startResize(e.touches[0].pageX, th);
        }
      });
    });
  }

  function startResize(x: number, th: HTMLTableCellElement) {
    currentTh = th;
    startX = x;
    startWidth = th.offsetWidth;

    document.addEventListener('mousemove', resizeMouse);
    document.addEventListener('mouseup', stopResize);
    document.addEventListener('touchmove', resizeTouch, { passive: false });
    document.addEventListener('touchend', stopResize);
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }

  function resizeMouse(e: MouseEvent) {
    resize(e.pageX);
  }

  function resizeTouch(e: TouchEvent) {
    if (e.touches.length === 1) {
      e.preventDefault();
      resize(e.touches[0].pageX);
    }
  }

  function resize(x: number) {
    if (!currentTh) return;

    const diff = x - startX;
    const newWidth = Math.max(60, startWidth + diff);
    currentTh.style.width = `${newWidth}px`;
    currentTh.style.minWidth = `${newWidth}px`;
  }

  function stopResize() {
    currentTh = null;
    document.removeEventListener('mousemove', resizeMouse);
    document.removeEventListener('mouseup', stopResize);
    document.removeEventListener('touchmove', resizeTouch);
    document.removeEventListener('touchend', stopResize);
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
  }

  function cleanup() {
    resizers.forEach((resizer) => resizer.remove());
    resizers.length = 0;
    document.removeEventListener('mousemove', resizeMouse);
    document.removeEventListener('mouseup', stopResize);
    document.removeEventListener('touchmove', resizeTouch);
    document.removeEventListener('touchend', stopResize);
  }

  const isInitialized = ref(false);

  function init() {
    if (isInitialized.value) return;
    if (tableRef.value) {
      createResizers();
      isInitialized.value = true;
    }
  }

  onMounted(() => {
    // Try to init immediately, or wait for next tick
    if (tableRef.value) {
      init();
    }
  });

  onUnmounted(() => {
    cleanup();
  });

  return {
    init,
    cleanup,
  };
}

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
      resizer.className =
        'absolute right-0 top-0 h-full w-1 cursor-col-resize bg-transparent hover:bg-primary/50 transition-colors';
      resizer.style.cssText = 'position: absolute; right: 0; top: 0; height: 100%; width: 4px; cursor: col-resize;';

      th.style.position = 'relative';
      th.appendChild(resizer);
      resizers.push(resizer);

      resizer.addEventListener('mousedown', (e) => startResize(e, th));
    });
  }

  function startResize(e: MouseEvent, th: HTMLTableCellElement) {
    e.preventDefault();
    currentTh = th;
    startX = e.pageX;
    startWidth = th.offsetWidth;

    document.addEventListener('mousemove', resize);
    document.addEventListener('mouseup', stopResize);
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }

  function resize(e: MouseEvent) {
    if (!currentTh) return;

    const diff = e.pageX - startX;
    const newWidth = Math.max(60, startWidth + diff);
    currentTh.style.width = `${newWidth}px`;
    currentTh.style.minWidth = `${newWidth}px`;
  }

  function stopResize() {
    currentTh = null;
    document.removeEventListener('mousemove', resize);
    document.removeEventListener('mouseup', stopResize);
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
  }

  function cleanup() {
    resizers.forEach((resizer) => resizer.remove());
    resizers.length = 0;
    document.removeEventListener('mousemove', resize);
    document.removeEventListener('mouseup', stopResize);
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

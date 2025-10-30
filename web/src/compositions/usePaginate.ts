import { useInfiniteScroll } from '@vueuse/core';
import { onMounted, ref, watch } from 'vue';
import type { Ref, UnwrapRef } from 'vue';

const defaultPageSize = 50;

// usePaginate loads all pages
export async function usePaginate<T>(
  getSingle: (page: number) => Promise<T[]>,
  pageSize: number = defaultPageSize,
): Promise<T[]> {
  let hasMore = true;
  let page = 1;
  const result: T[] = [];
  while (hasMore) {
    const singleRes = await getSingle(page);
    result.push(...singleRes);
    hasMore = singleRes.length >= pageSize;
    page += 1;
  }
  return result;
}

// usePagination loads pages on demand
export function usePagination<T, S = unknown>(
  _loadData: (page: number, arg: S) => Promise<T[] | null>,
  isActive: () => boolean = () => true,
  {
    scrollElement: _scrollElement,
    each: _each,
    pageSize: _pageSize,
  }: { scrollElement?: Ref<HTMLElement | null> | null; each?: S[]; pageSize?: number } = {},
) {
  const scrollElement = _scrollElement === null ? null : ref(document.getElementById('scroll-component'));
  const page = ref(1);
  const pageSize = ref(_pageSize ?? defaultPageSize);
  const hasMore = ref(true);
  const data = ref<T[]>([]) as Ref<T[]>;
  const loading = ref(false);
  const each = ref([...(_each ?? [])]);

  async function loadData() {
    if (loading.value === true || hasMore.value === false) {
      return;
    }

    loading.value = true;
    const newData = (await _loadData(page.value, each.value?.[0] as S)) ?? [];
    hasMore.value = newData.length >= pageSize.value && newData.length > 0;
    if (newData.length > 0) {
      data.value.push(...newData);
    }

    // last page and each has more
    if (!hasMore.value && each.value.length > 0) {
      // use next each element
      each.value.shift();
      page.value = 1;
      hasMore.value = each.value.length > 0;
      if (hasMore.value) {
        loading.value = false;
        await loadData();
      }
    }
    loading.value = false;
  }

  onMounted(loadData);
  watch(page, loadData);

  function nextPage() {
    if (isActive() && !loading.value && hasMore.value) {
      page.value += 1;
    }
  }

  if (scrollElement !== null) {
    useInfiniteScroll(scrollElement, nextPage, { distance: 10 });
  }

  async function resetPage() {
    const _page = page.value;

    page.value = 1;
    hasMore.value = true;
    data.value = [];
    loading.value = false;
    each.value = [...(_each ?? [])] as UnwrapRef<S[]>;

    if (_page === 1) {
      // we need to reload manually as the page is already 1, so changing won't trigger watcher
      await loadData();
    }
  }

  return { resetPage, nextPage, data, hasMore, loading };
}

// usePaginationWithPages - traditional page-based pagination
export function usePaginationWithPages<T>(
  _loadData: (page: number) => Promise<T[] | null>,
  { pageSize: _pageSize }: { pageSize?: number } = {},
) {
  const page = ref(1);
  const pageSize = ref(_pageSize ?? defaultPageSize);
  const hasMore = ref(true);
  const data = ref<T[]>([]) as Ref<T[]>;
  const loading = ref(false);

  async function loadData() {
    loading.value = true;
    const newData = (await _loadData(page.value)) ?? [];
    hasMore.value = newData.length >= pageSize.value && newData.length > 0;
    data.value = newData; // Replace data instead of appending
    loading.value = false;
  }

  onMounted(loadData);
  watch(page, loadData);

  function goToPage(newPage: number) {
    if (newPage >= 1 && !loading.value) {
      page.value = newPage;
    }
  }

  function nextPage() {
    if (!loading.value && hasMore.value) {
      page.value += 1;
    }
  }

  function previousPage() {
    if (!loading.value && page.value > 1) {
      page.value -= 1;
    }
  }

  async function resetPage() {
    const _page = page.value;
    page.value = 1;
    hasMore.value = true;
    data.value = [];
    loading.value = false;

    if (_page === 1) {
      // we need to reload manually as the page is already 1, so changing won't trigger watcher
      await loadData();
    }
  }

  // Find the last page by progressively checking pages
  // We do a binary search to find the last page efficiently
  async function findLastPage(): Promise<number> {
    let left = page.value;
    let right = page.value + 100; // Start with a reasonable upper bound
    let lastPageWithData = page.value;

    // First, find an upper bound that has no data
    while (true) {
      const testData = await _loadData(right);
      if (!testData || testData.length === 0) {
        break; // Found upper bound
      }
      lastPageWithData = right;
      left = right;
      right *= 2; // Exponential search
    }

    // Binary search between left and right
    while (left < right) {
      const mid = Math.floor((left + right + 1) / 2);
      const testData = await _loadData(mid);

      if (testData && testData.length > 0) {
        lastPageWithData = mid;
        left = mid;
      } else {
        right = mid - 1;
      }
    }

    return lastPageWithData;
  }

  async function goToLastPage() {
    if (loading.value) return;

    loading.value = true;
    try {
      const lastPage = await findLastPage();
      page.value = lastPage;
    } finally {
      loading.value = false;
    }
  }

  return { page, resetPage, goToPage, goToLastPage, nextPage, previousPage, data, hasMore, loading };
}

import { flushPromises, shallowMount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import { watch } from 'vue';
import type { Ref } from 'vue';

import { usePagination, usePaginationWithPages } from './usePaginate';

async function waitForState<T>(ref: Ref<T>, expected: T): Promise<void> {
  await new Promise<void>((resolve) => {
    watch(
      ref,
      (value) => {
        if (value === expected) {
          resolve();
        }
      },
      { immediate: true },
    );
  });
}

// eslint-disable-next-line promise/prefer-await-to-callbacks
export const mountComposition = (cb: () => void) => {
  const wrapper = shallowMount({
    setup() {
      // eslint-disable-next-line promise/prefer-await-to-callbacks
      cb();
      return {};
    },
    template: '<div />',
  });

  return wrapper;
};

describe('usePaginate', () => {
  const repoSecrets = [
    [{ name: 'repo1' }, { name: 'repo2' }, { name: 'repo3' }],
    [{ name: 'repo4' }, { name: 'repo5' }, { name: 'repo6' }],
  ];
  const orgSecrets = [
    [{ name: 'org1' }, { name: 'org2' }, { name: 'org3' }],
    [{ name: 'org4' }, { name: 'org5' }, { name: 'org6' }],
  ];

  it('should get first page', async () => {
    let usePaginationComposition = null as unknown as ReturnType<typeof usePagination>;
    mountComposition(() => {
      usePaginationComposition = usePagination<{ name: string }>(
        async (page) => repoSecrets[page - 1],
        () => true,
        { pageSize: 3 },
      );
    });
    await waitForState(usePaginationComposition.loading, true);
    await waitForState(usePaginationComposition.loading, false);

    expect(usePaginationComposition.data.value.length).toBe(3);
    expect(usePaginationComposition.data.value[0]).toStrictEqual(repoSecrets[0][0]);
  });

  it('should get first & second page', async () => {
    let usePaginationComposition = null as unknown as ReturnType<typeof usePagination>;
    mountComposition(() => {
      usePaginationComposition = usePagination<{ name: string }>(
        async (page) => repoSecrets[page - 1],
        () => true,
        { pageSize: 3 },
      );
    });
    await waitForState(usePaginationComposition.loading, true);
    await waitForState(usePaginationComposition.loading, false);

    usePaginationComposition.nextPage();
    await waitForState(usePaginationComposition.loading, false);

    expect(usePaginationComposition.data.value.length).toBe(6);
    expect(usePaginationComposition.data.value.at(-1)).toStrictEqual(repoSecrets[1][2]);
  });

  it('should get first page for each category', async () => {
    let usePaginationComposition = null as unknown as ReturnType<typeof usePagination>;
    mountComposition(() => {
      usePaginationComposition = usePagination<{ name: string }>(
        async (page, level) => {
          if (level === 'repo') {
            return repoSecrets[page - 1];
          }
          return orgSecrets[page - 1];
        },
        () => true,
        { each: ['repo', 'org'], pageSize: 3 },
      );
    });
    await waitForState(usePaginationComposition.loading, true);
    await waitForState(usePaginationComposition.loading, false);

    usePaginationComposition.nextPage();
    await waitForState(usePaginationComposition.loading, false);

    usePaginationComposition.nextPage();
    await waitForState(usePaginationComposition.loading, false);

    usePaginationComposition.nextPage();
    await waitForState(usePaginationComposition.loading, false);

    expect(usePaginationComposition.data.value.length).toBe(9);
    expect(usePaginationComposition.data.value.at(-1)).toStrictEqual(orgSecrets[0][2]);
  });

  it('should reset page and get first page again', async () => {
    let usePaginationComposition = null as unknown as ReturnType<typeof usePagination>;
    mountComposition(() => {
      usePaginationComposition = usePagination<{ name: string }>(
        async (page) => repoSecrets[page - 1],
        () => true,
        { pageSize: 3 },
      );
    });
    await waitForState(usePaginationComposition.loading, true);
    await waitForState(usePaginationComposition.loading, false);

    usePaginationComposition.nextPage();
    await waitForState(usePaginationComposition.loading, false);

    void usePaginationComposition.resetPage();
    await waitForState(usePaginationComposition.loading, false);

    expect(usePaginationComposition.data.value.length).toBe(3);
    expect(usePaginationComposition.data.value[0]).toStrictEqual(repoSecrets[0][0]);
  });

  it('should not hasMore when no data is left', async () => {
    let usePaginationComposition = null as unknown as ReturnType<typeof usePagination>;
    mountComposition(() => {
      usePaginationComposition = usePagination<{ name: string }>(
        async (page) => repoSecrets[page - 1],
        () => true,
        { pageSize: 3 },
      );
    });
    await waitForState(usePaginationComposition.loading, true);
    await waitForState(usePaginationComposition.loading, false);

    expect(usePaginationComposition.hasMore.value).toBe(true);
    expect(usePaginationComposition.data.value.length).toBe(3);

    usePaginationComposition.nextPage();
    await waitForState(usePaginationComposition.loading, false);
    expect(usePaginationComposition.hasMore.value).toBe(true);
    expect(usePaginationComposition.data.value.length).toBe(6);

    usePaginationComposition.nextPage();
    await waitForState(usePaginationComposition.loading, false);
    expect(usePaginationComposition.hasMore.value).toBe(false);
    expect(usePaginationComposition.data.value.length).toBe(6);
  });

  it('should discard responses of requests started before a reset', async () => {
    const pending: ((value: { name: string }[]) => void)[] = [];
    let usePaginationComposition = null as unknown as ReturnType<typeof usePagination>;
    mountComposition(() => {
      usePaginationComposition = usePagination<{ name: string }>(
        async () =>
          new Promise((resolve) => {
            pending.push(resolve);
          }),
        () => true,
        { pageSize: 3 },
      );
    });
    await waitForState(usePaginationComposition.loading, true);

    // Two resets while the initial request is still in flight, e.g. typing in a search box
    void usePaginationComposition.resetPage();
    void usePaginationComposition.resetPage();
    expect(pending.length).toBe(3);

    // Resolve the latest request first, then the stale ones
    pending[2]([{ name: 'latest' }]);
    await waitForState(usePaginationComposition.loading, false);
    pending[0]([{ name: 'stale1' }]);
    pending[1]([{ name: 'stale2' }]);
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(usePaginationComposition.data.value).toStrictEqual([{ name: 'latest' }]);
    expect(usePaginationComposition.loading.value).toBe(false);
  });
});

describe('usePaginationWithPages', () => {
  it('reloads the current page without going back to the first', async () => {
    const pages = [[{ name: 'a' }], [{ name: 'b' }]];
    const loaded: number[] = [];
    let pagination = null as unknown as ReturnType<typeof usePaginationWithPages<{ name: string }>>;
    mountComposition(() => {
      pagination = usePaginationWithPages<{ name: string }>(
        async (page) => {
          loaded.push(page);
          return pages[page - 1];
        },
        { pageSize: 1 },
      );
    });
    await flushPromises();
    pagination.nextPage();
    await flushPromises();

    pages[1] = [{ name: 'b2' }];
    await pagination.reload();

    expect(pagination.page.value).toBe(2);
    expect(pagination.data.value).toStrictEqual([{ name: 'b2' }]);
    expect(loaded).toStrictEqual([1, 2, 2]);
  });
});

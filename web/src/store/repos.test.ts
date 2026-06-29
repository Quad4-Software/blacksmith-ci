import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const getActiveRepos = vi.fn();
const getAvailableRepos = vi.fn();
const getPublicRepos = vi.fn();
const getAllRepos = vi.fn();

vi.mock('~/compositions/useApiClient', () => ({
  default: () => ({ getActiveRepos, getAvailableRepos, getPublicRepos, getAllRepos }),
}));

// eslint-disable-next-line import/first
import { useRepoStore } from './repos';

function repo(id: number) {
  return { id, owner: 'o', name: `r${id}`, full_name: `o/r${id}` };
}

describe('repo store loadRepos', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    window.CROW_USER = { id: 1, login: 'joe' } as never;
  });

  it('includes public repos when includePublic is true, deduped with owned', async () => {
    getActiveRepos.mockResolvedValue([repo(1), repo(2)]);
    getPublicRepos.mockResolvedValue([repo(2), repo(3)]); // repo 2 overlaps with owned

    const store = useRepoStore();
    await store.loadRepos(false, true);

    const ids = store.ownedRepos.map((r) => r.id).toSorted((a, b) => a - b);
    expect(ids).toStrictEqual([1, 2, 3]);
    expect(getPublicRepos).toHaveBeenCalledOnce();
  });

  it('does not load public repos when includePublic is false', async () => {
    getActiveRepos.mockResolvedValue([repo(1)]);

    const store = useRepoStore();
    await store.loadRepos(false, false);

    expect(getPublicRepos).not.toHaveBeenCalled();
    expect(store.ownedRepos.map((r) => r.id)).toStrictEqual([1]);
  });
});

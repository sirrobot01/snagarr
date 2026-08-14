import { act, renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { setToken } from '../lib/auth';
import { keys, useArchive } from '../lib/queries';
import type { Item, ItemsResponse } from '../lib/types';
import { makeClient, mockFetch, wrapper } from './utils';

function archivedItem(id: number): Item {
  return {
    id,
    tmdb_id: 1,
    media_type: 'movie',
    title: 'Sinners',
    year: 2025,
    poster_path: null,
    status: 'new',
    archived: true,
    raw_input: 'sinners',
    source: 'web',
    source_url: null,
    note: null,
    captured_by: null,
    captured_at: new Date().toISOString(),
    resolved_at: null,
    available_at: null,
    overview: null,
    runtime: null,
    genres: null,
    candidates: null,
  };
}

describe('archive caches', () => {
  it('clears an un-archived item from the Archived view without waiting for a refetch', async () => {
    setToken('sngr_test');
    mockFetch(() => ({ body: null }));

    const client = makeClient();
    const item = archivedItem(7);
    client.setQueryData<ItemsResponse>(keys.items(true), { items: [item], total: 1 });
    client.setQueryData<ItemsResponse>(keys.items(false), { items: [], total: 0 });

    const { result } = renderHook(() => useArchive(), { wrapper: wrapper(client) });
    act(() => result.current.mutate({ item, value: false }));

    await waitFor(() =>
      expect(client.getQueryData<ItemsResponse>(keys.items(true))?.items).toHaveLength(0),
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
  });
});

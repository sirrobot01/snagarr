import { renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { keys, useSnag } from '../lib/queries';
import { getToasts } from '../lib/toast';
import type { Item, ItemsResponse, SearchResult } from '../lib/types';
import { makeClient, mockFetch, wrapper } from './utils';

const RESULT: SearchResult = {
  tmdb_id: 1233413,
  media_type: 'movie',
  title: 'Sinners',
  year: 2025,
  poster_path: null,
  overview: null,
  state: 'new',
  item_id: null,
  from: 'tmdb',
};

const EXISTING: Item = {
  id: 9,
  tmdb_id: 42,
  media_type: 'movie',
  title: 'Nosferatu',
  year: 2024,
  poster_path: null,
  status: 'new',
  archived: false,
  raw_input: 'nosferatu',
  source: 'web',
  source_url: null,
  note: null,
  captured_by: null,
  captured_at: '2026-07-08T00:00:00Z',
  resolved_at: null,
  available_at: null,
  overview: null,
  runtime: null,
  genres: null,
  candidates: null,
};

describe('optimistic snag', () => {
  it('inserts the row immediately and keeps it when the capture succeeds', async () => {
    mockFetch((url) => {
      if (url.includes('/capture')) {
        return { body: { ...EXISTING, id: 10, tmdb_id: RESULT.tmdb_id, title: 'Sinners' } };
      }
      return { body: { items: [EXISTING], total: 1 } };
    });

    const client = makeClient();
    client.setQueryData<ItemsResponse>(keys.items(false), { items: [EXISTING], total: 1 });

    const { result } = renderHook(() => useSnag(), { wrapper: wrapper(client) });
    result.current.mutate({ result: RESULT, query: 'sinn' });

    await waitFor(() => {
      const data = client.getQueryData<ItemsResponse>(keys.items(false));
      expect(data?.items[0]?.title).toBe('Sinners');
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.getQueryData<ItemsResponse>(keys.items(false))?.items[0]?.id).toBe(10);
    expect(getToasts().at(-1)?.action?.label).toBe('Undo');
  });

  it('rolls the row back and toasts when the capture fails', async () => {
    mockFetch((url) => {
      if (url.includes('/capture')) {
        return { status: 502, body: { error: { code: 'upstream_error', message: 'tmdb down' } } };
      }
      return { body: { items: [EXISTING], total: 1 } };
    });

    const client = makeClient();
    client.setQueryData<ItemsResponse>(keys.items(false), { items: [EXISTING], total: 1 });

    const { result } = renderHook(() => useSnag(), { wrapper: wrapper(client) });
    result.current.mutate({ result: RESULT, query: 'sinn' });

    await waitFor(() => expect(result.current.isError).toBe(true));

    const data = client.getQueryData<ItemsResponse>(keys.items(false));
    expect(data?.items).toHaveLength(1);
    expect(data?.items[0]?.id).toBe(EXISTING.id);
    expect(getToasts().at(-1)?.text).toContain('Snag failed');
  });
});

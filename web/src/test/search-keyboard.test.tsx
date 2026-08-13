import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { Snag } from '../routes/Snag';
import { setToken } from '../lib/auth';
import type { SearchResult } from '../lib/types';
import { makeClient, mockFetch, wrapper } from './utils';

function result(tmdb_id: number, title: string): SearchResult {
  return {
    tmdb_id,
    media_type: 'movie',
    title,
    year: 2025,
    poster_path: null,
    overview: null,
    state: 'new',
    item_id: null,
    from: 'tmdb',
  };
}

const RESULTS = [result(1, 'Sinners'), result(2, 'Sinners and Saints')];

function setup() {
  setToken('sngr_test');

  const fetchMock = mockFetch((url) => {
    if (url.includes('/search')) return { body: { results: RESULTS } };
    if (url.includes('/capture')) {
      return { body: { id: 7, tmdb_id: 1, title: 'Sinners', year: 2025, status: 'new' } };
    }
    return { body: { items: [], total: 0 } };
  });

  render(<Snag />, { wrapper: wrapper(makeClient()) });
  return { fetchMock, input: screen.getByRole('searchbox') as HTMLInputElement };
}

function searchCalls(fetchMock: ReturnType<typeof mockFetch>) {
  return fetchMock.mock.calls.filter(([url]) => String(url).includes('/search'));
}

async function waitForRows() {
  await waitFor(() => expect(rows()).toHaveLength(2));
}

function rows() {
  return screen.getAllByRole('button').filter((el) => el.className.includes('sg-row'));
}

describe('search debounce', () => {
  afterEach(() => vi.useRealTimers());

  async function tick(ms: number) {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(ms);
    });
  }

  it('holds the request until the query has been still for 250 ms', async () => {
    vi.useFakeTimers();
    const { fetchMock, input } = setup();

    fireEvent.change(input, { target: { value: 'sinners' } });
    await tick(200);
    expect(searchCalls(fetchMock)).toHaveLength(0);

    await tick(100);
    expect(searchCalls(fetchMock)).toHaveLength(1);
  });

  it('restarts the wait on every keystroke', async () => {
    vi.useFakeTimers();
    const { fetchMock, input } = setup();

    fireEvent.change(input, { target: { value: 'si' } });
    await tick(200);
    fireEvent.change(input, { target: { value: 'sin' } });
    await tick(200);
    expect(searchCalls(fetchMock)).toHaveLength(0);

    await tick(100);
    expect(searchCalls(fetchMock)).toHaveLength(1);
  });
});

describe('keyboard navigation', () => {
  it('moves the cursor with the arrow keys and snags the active row on Enter', async () => {
    const { fetchMock, input } = setup();

    fireEvent.change(input, { target: { value: 'sinners' } });
    await waitForRows();

    expect(rows()[0]).toHaveAttribute('data-active', '1');

    fireEvent.keyDown(input, { key: 'ArrowDown' });
    expect(rows()[1]).toHaveAttribute('data-active', '1');
    expect(rows()[0]).not.toHaveAttribute('data-active');

    fireEvent.keyDown(input, { key: 'ArrowUp' });
    expect(rows()[0]).toHaveAttribute('data-active', '1');

    fireEvent.keyDown(input, { key: 'Enter' });

    await waitFor(() => {
      const capture = fetchMock.mock.calls.find(([url]) => String(url).includes('/capture'));
      expect(capture).toBeDefined();
      expect(JSON.parse(String(capture?.[1]?.body))).toEqual({
        tmdb_id: 1,
        media_type: 'movie',
        query: 'sinners',
        source: 'web',
      });
    });

    const resolves = fetchMock.mock.calls.filter(([url]) => String(url).includes('/resolve'));
    expect(resolves).toHaveLength(0);
  });

  it('wraps the cursor past the top of the list', async () => {
    const { input } = setup();

    fireEvent.change(input, { target: { value: 'sinners' } });
    await waitForRows();

    fireEvent.keyDown(input, { key: 'ArrowUp' });
    expect(rows()[1]).toHaveAttribute('data-active', '1');
  });

  it('clears the query and blurs on Escape', () => {
    const { input } = setup();

    fireEvent.change(input, { target: { value: 'sinners' } });
    fireEvent.keyDown(input, { key: 'Escape' });

    expect(input.value).toBe('');
    expect(input).not.toHaveFocus();
  });
});

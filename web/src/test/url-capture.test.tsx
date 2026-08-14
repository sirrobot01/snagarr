import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { setToken } from '../lib/auth';
import { Snag } from '../routes/Snag';
import { makeClient, mockFetch, wrapper } from './utils';

const LINK = 'https://letterboxd.com/film/sinners/';

describe('link capture', () => {
  it('captures a pasted link instead of searching for it', async () => {
    setToken('sngr_test');
    const fetchMock = mockFetch((url) => {
      if (url.includes('/capture')) {
        return { status: 202, body: { id: 11, title: LINK, status: 'needs_review' } };
      }
      return { body: { items: [], total: 0 } };
    });
    render(<Snag />, { wrapper: wrapper(makeClient()) });

    const input = screen.getByRole('searchbox') as HTMLInputElement;
    fireEvent.change(input, { target: { value: LINK } });

    expect(await screen.findByRole('button', { name: /snag link/i })).toBeInTheDocument();
    expect(screen.getByText('letterboxd.com', { exact: false })).toBeInTheDocument();

    fireEvent.keyDown(input, { key: 'Enter' });

    await waitFor(() => {
      const capture = fetchMock.mock.calls.find(([url]) => String(url).includes('/capture'));
      expect(capture).toBeDefined();
      expect(JSON.parse(String(capture?.[1]?.body))).toEqual({ url: LINK, source: 'web' });
    });
    await waitFor(() => expect(input.value).toBe(''));
    expect(fetchMock.mock.calls.some(([url]) => String(url).includes('/search'))).toBe(false);
  });
});

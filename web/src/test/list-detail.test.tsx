import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import List from '../routes/List';
import { setToken } from '../lib/auth';
import type { Item } from '../lib/types';
import { makeClient, mockFetch, wrapper } from './utils';

const SEVERANCE: Partial<Item> = {
  id: 7,
  tmdb_id: 95396,
  media_type: 'tv',
  title: 'Severance',
  year: 2022,
  status: 'available',
  source: 'web',
  captured_at: '2026-08-01T10:00:00Z',
  archived: false,
};

/** Desktop, so the list renders popovers rather than the mobile sheet. */
function asDesktop() {
  window.matchMedia = ((query: string) =>
    ({
      matches: true,
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    }) as unknown as MediaQueryList) as typeof window.matchMedia;
}

function setup(path: string, services: Record<string, boolean>) {
  setToken('sngr_test');
  asDesktop();
  window.history.pushState({}, '', path);

  const fetchMock = mockFetch((url) => {
    if (url.includes('/status')) return { body: { services } };
    if (url.includes('/me')) return { body: { id: 1, username: 'owner', role: 'admin' } };
    if (url.includes('/items')) return { body: { items: [SEVERANCE], total: 1 } };
    return { body: {} };
  });

  render(<List />, { wrapper: wrapper(makeClient()) });
  return fetchMock;
}

describe('list detail', () => {
  // "Open" on a search result promises that item, not the list around it.
  it('opens the item named in the query string', async () => {
    setup('/list?item=7', { radarr: true, sonarr: true, overseerr: true });

    expect(await screen.findByRole('button', { name: /Send to Sonarr/ })).toBeEnabled();
  });

  it('leaves the detail closed when no item is named', async () => {
    setup('/list', { radarr: true, sonarr: true, overseerr: true });

    await screen.findByRole('button', { name: /Open details for Severance/ });
    expect(screen.queryByRole('button', { name: /Send to Sonarr/ })).toBeNull();
  });

  // A series needs Sonarr; a connected Radarr is no help to it.
  it('disables the send a service cannot take and explains why', async () => {
    setup('/list?item=7', { radarr: true, sonarr: false, overseerr: false });

    const send = await screen.findByRole('button', { name: /Send to Sonarr/ });
    expect(send).toBeDisabled();
    expect(send).toHaveAttribute('title', 'Sonarr is not connected');

    const request = screen.getByRole('button', { name: /Request/ });
    expect(request).toBeDisabled();
    expect(request).toHaveAttribute('title', 'Overseerr is not connected');
  });

  it('offers the send once the matching service is connected', async () => {
    setup('/list?item=7', { radarr: false, sonarr: true, overseerr: true });

    expect(await screen.findByRole('button', { name: /Send to Sonarr/ })).toBeEnabled();
    expect(screen.getByRole('button', { name: /Request/ })).toBeEnabled();
  });

  it('sends the selection to the arrs, not to a "manager"', async () => {
    setup('/list', { radarr: true, sonarr: true, overseerr: true });

    const user = userEvent.setup();
    // Rows carry checkboxes only in the table view.
    await user.click(await screen.findByRole('button', { name: 'Table view' }));
    await user.click(await screen.findByRole('checkbox', { name: /Select Severance/ }));

    await waitFor(() => expect(screen.getByText('1 selected')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /Send to \*Arr/ })).toBeEnabled();
  });

  it('disables the bulk send when no arr is connected', async () => {
    setup('/list', { radarr: false, sonarr: false, overseerr: true });

    const user = userEvent.setup();
    // Rows carry checkboxes only in the table view.
    await user.click(await screen.findByRole('button', { name: 'Table view' }));
    await user.click(await screen.findByRole('checkbox', { name: /Select Severance/ }));

    const send = await screen.findByRole('button', { name: /Send to \*Arr/ });
    expect(send).toBeDisabled();
    expect(send).toHaveAttribute('title', 'Connect Radarr or Sonarr first');
  });
});

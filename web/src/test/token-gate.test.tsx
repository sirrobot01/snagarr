import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { makeClient, mockFetch, wrapper } from './utils';

async function freshAuth() {
  vi.resetModules();
  return import('../lib/auth');
}

describe('token gate', () => {
  beforeEach(() => {
    localStorage.clear();
    history.replaceState(null, '', '/');
  });

  it('consumes a #token= fragment and strips it from the URL', async () => {
    history.replaceState(null, '', '/#token=sngr_fromhash');
    const auth = await freshAuth();

    auth.initToken();

    expect(auth.getToken()).toBe('sngr_fromhash');
    expect(localStorage.getItem('snagarr.token')).toBe('sngr_fromhash');
    expect(window.location.hash).toBe('');
  });

  it('reads a stored token when there is no fragment', async () => {
    localStorage.setItem('snagarr.token', 'sngr_stored');
    const auth = await freshAuth();

    auth.initToken();

    expect(auth.getToken()).toBe('sngr_stored');
  });

  it('shows the gate with no token and leaves it once one is entered', async () => {
    const auth = await freshAuth();
    auth.initToken();

    const { App } = await import('../App');
    mockFetch((url) => {
      if (url.includes('/status')) {
        return { body: { counts: { needs_review: 0 }, sync: { collection_at: null } } };
      }
      if (url.includes('/items')) return { body: { items: [], total: 0 } };
      return { body: { id: 1, display_name: 'Mukhtar', role: 'admin' } };
    });

    render(<App />, { wrapper: wrapper(makeClient()) });
    expect(screen.getByRole('heading', { name: /enter your token/i })).toBeInTheDocument();

    const user = userEvent.setup();
    await user.type(screen.getByLabelText(/token/i), 'sngr_typed');
    await user.click(screen.getByRole('button', { name: /continue/i }));

    expect(localStorage.getItem('snagarr.token')).toBe('sngr_typed');
    expect(screen.queryByRole('heading', { name: /enter your token/i })).not.toBeInTheDocument();
  });
});

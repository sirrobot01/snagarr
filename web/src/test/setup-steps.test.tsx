import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { setToken } from '../lib/auth';
import Setup from '../routes/Setup';
import { makeClient, mockFetch, wrapper } from './utils';

function mount(builtin: boolean) {
  setToken('sngr_test');
  mockFetch((url) => {
    if (url.includes('/settings')) {
      return {
        body: {
          tmdb: { api_key: '', configured: builtin, builtin_key: builtin, locked: false },
          general: {
            reconcile_interval: '15m0s',
            public_url: '',
            auto_send: true,
            configured: true,
            locked: false,
          },
        },
      };
    }
    return { body: { services: [] } };
  });
  render(<Setup />, { wrapper: wrapper(makeClient()) });
}

describe('setup wizard', () => {
  it('skips the TMDB step when the build carries the shared key', async () => {
    mount(true);

    expect(
      await screen.findByRole('heading', { name: /point at your library/i }),
    ).toBeInTheDocument();
    expect(screen.getByText(/step 1 of 3/i)).toBeInTheDocument();
    expect(screen.queryByText(/connect tmdb/i)).not.toBeInTheDocument();
  });

  it('starts with the TMDB step when the build carries no key', async () => {
    mount(false);

    expect(await screen.findByRole('heading', { name: /connect tmdb/i })).toBeInTheDocument();
    expect(screen.getByText(/step 1 of 4/i)).toBeInTheDocument();
  });
});

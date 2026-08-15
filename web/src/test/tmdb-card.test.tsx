import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { TmdbCard } from '../components/settings/TmdbCard';
import type { Draft } from '../components/settings/draft';
import { setToken } from '../lib/auth';
import type { Settings } from '../lib/types';
import { makeClient, mockFetch, wrapper } from './utils';

function settingsWith(builtin: boolean): Settings {
  return {
    tmdb: { api_key: '', configured: builtin, builtin_key: builtin },
    general: {
      reconcile_interval: '15m',
      public_url: '',
      auto_send: true,
      configured: true,
    },
  };
}

const draft: Draft = { patch: {}, dirty: false, set: () => {}, reset: () => {} };

describe('TMDB card', () => {
  it('presents the key as an optional override when the build carries one', () => {
    setToken('sngr_test');
    mockFetch(() => ({ body: null }));
    render(<TmdbCard settings={settingsWith(true)} draft={draft} />, {
      wrapper: wrapper(makeClient()),
    });

    expect(screen.getByLabelText(/optional/i)).toBeInTheDocument();
    expect(screen.getByText(/ships with a shared TMDB key/i)).toBeInTheDocument();
  });

  it('requires a key when the build carries none', () => {
    setToken('sngr_test');
    mockFetch(() => ({ body: null }));
    render(<TmdbCard settings={settingsWith(false)} draft={draft} />, {
      wrapper: wrapper(makeClient()),
    });

    expect(screen.queryByLabelText(/optional/i)).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText(/paste your TMDB API key/i)).toBeInTheDocument();
  });
});

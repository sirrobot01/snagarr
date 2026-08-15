import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { TelegramCard } from '../components/settings/TelegramCard';
import type { Draft } from '../components/settings/draft';
import { setToken } from '../lib/auth';
import type { Settings } from '../lib/types';
import { makeClient, mockFetch, wrapper } from './utils';

const settings: Settings = {
  tmdb: { api_key: '', configured: true },
  telegram: { bot_token: '', configured: false },
  general: { reconcile_interval: '15m', public_url: '', auto_send: true, configured: true },
};

const draft: Draft = { patch: {}, dirty: false, set: () => {}, reset: () => {} };

describe('Telegram card', () => {
  it('holds the token and tests it through the settings endpoint', async () => {
    setToken('sngr_test');
    const fetchMock = mockFetch((url) => {
      if (url.includes('/settings/test')) {
        return { body: { ok: true, message: '@snagarr_bot' } };
      }
      return { body: null };
    });
    render(<TelegramCard settings={settings} draft={draft} />, {
      wrapper: wrapper(makeClient()),
    });

    expect(screen.getByLabelText(/bot token/i)).toBeInTheDocument();
    expect(screen.getByText(/BotFather/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /test connection/i }));
    await waitFor(() => expect(screen.getByText('@snagarr_bot')).toBeInTheDocument());

    const test = fetchMock.mock.calls.find(([url]) => String(url).includes('/settings/test'));
    expect(JSON.parse(String(test?.[1]?.body))).toEqual({ service: 'telegram' });
  });
});

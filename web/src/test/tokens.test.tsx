import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { TokensDialog } from '../components/settings/TokensDialog';
import type { HouseholdUser, Token } from '../lib/types';
import { makeClient, mockFetch, wrapper } from './utils';

const member: HouseholdUser = {
  id: 2,
  username: 'amina',
  role: 'member',
  telegram_user_id: null,
  token_count: 1,
  created_at: '2026-01-01T00:00:00Z',
};

function token(overrides: Partial<Token> = {}): Token {
  return {
    id: 7,
    name: 'iPhone Shortcut',
    prefix: 'sngr_ab12',
    created_at: '2026-01-01T00:00:00Z',
    last_used_at: null,
    revoked: false,
    ...overrides,
  };
}

function open() {
  render(<TokensDialog user={member} onClose={vi.fn()} />, { wrapper: wrapper(makeClient()) });
}

describe('tokens', () => {
  /* The list endpoint never returns a secret, so the dialog can only show what
     a token is called and when it was last used. */
  it('lists the tokens a member holds and leaves revoked ones out', async () => {
    mockFetch(() => ({ body: { tokens: [token(), token({ id: 8, name: 'Old laptop', revoked: true })] } }));

    open();

    expect(await screen.findByText('iPhone Shortcut')).toBeInTheDocument();
    expect(screen.getByText('sngr_ab12…')).toBeInTheDocument();
    expect(screen.getByText('never')).toBeInTheDocument();
    expect(screen.queryByText('Old laptop')).not.toBeInTheDocument();
  });

  /* POST is the only moment the raw secret exists, so it is held in the dialog
     rather than looked up again. */
  it('issues a named token and shows the secret once', async () => {
    const fetchSpy = mockFetch((_url, init) => {
      if (init?.method === 'POST') {
        return {
          status: 201,
          body: { id: 9, name: 'Bookmarklet', token: 'sngr_secret', created_at: '2026-01-01T00:00:00Z' },
        };
      }
      return { body: { tokens: [] } };
    });

    open();
    const user = userEvent.setup();

    await user.type(await screen.findByLabelText('Token name'), 'Bookmarklet');
    await user.click(screen.getByRole('button', { name: /issue token/i }));

    expect(await screen.findByLabelText('New token')).toHaveValue('sngr_secret');

    const post = fetchSpy.mock.calls.find((call) => (call[1] as RequestInit)?.method === 'POST');
    expect(String(post?.[0])).toBe('/api/v1/users/2/tokens');
    expect(JSON.parse(String((post?.[1] as RequestInit).body))).toEqual({ name: 'Bookmarklet' });
  });

  it('asks before it revokes one token', async () => {
    const fetchSpy = mockFetch((_url, init) => {
      if (init?.method === 'DELETE') return { status: 204 };
      return { body: { tokens: [token()] } };
    });

    open();
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: /revoke/i }));
    const confirm = await screen.findByRole('dialog', { name: /revoke iphone shortcut/i });
    expect(fetchSpy.mock.calls.some((call) => (call[1] as RequestInit)?.method === 'DELETE')).toBe(
      false,
    );

    await user.click(within(confirm).getByRole('button', { name: /^revoke$/i }));

    await waitFor(() => {
      const del = fetchSpy.mock.calls.find((call) => (call[1] as RequestInit)?.method === 'DELETE');
      expect(String(del?.[0])).toBe('/api/v1/tokens/7');
    });
  });
});

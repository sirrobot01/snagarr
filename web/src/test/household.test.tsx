import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { HouseholdSection } from '../components/settings/HouseholdSection';
import { makeClient, mockFetch, wrapper } from './utils';

const users = [
  {
    id: 1,
    username: 'mukhtar',
    role: 'admin',
    telegram_user_id: null,
    token_count: 2,
    created_at: '2026-01-01T00:00:00Z',
  },
  {
    id: 2,
    username: 'amina',
    role: 'member',
    telegram_user_id: null,
    token_count: 0,
    created_at: '2026-01-01T00:00:00Z',
  },
];

function open() {
  mockFetch((url) => {
    if (url.includes('/tokens')) return { body: { tokens: [] } };
    if (url.includes('/status')) return { body: { services: {}, sync: { running: false } } };
    return { body: { users } };
  });
  render(<HouseholdSection publicUrl="https://snagarr.example.com" meId={1} />, {
    wrapper: wrapper(makeClient()),
  });
}

describe('household', () => {
  it('opens the member form in a dialog', async () => {
    open();
    const user = userEvent.setup();

    expect(screen.queryByLabelText('Username')).not.toBeInTheDocument();
    await user.click(await screen.findByRole('button', { name: /add household member/i }));

    const dialog = await screen.findByRole('dialog', { name: /add a household member/i });
    expect(dialog).toBeInTheDocument();
    expect(screen.getByLabelText('Username')).toBeInTheDocument();
  });

  /* An admin reaches every member's tokens from the row that names them, which
     is the only place the interface issues one. */
  it('opens one member’s tokens from their row', async () => {
    open();
    const user = userEvent.setup();

    await screen.findByText('@amina');
    const rows = screen.getAllByRole('button', { name: /tokens/i });
    await user.click(rows[1]);

    expect(await screen.findByRole('dialog', { name: /tokens · amina/i })).toBeInTheDocument();
    expect(screen.getByLabelText('Token name')).toBeInTheDocument();
  });

  it('keeps the bookmarklet behind a dialog and a confirmation of its own', async () => {
    open();
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: /browser bookmarklet/i }));

    const dialog = await screen.findByRole('dialog', { name: /browser bookmarklet/i });
    expect(dialog).toBeInTheDocument();
    // Nothing is issued until the reader asks for it.
    expect(screen.queryByLabelText(/drag this/i)).not.toBeInTheDocument();
  });
});

// The admin registers through first-run, which never asks for a Telegram ID,
// so their own row must be linkable after the fact — like everyone else's.
it('links the admin’s own Telegram account from their row', async () => {
  const requests: Array<{ url: string; init?: RequestInit }> = [];
  mockFetch((url, init) => {
    requests.push({ url, init });
    if (url.includes('/tokens')) return { body: { tokens: [] } };
    if (url.includes('/status')) return { body: { services: {}, sync: { running: false } } };
    if (init?.method === 'PATCH') {
      return { body: { ...users[0], telegram_user_id: 5551234 } };
    }
    return { body: { users } };
  });
  render(<HouseholdSection publicUrl="https://snagarr.example.com" meId={1} />, {
    wrapper: wrapper(makeClient()),
  });
  const user = userEvent.setup();

  const linkButtons = await screen.findAllByRole('button', { name: /telegram/i });
  await user.click(linkButtons[0]);

  expect(await screen.findByText(/link telegram for mukhtar/i)).toBeInTheDocument();
  await user.type(screen.getByLabelText(/telegram user id/i), '5551234');
  await user.click(screen.getByRole('button', { name: /^save$/i }));

  const patch = requests.find((r) => r.init?.method === 'PATCH');
  expect(patch?.url).toContain('/users/1');
  expect(JSON.parse(String(patch?.init?.body))).toEqual({ telegram_user_id: 5551234 });
});

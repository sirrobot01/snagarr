import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { mockFetch } from './utils';

async function freshAuth() {
  vi.resetModules();
  return import('../lib/auth');
}

describe('account gate', () => {
  beforeEach(() => {
    localStorage.clear();
    history.replaceState(null, '', '/');
  });

  it('does not accept legacy token links and removes the secret from the URL', async () => {
    history.replaceState(null, '', '/#token=sngr_fromhash');
    const auth = await freshAuth();

    auth.initToken();

    expect(auth.getToken()).toBeNull();
    expect(localStorage.getItem('snagarr.token')).toBeNull();
    expect(window.location.hash).toBe('');
  });

  it('keeps existing browser sessions signed in', async () => {
    localStorage.setItem('snagarr.token', 'sngr_stored');
    const auth = await freshAuth();

    auth.initToken();

    expect(auth.getToken()).toBe('sngr_stored');
  });

  it('creates the first account and stores the returned browser session', async () => {
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    mockFetch((url, init) => {
      requests.push({ url, init });
      if (url.endsWith('/auth/status')) {
        return { body: { initialized: false, setup_required: true } };
      }
      return {
        status: 201,
        body: {
          token: 'sngr_registered',
          user: { id: 1, username: 'alex.m', role: 'admin' },
        },
      };
    });
    const { TokenGate } = await import('../components/TokenGate');
    const user = userEvent.setup();

    render(<TokenGate />);
    expect(await screen.findByRole('heading', { name: /create your account/i })).toBeInTheDocument();
    expect(screen.queryByLabelText(/your name/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/credentials stay/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/private by default/i)).not.toBeInTheDocument();

    await user.type(screen.getByLabelText(/^username$/i), 'alex.m');
    await user.type(screen.getByLabelText(/^password$/i), 'x');
    await user.type(screen.getByLabelText(/confirm password/i), 'x');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    await waitFor(() => expect(localStorage.getItem('snagarr.token')).toBe('sngr_registered'));
	expect(window.location.pathname).toBe('/setup');
    const registration = requests.find(({ url }) => url.endsWith('/auth/register'));
    expect(registration?.init?.method).toBe('POST');
    expect(JSON.parse(String(registration?.init?.body))).toEqual({
      username: 'alex.m',
      password: 'x',
    });
  });

  it('shows sign-in for an initialized server with a password visibility control', async () => {
    mockFetch((url, init) => {
      if (url.endsWith('/auth/status')) {
        return { body: { initialized: true, setup_required: true } };
      }
      expect(url).toMatch(/\/auth\/login$/);
      expect(JSON.parse(String(init?.body))).toEqual({ username: 'alex', password: 'secret' });
      return {
        body: {
          token: 'sngr_signed_in',
          user: { id: 1, username: 'alex', role: 'admin' },
        },
      };
    });
    const { TokenGate } = await import('../components/TokenGate');
    const user = userEvent.setup();

    render(<TokenGate />);
    expect(await screen.findByRole('heading', { name: /sign in to snagarr/i })).toBeInTheDocument();
    expect(screen.queryByLabelText(/your name/i)).not.toBeInTheDocument();

    const password = screen.getByLabelText(/^password$/i);
    expect(password).toHaveAttribute('type', 'password');
    await user.click(screen.getByRole('button', { name: /show password/i }));
    expect(password).toHaveAttribute('type', 'text');

    await user.type(screen.getByLabelText(/^username$/i), 'alex');
    await user.type(password, 'secret');
    await user.click(screen.getByRole('button', { name: /^sign in$/i }));

    await waitFor(() => expect(localStorage.getItem('snagarr.token')).toBe('sngr_signed_in'));
	expect(window.location.pathname).toBe('/setup');
  });

  it('reports field validation accessibly without sending an invalid registration', async () => {
    const fetch = mockFetch(() => ({ body: { initialized: false, setup_required: true } }));
    const { TokenGate } = await import('../components/TokenGate');
    const user = userEvent.setup();

    render(<TokenGate />);
    await screen.findByRole('heading', { name: /create your account/i });
    await user.type(screen.getByLabelText(/^password$/i), 'x');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    expect(await screen.findByText(/enter your username/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/^username$/i)).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByLabelText(/^password$/i)).not.toHaveAttribute('aria-invalid');
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it('offers a retry when account status cannot be loaded', async () => {
    let attempt = 0;
    mockFetch(() => {
      attempt += 1;
      if (attempt === 1) return { status: 503, body: { error: { message: 'Server is starting.' } } };
      return { body: { initialized: true, setup_required: true } };
    });
    const { TokenGate } = await import('../components/TokenGate');
    const user = userEvent.setup();

    render(<TokenGate />);
    expect(await screen.findByRole('heading', { name: /couldn’t reach snagarr/i })).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('Server is starting.');

    await user.click(screen.getByRole('button', { name: /try again/i }));
    expect(await screen.findByRole('heading', { name: /sign in to snagarr/i })).toBeInTheDocument();
  });
});

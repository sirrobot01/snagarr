import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { MyServices } from '../components/settings/MyServices';
import type { Service } from '../lib/types';
import { makeClient, mockFetch, wrapper } from './utils';

function radarr(overrides: Partial<Service> = {}): Service {
  return {
    id: 3,
    user_id: 1,
    kind: 'radarr',
    name: 'Default',
    /* What the API sends: the key is masked down to its last four characters. */
    config: {
      url: 'http://radarr.lan:7878',
      api_key: '••••4e2a',
      quality_profile_id: 4,
      root_folder: '/movies',
      search_on_add: true,
    },
    enabled: true,
    locked: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

function bodyOf(call: unknown[]): Record<string, unknown> {
  const init = call[1] as RequestInit;
  return JSON.parse(String(init.body)) as Record<string, unknown>;
}

describe('services', () => {
  it('creates a service of the chosen kind', async () => {
    const fetchSpy = mockFetch((_url, init) => {
      if (init?.method === 'POST') return { status: 201, body: radarr({ kind: 'sonarr' }) };
      return { body: { services: [] } };
    });

    render(<MyServices />, { wrapper: wrapper(makeClient()) });
    await screen.findByLabelText(/add service/i);

    const user = userEvent.setup();
    await user.selectOptions(screen.getByLabelText(/add service/i), 'sonarr');
    await user.click(screen.getByRole('button', { name: /^add$/i }));

    await waitFor(() => {
      const post = fetchSpy.mock.calls.find((call) => (call[1] as RequestInit)?.method === 'POST');
      expect(post).toBeDefined();
      expect(String(post?.[0])).toBe('/api/v1/services');
      expect(bodyOf(post as unknown[])).toEqual({ kind: 'sonarr', name: 'Default' });
    });
  });

  /* The secret only ever leaves the server once. Editing anything else must
     send the mask back untouched — or not send it at all — so the stored key
     survives the save. */
  it('saves an edit without disturbing the masked secret', async () => {
    const fetchSpy = mockFetch((_url, init) => {
      if (init?.method === 'PATCH') return { body: radarr({ config: { root_folder: '/films' } }) };
      return { body: { services: [radarr()] } };
    });

    render(<MyServices />, { wrapper: wrapper(makeClient()) });
    const key = await screen.findByLabelText('API key');
    expect(key).toHaveValue('••••4e2a');

    const user = userEvent.setup();
    await user.clear(screen.getByLabelText('Root folder'));
    await user.type(screen.getByLabelText('Root folder'), '/films');
    await user.click(screen.getByRole('button', { name: /^save$/i }));

    await waitFor(() => {
      const patch = fetchSpy.mock.calls.find(
        (call) => (call[1] as RequestInit)?.method === 'PATCH',
      );
      expect(patch).toBeDefined();
      expect(String(patch?.[0])).toBe('/api/v1/services/3');
      expect(bodyOf(patch as unknown[])).toEqual({ config: { root_folder: '/films' } });
    });
  });
});

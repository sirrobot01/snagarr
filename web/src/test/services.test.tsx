import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { MyServices } from '../components/settings/MyServices';
import { PlexServerPicker } from '../components/settings/PlexSignIn';
import { SectionsField } from '../components/settings/options';
import type { PlexServer, Service } from '../lib/types';
import { makeClient, mockFetch, wrapper } from './utils';

function radarr(overrides: Partial<Service> = {}): Service {
  return {
    id: 3,
    user_id: 1,
    kind: 'radarr',
    name: 'Radarr - Default',
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
  /* Picking a kind used to POST straight away, which left an empty record
     behind whenever somebody changed their mind. Nothing is written now until
     the reader asks for it. */
  it('writes nothing until the new connection is added', async () => {
    const fetchSpy = mockFetch((_url, init) => {
      if (init?.method === 'POST') return { status: 201, body: radarr({ kind: 'sonarr' }) };
      return { body: { services: [] } };
    });
    const posted = () =>
      fetchSpy.mock.calls.filter((call) => (call[1] as RequestInit)?.method === 'POST');

    render(<MyServices />, { wrapper: wrapper(makeClient()) });

    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: /connect a service/i }));
    await user.click(await screen.findByRole('button', { name: /sonarr/i }));

    expect(await screen.findByLabelText('Connection name')).toHaveValue('Sonarr - Default');
    expect(posted()).toHaveLength(0);

    await user.type(screen.getByLabelText('URL'), 'http://sonarr.lan:8989');
    await user.click(screen.getByRole('button', { name: /add connection/i }));

    await waitFor(() => {
      const post = posted()[0];
      expect(post).toBeDefined();
      expect(String(post?.[0])).toBe('/api/v1/services');
      expect(bodyOf(post as unknown[])).toEqual({
        kind: 'sonarr',
        name: 'Sonarr - Default',
        config: { url: 'http://sonarr.lan:8989' },
        enabled: true,
      });
    });
  });

  /* A quality profile has to be pickable while the connection is being built,
     which is before it exists. The test is what proves the credentials reach
     the service, so it is what fills the lists. */
  it('offers the real profiles and folders once a test has answered', async () => {
    const fetchSpy = mockFetch((url) => {
      if (String(url).endsWith('/services/test')) {
        return { body: { ok: true, message: 'OK · 611 monitored' } };
      }
      if (String(url).endsWith('/services/options')) {
        return {
          body: {
            quality_profiles: [{ id: 4, name: 'HD-1080p' }],
            root_folders: [{ path: '/movies', free_space: 812340000000 }],
          },
        };
      }
      return { body: { services: [] } };
    });

    render(<MyServices />, { wrapper: wrapper(makeClient()) });
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: /connect a service/i }));
    await user.click(await screen.findByRole('button', { name: 'Radarr' }));

    // Nothing has answered yet, so the fields ask for the raw values.
    expect(await screen.findByLabelText('Quality profile ID')).toBeInTheDocument();

    await user.type(screen.getByLabelText('URL'), 'http://radarr.lan:7878');
    await user.type(screen.getByLabelText('API key'), 'secret');
    await user.click(screen.getByRole('button', { name: /test connection/i }));

    const profile = await screen.findByLabelText('Quality profile');
    expect(profile.tagName).toBe('SELECT');
    await user.selectOptions(profile, '4');
    expect(screen.getByLabelText('Root folder')).toHaveDisplayValue('Choose a folder');

    const options = fetchSpy.mock.calls.find((call) =>
      String(call[0]).endsWith('/services/options'),
    );
    expect(bodyOf(options as unknown[])).toEqual({
      kind: 'radarr',
      config: { url: 'http://radarr.lan:7878', api_key: 'secret' },
    });
  });

  /* Testing is not saving. It used to save first, which emptied the draft and
     greyed out Save the moment the reader tried the connection. */
  it('tests the typed credentials without saving them', async () => {
    const fetchSpy = mockFetch((url, init) => {
      if (String(url).endsWith('/services/test')) {
        return { body: { ok: true, message: 'OK · 611 items' } };
      }
      if (init?.method === 'PATCH') return { body: radarr() };
      return { body: { services: [radarr()] } };
    });

    render(<MyServices />, { wrapper: wrapper(makeClient()) });
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: /radarr - default/i }));

    await user.clear(await screen.findByLabelText('Root folder'));
    await user.type(screen.getByLabelText('Root folder'), '/films');
    await user.click(screen.getByRole('button', { name: /test connection/i }));

    expect(await screen.findByText('OK · 611 items')).toBeInTheDocument();

    const test = fetchSpy.mock.calls.find((call) => String(call[0]).endsWith('/services/test'));
    expect(bodyOf(test as unknown[])).toEqual({
      id: 3,
      kind: 'radarr',
      config: {
        url: 'http://radarr.lan:7878',
        api_key: '••••4e2a',
        quality_profile_id: 4,
        root_folder: '/films',
        search_on_add: true,
      },
    });
    expect(
      fetchSpy.mock.calls.some((call) => (call[1] as RequestInit)?.method === 'PATCH'),
    ).toBe(false);
    expect(screen.getByRole('button', { name: /^save$/i })).toBeEnabled();
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
    const tile = await screen.findByRole('button', { name: /radarr - default/i });
    expect(screen.queryByLabelText('API key')).not.toBeInTheDocument();

    const user = userEvent.setup();
    await user.click(tile);
    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    const key = await screen.findByLabelText('API key');
    expect(key).toHaveValue('••••4e2a');

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

  /* plex.tv repeats a server once per account it is shared with, and it cannot
     know which address this Snagarr reaches. So: each server once, and picking
     one hands back the token alone. */
  it('lists each Plex server once and takes only its token', async () => {
    const living: PlexServer = {
      name: 'Living room',
      client_identifier: 'plex-1',
      connections: [
        { uri: 'http://192.168.1.5:32400', local: true, relay: false, reachable: true },
        { uri: 'https://remote.example:32400', local: false, relay: false, reachable: false },
      ],
    };
    const servers: PlexServer[] = [
      living,
      { ...living },
      { name: 'Attic', client_identifier: 'plex-2', connections: [] },
    ];
    const picked = vi.fn();
    const user = userEvent.setup();

    render(<PlexServerPicker servers={servers} token="plex-token" onPicked={picked} />);

    expect(screen.getAllByRole('button')).toHaveLength(2);
    await user.click(screen.getByRole('button', { name: 'Use the token for Living room' }));
    expect(picked).toHaveBeenCalledWith('plex-token');
  });

  it('accepts comma-separated Plex section IDs as text', async () => {
    const changed = vi.fn();
    const user = userEvent.setup();

    render(
      <SectionsField id={7} config={{ section_ids: [] }} locked={false} onChange={changed} />,
      { wrapper: wrapper(makeClient()) },
    );

    const input = screen.getByLabelText('Section IDs');
    expect(input).toHaveAttribute('type', 'text');
    await user.type(input, '1, 2');
    expect(input).toHaveValue('1, 2');
    await user.tab();
    expect(changed).toHaveBeenCalledWith({ section_ids: ['1', '2'] });
  });
});

import { getToken, rejectToken } from './auth';
import type {
  CapturePayload,
  CreatedToken,
  HouseholdUser,
  Item,
  ItemsResponse,
  MediaType,
  NewService,
  PlexPin,
  PlexPinCheck,
  PlexServer,
  SearchResponse,
  SendTarget,
  Service,
  ServiceConfig,
  ServiceKind,
  ServiceOptions,
  ServicePatch,
  Settings,
  SettingsPatch,
  StatusResponse,
  TestResult,
  Token,
  UserRef,
} from './types';

const BASE = '/api/v1';

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

export class NetworkError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'NetworkError';
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  const token = getToken();
  if (token) headers.set('Authorization', `Bearer ${token}`);
  if (init.body) headers.set('Content-Type', 'application/json');

  let res: Response;
  try {
    res = await fetch(BASE + path, { ...init, headers });
  } catch {
    throw new NetworkError('network unreachable');
  }

  if (res.status === 401) {
    rejectToken();
    throw new ApiError(401, 'unauthorized', 'token rejected');
  }
  if (res.status === 204) return undefined as T;

  const body = await res.json().catch(() => null);
  if (!res.ok) {
    const err = (body as { error?: { code?: string; message?: string } } | null)?.error;
    throw new ApiError(res.status, err?.code ?? 'error', err?.message ?? res.statusText);
  }
  return body as T;
}

function json(body: unknown): RequestInit {
  return { body: JSON.stringify(body) };
}

function query(params: Record<string, string | number | boolean | undefined>) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') search.set(key, String(value));
  }
  const out = search.toString();
  return out ? `?${out}` : '';
}

export const api = {
  me: () => request<UserRef>('/me'),
  status: () => request<StatusResponse>('/status'),

  search: (q: string, limit = 12) =>
    request<SearchResponse>(`/search${query({ q, limit })}`),

  items: (params: {
    status?: string;
    type?: string;
    q?: string;
    captured_by?: number;
    archived?: boolean;
    limit?: number;
    offset?: number;
  }) => request<ItemsResponse>(`/items${query(params)}`),

  item: (id: number) => request<Item>(`/items/${id}`),

  capture: (payload: CapturePayload) =>
    request<Item>('/capture', { method: 'POST', ...json(payload) }),

  resolve: (id: number, tmdb_id: number, media_type: MediaType) =>
    request<Item>(`/items/${id}/resolve`, { method: 'POST', ...json({ tmdb_id, media_type }) }),

  send: (id: number, target: SendTarget) =>
    request<Item>(`/items/${id}/send`, { method: 'POST', ...json({ target }) }),

  archive: (id: number, archived: boolean) =>
    request<Item>(`/items/${id}/archive`, { method: 'POST', ...json({ archived }) }),

  remove: (id: number) => request<void>(`/items/${id}`, { method: 'DELETE' }),

  users: () => request<{ users: HouseholdUser[] }>('/users'),

  createUser: (body: {
    username: string;
    password: string;
    role: string;
    telegram_user_id?: number;
  }) =>
    request<HouseholdUser>('/users', { method: 'POST', ...json(body) }),

  updateUser: (
    id: number,
    patch: { username?: string; password?: string; role?: string; telegram_user_id?: number },
  ) => request<HouseholdUser>(`/users/${id}`, { method: 'PATCH', ...json(patch) }),

  deleteUser: (id: number) => request<void>(`/users/${id}`, { method: 'DELETE' }),

  tokens: (userId: number) => request<{ tokens: Token[] }>(`/users/${userId}/tokens`),

  createToken: (userId: number, name: string) =>
    request<CreatedToken>(`/users/${userId}/tokens`, { method: 'POST', ...json({ name }) }),

  revokeToken: (id: number) => request<void>(`/tokens/${id}`, { method: 'DELETE' }),

  /** Admin only — the global settings are TMDB, the Telegram bot and General. */
  settings: () => request<Settings>('/settings'),

  saveSettings: (patch: SettingsPatch) =>
    request<Settings>('/settings', { method: 'PUT', ...json(patch) }),

  /* The two global services test through the settings endpoint; everything
     else is a service record with a test of its own. */
  testTmdb: () =>
    request<TestResult>('/settings/test', { method: 'POST', ...json({ service: 'tmdb' }) }),

  testTelegram: () =>
    request<TestResult>('/settings/test', { method: 'POST', ...json({ service: 'telegram' }) }),

  /* ── Services ───────────────────────────────────────────────────────────── */

  services: () => request<{ services: Service[] }>('/services'),

  /** Admin only — another member's stack, for the household view. */
  userServices: (userId: number) => request<{ services: Service[] }>(`/users/${userId}/services`),

  createService: (body: NewService) =>
    request<Service>('/services', { method: 'POST', ...json(body) }),

  updateService: (id: number, patch: ServicePatch) =>
    request<Service>(`/services/${id}`, { method: 'PATCH', ...json(patch) }),

  deleteService: (id: number) => request<void>(`/services/${id}`, { method: 'DELETE' }),

  testService: (id: number) => request<TestResult>(`/services/${id}/test`, { method: 'POST' }),

  /** Tests what the user has typed. `id` lets the server fill in any secret
      that came back masked, so an edit to the URL alone still tests. */
  testDraft: (body: { id?: number; kind: ServiceKind; config: ServiceConfig }) =>
    request<TestResult>('/services/test', { method: 'POST', ...json(body) }),

  serviceOptions: (id: number) => request<ServiceOptions>(`/services/${id}/options`),

  /** The quality profiles, root folders and library sections the typed
      credentials can reach. Saves nothing, so it works before a first save. */
  draftOptions: (body: { id?: number; kind: ServiceKind; config: ServiceConfig }) =>
    request<ServiceOptions>('/services/options', { method: 'POST', ...json(body) }),

  /* ── Plex sign-in ───────────────────────────────────────────────────────── */

  plexPin: () => request<PlexPin>('/plex/pin', { method: 'POST' }),

  /** Resolves with `token` once linked, with `status: "pending"` while waiting.
      Throws ApiError(410) once the pin has expired. */
  plexPinCheck: (id: number) => request<PlexPinCheck>(`/plex/pin/${id}`),

  plexServers: (token: string) =>
    request<{ servers: PlexServer[] }>(`/plex/servers${query({ token })}`),

  sync: () => request<void>('/admin/sync', { method: 'POST' }),
};

import { clearToken, getToken } from './auth';
import type {
  CapturePayload,
  CreatedToken,
  HouseholdUser,
  Item,
  ItemsResponse,
  MediaType,
  SearchResponse,
  SendTarget,
  ServiceKey,
  ServiceOptions,
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
    clearToken();
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

  createUser: (body: { display_name: string; role: string; telegram_user_id?: number }) =>
    request<HouseholdUser>('/users', { method: 'POST', ...json(body) }),

  deleteUser: (id: number) => request<void>(`/users/${id}`, { method: 'DELETE' }),

  tokens: (userId: number) => request<{ tokens: Token[] }>(`/users/${userId}/tokens`),

  createToken: (userId: number, name: string) =>
    request<CreatedToken>(`/users/${userId}/tokens`, { method: 'POST', ...json({ name }) }),

  revokeToken: (id: number) => request<void>(`/tokens/${id}`, { method: 'DELETE' }),

  settings: () => request<Settings>('/settings'),

  saveSettings: (patch: SettingsPatch) =>
    request<Settings>('/settings', { method: 'PUT', ...json(patch) }),

  testService: (service: ServiceKey) =>
    request<TestResult>('/settings/test', { method: 'POST', ...json({ service }) }),

  serviceOptions: (service: ServiceKey) =>
    request<ServiceOptions>(`/settings/options${query({ service })}`),

  sync: () => request<void>('/admin/sync', { method: 'POST' }),
};

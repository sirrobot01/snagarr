const STORAGE_KEY = 'snagarr.token';
const API_BASE = '/api/v1';

let token: string | null = null;
const listeners = new Set<() => void>();

export interface AuthUser {
  id: number;
  username: string;
  role: string;
}

export interface AuthSession {
  token: string;
  user: AuthUser;
}

export interface RegistrationDetails {
  username: string;
  password: string;
}

export interface SignInDetails {
  username: string;
  password: string;
}

export interface AuthStatus {
  initialized: boolean;
  setup_required: boolean;
}

export class AuthRequestError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = 'AuthRequestError';
  }
}

function emit() {
  for (const listener of listeners) listener();
}

/* Browser sessions are internal implementation details. Legacy token-bearing
   fragments are scrubbed, but deliberately never accepted as authentication. */
export function initToken() {
  if (/(?:^#|[&#])token=/.test(window.location.hash)) {
    history.replaceState(null, '', window.location.pathname + window.location.search);
  }
  token = localStorage.getItem(STORAGE_KEY);
}

let rejected = false;

export function getToken(): string | null {
  return token;
}

/** True when the server refused the stored session, as opposed to the user
    signing out or never having signed in. */
export function wasRejected(): boolean {
  return rejected;
}

export function setToken(next: string) {
  rejected = false;
  token = next.trim();
  localStorage.setItem(STORAGE_KEY, token);
  emit();
}

export function clearToken() {
  rejected = false;
  token = null;
  localStorage.removeItem(STORAGE_KEY);
  emit();
}

/** A 401 for a session the browser still holds: expired or revoked. The
    sign-in screen uses the distinction to say why the user is looking at it. */
export function rejectToken() {
  rejected = rejected || token !== null;
  token = null;
  localStorage.removeItem(STORAGE_KEY);
  emit();
}

export function subscribeToken(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

async function authRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body) headers.set('Content-Type', 'application/json');

  let response: Response;
  try {
    response = await fetch(API_BASE + path, { ...init, headers });
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error;
    throw new AuthRequestError(0, 'Snagarr could not be reached. Check the server and try again.');
  }

  const body = await response.json().catch(() => null);
  if (!response.ok) {
    const payload = body as { error?: { message?: string }; message?: string } | null;
    const message =
      payload?.error?.message ?? payload?.message ?? 'Something went wrong. Please try again.';
    throw new AuthRequestError(response.status, message);
  }
  return body as T;
}

export function getAuthStatus(signal?: AbortSignal) {
  return authRequest<AuthStatus>('/auth/status', { signal });
}

export function registerAccount(details: RegistrationDetails) {
  return authRequest<AuthSession>('/auth/register', {
    method: 'POST',
    body: JSON.stringify(details),
  });
}

export function signIn(details: SignInDetails) {
  return authRequest<AuthSession>('/auth/login', {
    method: 'POST',
    body: JSON.stringify(details),
  });
}

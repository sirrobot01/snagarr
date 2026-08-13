const STORAGE_KEY = 'snagarr.token';

let token: string | null = null;
const listeners = new Set<() => void>();

function emit() {
  for (const listener of listeners) listener();
}

/* The first run prints a setup URL carrying the token in the fragment. Consume
   it before React mounts, then strip it so the token never sits in history. */
export function initToken() {
  const match = /[#&]token=([^&]+)/.exec(window.location.hash);
  if (match) {
    setToken(decodeURIComponent(match[1]));
    history.replaceState(null, '', window.location.pathname + window.location.search);
    return;
  }
  token = localStorage.getItem(STORAGE_KEY);
}

export function getToken(): string | null {
  return token;
}

export function setToken(next: string) {
  token = next.trim();
  localStorage.setItem(STORAGE_KEY, token);
  emit();
}

export function clearToken() {
  token = null;
  localStorage.removeItem(STORAGE_KEY);
  emit();
}

export function subscribeToken(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

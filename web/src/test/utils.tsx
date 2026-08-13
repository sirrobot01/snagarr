import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { vi } from 'vitest';

export function makeClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

export function wrapper(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

type Handler = (url: string, init?: RequestInit) => { status?: number; body?: unknown };

export function mockFetch(handler: Handler) {
  const spy = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const { status = 200, body = null } = handler(url, init);
    return {
      ok: status < 400,
      status,
      statusText: String(status),
      json: async () => body,
    } as Response;
  });
  vi.stubGlobal('fetch', spy);
  return spy;
}

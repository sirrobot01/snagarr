import { useMutation, useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { ApiError, api } from '../../lib/api';
import { keys } from '../../lib/queries';
import type {
  ArrKind,
  LibraryKind,
  Service,
  ServiceConfig,
  ServiceKind,
  TestResult,
} from '../../lib/types';
import type { ServiceDraft } from './draft';

export const KINDS: { value: ServiceKind; label: string }[] = [
  { value: 'plex', label: 'Plex' },
  { value: 'emby', label: 'Emby' },
  { value: 'jellyfin', label: 'Jellyfin' },
  { value: 'radarr', label: 'Radarr' },
  { value: 'sonarr', label: 'Sonarr' },
  { value: 'overseerr', label: 'Overseerr' },
  { value: 'ntfy', label: 'ntfy' },
];

export function kindLabel(kind: ServiceKind): string {
  return KINDS.find((item) => item.value === kind)?.label ?? kind;
}

export function isLibrary(kind: ServiceKind): kind is LibraryKind {
  return kind === 'plex' || kind === 'emby' || kind === 'jellyfin';
}

export function isArr(kind: ServiceKind): kind is ArrKind {
  return kind === 'radarr' || kind === 'sonarr';
}

/* One name per kind per member is a server-side rule, so the second Radarr gets
   "Radarr - Default 2" rather than a 409 the user has to work out for
   themselves. The kind is in the name because the name is what every other
   screen prints. */
export function freeName(services: Service[], kind: ServiceKind): string {
  const base = `${kindLabel(kind)} - Default`;
  const taken = new Set(services.filter((s) => s.kind === kind).map((s) => s.name));
  if (!taken.has(base)) return base;
  for (let n = 2; ; n += 1) {
    if (!taken.has(`${base} ${n}`)) return `${base} ${n}`;
  }
}

/* Mirrors the Configured() methods in internal/config/services.go. The service
   API sends no such flag, so the card works it out from the config it has. */
export function configured(service: Service): boolean {
  const { url, token, api_key, topic } = service.config;
  if (isLibrary(service.kind)) return Boolean(url && token);
  if (service.kind === 'ntfy') return Boolean(topic);
  return Boolean(url && api_key);
}

export interface ServiceTest {
  result: TestResult | null;
  pending: boolean;
  run: () => void;
  /** The credentials that last answered. The options lookup runs on these. */
  probed: ServiceConfig | null;
}

function toTest(
  status: 'idle' | 'pending' | 'success' | 'error',
  data: TestResult | undefined,
  error: Error | null,
): TestResult | null {
  if (status === 'success' && data) return data;
  if (status === 'error') {
    return { ok: false, message: error instanceof ApiError ? error.message : 'network unreachable' };
  }
  return null;
}

/* The test runs against what is on screen, not against what is stored. That is
   what lets a new connection be tested before it is created, and it leaves a
   pending edit pending — testing is not saving. Any secret the client holds
   only as a mask is filled in server-side from the record `id` names. */
export function useTestDraft(kind: ServiceKind, draft: ServiceDraft, id?: number): ServiceTest {
  // The config travels as the mutation's variable so the answer is matched to
  // the credentials that produced it, not to whatever has been typed since.
  const [probed, setProbed] = useState<ServiceConfig | null>(null);
  const test = useMutation({
    mutationFn: (config: ServiceConfig) => api.testDraft({ id, kind, config }),
    onSuccess: (result, config) => {
      if (result.ok) setProbed(config);
    },
  });

  return {
    result: toTest(test.status, test.data, test.error),
    pending: test.isPending,
    run: () => test.mutate(draft.config),
    probed,
  };
}

export function useTmdbTest(): ServiceTest {
  const test = useMutation({ mutationFn: api.testTmdb });
  return {
    result: toTest(test.status, test.data, test.error),
    pending: test.isPending,
    run: () => test.mutate(),
    probed: null,
  };
}

/* The lookup calls the service itself, so it runs on credentials that have
   answered — the stored ones, or the ones a test just accepted — rather than on
   every keystroke. A null config means there is nothing to ask yet. */
export function useServiceOptions(kind: ServiceKind, config: ServiceConfig | null, id?: number) {
  const credentials = config
    ? `${config.url ?? ''}|${config.api_key ?? ''}|${config.token ?? ''}`
    : '';

  return useQuery({
    queryKey: keys.options(id ?? 0, credentials),
    queryFn: () => api.draftOptions({ id, kind, config: config ?? {} }),
    enabled: config !== null,
    retry: false,
    staleTime: 5 * 60_000,
  });
}

export interface CardStatus {
  state: 'ok' | 'unset' | 'error';
  label: string;
}

export function cardStatus(ready: boolean, result: TestResult | null): CardStatus {
  if (result) return { state: result.ok ? 'ok' : 'error', label: result.message };
  return ready ? { state: 'ok', label: 'Ready' } : { state: 'unset', label: 'Not set' };
}

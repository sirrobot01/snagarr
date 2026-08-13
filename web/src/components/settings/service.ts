import { useMutation, useQuery } from '@tanstack/react-query';
import { ApiError, api } from '../../lib/api';
import { keys, useSaveService } from '../../lib/queries';
import type { Service, ServiceKind, TestResult } from '../../lib/types';
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

export function isLibrary(kind: ServiceKind): boolean {
  return kind === 'plex' || kind === 'emby' || kind === 'jellyfin';
}

export function isArr(kind: ServiceKind): boolean {
  return kind === 'radarr' || kind === 'sonarr';
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

/* POST /services/{id}/test reads the stored record, so pending edits go up
   first or the test would answer for the previous credentials. */
export function useSaveThenTest(service: Service, draft: ServiceDraft): ServiceTest {
  const test = useMutation({ mutationFn: () => api.testService(service.id) });
  const save = useSaveService();

  async function saveThenTest() {
    if (draft.dirty) {
      try {
        await save.mutateAsync({ id: service.id, patch: draft.patch });
      } catch {
        return;
      }
      draft.reset();
    }
    test.mutate();
  }

  return {
    result: toTest(test.status, test.data, test.error),
    pending: test.isPending || save.isPending,
    run: () => void saveThenTest(),
  };
}

export function useTmdbTest(): ServiceTest {
  const test = useMutation({ mutationFn: api.testTmdb });
  return {
    result: toTest(test.status, test.data, test.error),
    pending: test.isPending,
    run: () => test.mutate(),
  };
}

/* The lookup goes through the stored credentials, so it stays disabled until
   the record is saved and complete. */
export function useServiceOptions(id: number, enabled: boolean) {
  return useQuery({
    queryKey: keys.options(id),
    queryFn: () => api.serviceOptions(id),
    enabled,
    retry: false,
    staleTime: 5 * 60_000,
  });
}

export interface CardStatus {
  state: 'ok' | 'unset' | 'error';
  label: string;
}

export function cardStatus(ready: boolean, result: TestResult | null): CardStatus {
  if (result) return { state: result.ok ? 'ok' : 'error', label: result.message.toUpperCase() };
  return ready ? { state: 'ok', label: 'OK' } : { state: 'unset', label: 'NOT SET' };
}

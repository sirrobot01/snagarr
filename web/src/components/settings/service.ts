import { useMutation, useQuery } from '@tanstack/react-query';
import { ApiError, api } from '../../lib/api';
import { keys, useSaveSettings } from '../../lib/queries';
import type { ServiceKey, TestResult } from '../../lib/types';
import type { Draft } from './draft';

export interface ServiceTest {
  result: TestResult | null;
  pending: boolean;
  run: () => void;
}

export function useServiceTest(service: ServiceKey): ServiceTest {
  const test = useMutation({ mutationFn: () => api.testService(service) });

  const result =
    test.status === 'success'
      ? test.data
      : test.status === 'error'
        ? { ok: false, message: test.error instanceof ApiError ? test.error.message : 'network unreachable' }
        : null;

  return { result, pending: test.isPending, run: () => test.mutate() };
}

/* POST /settings/test reads the stored settings, so pending edits go up first
   or the test would answer for the previous credentials. */
export function useSaveThenTest(service: ServiceKey, draft: Draft): ServiceTest {
  const test = useServiceTest(service);
  const save = useSaveSettings();

  async function saveThenTest() {
    if (draft.dirty) {
      try {
        await save.mutateAsync(draft.patch);
      } catch {
        return;
      }
      draft.reset();
    }
    test.run();
  }

  return {
    result: test.result,
    pending: test.pending || save.isPending,
    run: () => void saveThenTest(),
  };
}

export function useServiceOptions(service: ServiceKey, enabled: boolean) {
  return useQuery({
    queryKey: keys.options(service),
    queryFn: () => api.serviceOptions(service),
    enabled,
    retry: false,
    staleTime: 5 * 60_000,
  });
}

export interface CardStatus {
  state: 'ok' | 'unset' | 'error';
  label: string;
}

export function cardStatus(configured: boolean, result: TestResult | null): CardStatus {
  if (result) return { state: result.ok ? 'ok' : 'error', label: result.message.toUpperCase() };
  return configured ? { state: 'ok', label: 'OK' } : { state: 'unset', label: 'NOT SET' };
}

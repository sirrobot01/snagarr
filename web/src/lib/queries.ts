import {
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from '@tanstack/react-query';
import { ApiError, api } from './api';
import { pushToast } from './toast';
import type {
  Item,
  ItemsResponse,
  NewService,
  SearchResponse,
  SearchResult,
  SendTarget,
  Service,
  ServicePatch,
  SettingsPatch,
  UserRef,
} from './types';

export const keys = {
  me: ['me'] as const,
  status: ['status'] as const,
  search: (q: string) => ['search', q] as const,
  items: (archived: boolean) => ['items', archived] as const,
  item: (id: number) => ['item', id] as const,
  settings: ['settings'] as const,
  users: ['users'] as const,
  tokens: (userId: number) => ['tokens', userId] as const,
  services: ['services'] as const,
  userServices: (userId: number) => ['services', 'user', userId] as const,
  options: (id: number) => ['options', id] as const,
};

const TEMP_ID = -1;

/** `query` is what the user actually typed; the server keeps it as `raw_input`. */
export interface SnagInput {
  result: SearchResult;
  query: string;
}

export function useMe() {
  return useQuery({
    queryKey: keys.me,
    queryFn: api.me,
    staleTime: Infinity,
    retry: (count, error) => !(error instanceof ApiError) && count < 2,
  });
}

export function useStatus() {
  return useQuery({ queryKey: keys.status, queryFn: api.status, refetchInterval: 60_000 });
}

export function useSearch(q: string) {
  return useQuery({
    queryKey: keys.search(q),
    queryFn: () => api.search(q),
    enabled: q.length > 1,
    staleTime: 5 * 60_000,
    placeholderData: (prev) => prev,
  });
}

export function useItems(archived = false) {
  return useQuery({
    queryKey: keys.items(archived),
    queryFn: () => api.items({ archived, limit: 500 }),
    staleTime: 30_000,
  });
}

export function useItem(id: number | null) {
  return useQuery({
    queryKey: keys.item(id ?? 0),
    queryFn: () => api.item(id as number),
    enabled: id !== null,
  });
}

function optimisticItem(result: SearchResult): Item {
  return {
    id: TEMP_ID,
    tmdb_id: result.tmdb_id,
    media_type: result.media_type,
    title: result.title,
    year: result.year,
    poster_path: result.poster_path,
    status: 'new',
    archived: false,
    raw_input: result.title,
    source: 'web',
    source_url: null,
    note: null,
    captured_by: null,
    captured_at: new Date().toISOString(),
    resolved_at: null,
    available_at: null,
    overview: result.overview,
    runtime: null,
    genres: null,
    candidates: null,
  };
}

/* Mark every cached search row for this title as captured, so the row action
   flips to ✓ before the request lands. */
function markSearchRows(client: QueryClient, tmdbId: number, itemId: number | null) {
  client.setQueriesData<SearchResponse>({ queryKey: ['search'] }, (prev) =>
    prev
      ? {
          results: prev.results.map((r) =>
            r.tmdb_id === tmdbId ? { ...r, item_id: itemId } : r,
          ),
        }
      : prev,
  );
}

function replaceItem(client: QueryClient, id: number, next: Item | null) {
  client.setQueryData<ItemsResponse>(keys.items(false), (prev) => {
    if (!prev) return prev;
    const items = next
      ? prev.items.map((i) => (i.id === id ? next : i))
      : prev.items.filter((i) => i.id !== id);
    return { items, total: prev.total + (next ? 0 : -1) };
  });
}

function failed(action: string, error: unknown) {
  const message = error instanceof ApiError ? error.message : 'network unreachable';
  pushToast(`${action.charAt(0).toUpperCase()}${action.slice(1)} failed — ${message}`);
}

export function useSnag() {
  const client = useQueryClient();

  return useMutation({
    mutationFn: ({ result, query }: SnagInput) =>
      api.capture({
        tmdb_id: result.tmdb_id,
        media_type: result.media_type,
        query,
        source: 'web',
      }),
    onMutate: async ({ result }) => {
      await client.cancelQueries({ queryKey: keys.items(false) });
      const previousItems = client.getQueryData<ItemsResponse>(keys.items(false));
      const previousSearch = client.getQueriesData<SearchResponse>({ queryKey: ['search'] });

      client.setQueryData<ItemsResponse>(keys.items(false), (prev) =>
        prev
          ? { items: [optimisticItem(result), ...prev.items], total: prev.total + 1 }
          : { items: [optimisticItem(result)], total: 1 },
      );
      markSearchRows(client, result.tmdb_id, TEMP_ID);

      return { previousItems, previousSearch };
    },
    onError: (error, _input, context) => {
      if (context?.previousItems) client.setQueryData(keys.items(false), context.previousItems);
      for (const [key, data] of context?.previousSearch ?? []) client.setQueryData(key, data);
      failed('snag', error);
    },
    onSuccess: (item, { result }) => {
      replaceItem(client, TEMP_ID, item);
      markSearchRows(client, result.tmdb_id, item.id);

      const label = item.year ? `${item.title} (${item.year})` : item.title;
      pushToast(`Snagged — ${label}`, {
        label: 'Undo',
        run: () => undoSnag(client, item, result.tmdb_id),
      });
    },
    onSettled: () => {
      void client.invalidateQueries({ queryKey: keys.items(false) });
      void client.invalidateQueries({ queryKey: keys.status });
    },
  });
}

async function undoSnag(client: QueryClient, item: Item, tmdbId: number) {
  replaceItem(client, item.id, null);
  markSearchRows(client, tmdbId, null);
  try {
    await api.remove(item.id);
  } catch (error) {
    failed('undo', error);
  }
  void client.invalidateQueries({ queryKey: keys.items(false) });
  void client.invalidateQueries({ queryKey: keys.status });
}

function useItemMutation<V>(
  action: string,
  run: (item: Item, value: V) => Promise<Item | void>,
  patch: (item: Item, value: V) => Item | null,
) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: ({ item, value }: { item: Item; value: V }) => run(item, value),
    onMutate: async ({ item, value }) => {
      await client.cancelQueries({ queryKey: keys.items(false) });
      const previous = client.getQueryData<ItemsResponse>(keys.items(false));
      replaceItem(client, item.id, patch(item, value));
      client.setQueryData(keys.item(item.id), patch(item, value) ?? undefined);
      return { previous };
    },
    onError: (error, _vars, context) => {
      if (context?.previous) client.setQueryData(keys.items(false), context.previous);
      failed(action, error);
    },
    onSettled: () => {
      void client.invalidateQueries({ queryKey: ['items'] });
      void client.invalidateQueries({ queryKey: keys.status });
    },
  });
}

export function useSend() {
  return useItemMutation<SendTarget>(
    'send',
    (item, target) => api.send(item.id, target),
    (item, target) => ({ ...item, status: target === 'overseerr' ? 'requested' : 'monitored' }),
  );
}

export function useArchive() {
  return useItemMutation<boolean>(
    'archive',
    (item, archived) => api.archive(item.id, archived),
    (item, archived) => (archived ? null : { ...item, archived: false }),
  );
}

export function useDeleteItem() {
  return useItemMutation<void>(
    'delete',
    (item) => api.remove(item.id),
    () => null,
  );
}

export function useResolve() {
  const client = useQueryClient();

  return useMutation({
    mutationFn: ({ item, candidate }: { item: Item; candidate: { tmdb_id: number; media_type: 'movie' | 'tv' } }) =>
      api.resolve(item.id, candidate.tmdb_id, candidate.media_type),
    onMutate: async ({ item, candidate }) => {
      await client.cancelQueries({ queryKey: keys.items(false) });
      const previous = client.getQueryData<ItemsResponse>(keys.items(false));
      replaceItem(client, item.id, { ...item, status: 'new', tmdb_id: candidate.tmdb_id });
      return { previous };
    },
    onError: (error, _vars, context) => {
      if (context?.previous) client.setQueryData(keys.items(false), context.previous);
      failed('resolve', error);
    },
    onSettled: () => {
      void client.invalidateQueries({ queryKey: ['items'] });
      void client.invalidateQueries({ queryKey: keys.status });
    },
  });
}

export function useCaptureQuery() {
  const client = useQueryClient();

  return useMutation({
    mutationFn: (value: string) =>
      api.capture(/^https?:\/\//i.test(value) ? { url: value, source: 'web' } : { query: value, source: 'web' }),
    onError: (error) => failed('capture', error),
    onSuccess: (item) => {
      pushToast(`Snagged — ${item.title}`);
      void client.invalidateQueries({ queryKey: ['items'] });
    },
  });
}

/* ── Admin ──────────────────────────────────────────────────────────────── */

export function useSettings(enabled: boolean) {
  return useQuery({ queryKey: keys.settings, queryFn: api.settings, enabled });
}

export function useSaveSettings() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (patch: SettingsPatch) => api.saveSettings(patch),
    onSuccess: (settings) => client.setQueryData(keys.settings, settings),
    onError: (error) => failed('save', error),
  });
}

export function useUsers(enabled: boolean) {
  return useQuery({ queryKey: keys.users, queryFn: api.users, enabled });
}

export function isAdmin(me: UserRef | undefined) {
  return me?.role === 'admin';
}

/* ── Services ─────────────────────────────────────────────────────────────── */

/** Every member owns their own services, so this needs no role check. */
export function useServices() {
  return useQuery({ queryKey: keys.services, queryFn: api.services });
}

/** Admin only: another member's stack, read-only. */
export function useUserServices(userId: number | null) {
  return useQuery({
    queryKey: keys.userServices(userId ?? 0),
    queryFn: () => api.userServices(userId as number),
    enabled: userId !== null,
  });
}

/* Creating, editing or deleting a service changes what the household can
   reach, so the status badges have to be refetched with the list. */
function useServiceMutation<V, R = unknown>(action: string, run: (value: V) => Promise<R>) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: run,
    onError: (error) => failed(action, error),
    onSettled: () => {
      void client.invalidateQueries({ queryKey: keys.services });
      void client.invalidateQueries({ queryKey: keys.status });
    },
  });
}

export function useCreateService() {
  return useServiceMutation<NewService, Service>('add service', api.createService);
}

export function useSaveService() {
  return useServiceMutation<{ id: number; patch: ServicePatch }>('save', ({ id, patch }) =>
    api.updateService(id, patch),
  );
}

export function useDeleteService() {
  return useServiceMutation<number>('delete service', api.deleteService);
}

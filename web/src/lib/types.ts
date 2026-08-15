/* Mirrors docs/api.md. */

export type MediaType = 'movie' | 'tv';
export type Status =
  | 'needs_review'
  | 'new'
  | 'monitored'
  | 'requested'
  | 'available'
  | 'watched';
export type Role = 'admin' | 'member';
export type Source = 'web' | 'shortcut' | 'telegram' | 'bookmarklet' | 'api' | 'cli';
export type SendTarget = 'radarr' | 'sonarr' | 'overseerr';

export interface UserRef {
  id: number;
  username: string;
  role: Role;
}

export interface Candidate {
  tmdb_id: number;
  media_type: MediaType;
  title: string;
  year: number | null;
  poster_path: string | null;
  overview: string | null;
  score: number;
}

export interface Item {
  id: number;
  tmdb_id: number | null;
  media_type: MediaType | '';
  title: string;
  year: number | null;
  poster_path: string | null;
  status: Status;
  archived: boolean;
  raw_input: string;
  source: Source;
  source_url: string | null;
  note: string | null;
  captured_by: UserRef | null;
  captured_at: string;
  resolved_at: string | null;
  available_at: string | null;
  overview: string | null;
  runtime: number | null;
  genres: string[] | null;
  candidates: Candidate[] | null;
}

export interface SearchResult {
  tmdb_id: number;
  media_type: MediaType;
  title: string;
  year: number | null;
  poster_path: string | null;
  overview: string | null;
  state: Status;
  item_id: number | null;
  from: 'library' | 'tmdb';
}

export interface ItemsResponse {
  items: Item[];
  total: number;
}

export interface SearchResponse {
  results: SearchResult[];
}

/* Send `tmdb_id` + `media_type` to capture an exact title — the server resolves
   inline and returns the finished item. Send `query` or `url` on its own to
   capture free text, which resolves in the background. */
export interface CapturePayload {
  tmdb_id?: number;
  media_type?: MediaType;
  query?: string;
  url?: string;
  source?: Source;
  note?: string | null;
  source_url?: string | null;
}

export interface StatusResponse {
  version: string;
  counts: {
    total: number;
    ready: number;
    pending: number;
    needs_review: number;
    archived: number;
  };
  sync: {
    library_at: string | null;
    arr_at: string | null;
    collection_at: string | null;
    running: boolean;
  };
  services: Record<string, boolean>;
}

export interface HouseholdUser {
  id: number;
  username: string;
  role: Role;
  telegram_user_id: number | null;
  token_count: number;
  created_at: string;
}

export interface Token {
  id: number;
  name: string;
  prefix: string;
  created_at: string;
  last_used_at: string | null;
  revoked: boolean;
}

export interface CreatedToken {
  id: number;
  name: string;
  token: string;
  created_at: string;
}

/* ── Settings ───────────────────────────────────────────────────────────────
   Only TMDB and General are global now. Every other integration is a service
   record owned by a household member. */

export interface TmdbSettings {
  api_key: string;
  configured: boolean;
  locked?: boolean;
  /** True when this build carries Snagarr's shared key, making the field an
      optional override rather than a requirement. */
  builtin_key?: boolean;
}

export interface GeneralSettings {
  /** Go duration string. The reconcile loop re-reads it, so a change needs no restart. */
  reconcile_interval: string;
  public_url: string;
  /** Hands a resolved capture to the capturer's own Radarr or Sonarr. On by default. */
  auto_send: boolean;
  configured: boolean;
  locked?: boolean;
}

export interface Settings {
  tmdb: TmdbSettings;
  general: GeneralSettings;
}

export type SettingsPatch = {
  [K in keyof Settings]?: Partial<Settings[K]>;
};

/* ── Services ─────────────────────────────────────────────────────────────── */

/** The three media servers differ only in how their client is built. */
export type LibraryKind = 'plex' | 'emby' | 'jellyfin';
/** Radarr and Sonarr share every config field except season folders. */
export type ArrKind = 'radarr' | 'sonarr';

export type ServiceKind = LibraryKind | ArrKind | 'overseerr' | 'ntfy';

/** The union of every kind's config document. Which keys a kind uses is fixed
    by `internal/config/services.go`; the server fills in the rest as defaults. */
export interface ServiceConfig {
  url?: string;
  /** Plex/Emby/Jellyfin credential. Masked on the way out. */
  token?: string;
  /** Radarr/Sonarr/Overseerr credential. Masked on the way out. */
  api_key?: string;
  /** Go marshals an empty []string as null, so this arrives unset or null. */
  section_ids?: string[] | null;
  collection_name?: string;
  quality_profile_id?: number;
  root_folder?: string;
  season_folder?: boolean;
  search_on_add?: boolean;
  topic?: string;
  priority?: number;
}

export interface Service {
  id: number;
  user_id: number;
  kind: ServiceKind;
  name: string;
  config: ServiceConfig;
  enabled: boolean;
  /** An environment variable pins this record; every restart rewrites it. */
  locked: boolean;
  created_at: string;
  updated_at: string;
}

export interface NewService {
  kind: ServiceKind;
  name: string;
  config?: ServiceConfig;
  enabled?: boolean;
}

export interface ServicePatch {
  name?: string;
  config?: ServiceConfig;
  enabled?: boolean;
}

export interface TestResult {
  ok: boolean;
  message: string;
}

export interface ServiceOptions {
  quality_profiles?: { id: number; name: string }[];
  root_folders?: { path: string; free_space: number }[];
  sections?: { id: string; title: string; type: string }[];
}

/* ── Plex sign-in ─────────────────────────────────────────────────────────── */

export interface PlexPin {
  id: number;
  code: string;
  auth_url: string;
  expires_at: string;
}

/** 200 carries `token`; 202 carries `status: "pending"`; 410 is an error. */
export interface PlexPinCheck {
  token?: string;
  status?: string;
}

export interface PlexConnection {
  uri: string;
  local: boolean;
  relay: boolean;
  /** True when this server answered snagarr on this address. */
  reachable: boolean;
}

export interface PlexServer {
  name: string;
  client_identifier: string;
  /** Ordered best-first: answers before silence, local before relay. Every
      returned route remains selectable. */
  connections: PlexConnection[];
}

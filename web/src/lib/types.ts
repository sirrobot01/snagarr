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
  display_name: string;
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
  display_name: string;
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

export type ServiceKey =
  | 'tmdb'
  | 'library'
  | 'radarr'
  | 'sonarr'
  | 'overseerr'
  | 'ntfy'
  | 'telegram';

export interface TmdbSettings {
  api_key: string;
  configured: boolean;
  locked?: boolean;
}
export interface LibrarySettings {
  provider: 'plex' | 'emby' | 'jellyfin' | '';
  url: string;
  token: string;
  section_ids: string[];
  collection_name: string;
  configured: boolean;
  locked?: boolean;
}
export interface ArrSettings {
  url: string;
  api_key: string;
  quality_profile_id: number | null;
  root_folder: string;
  season_folder?: boolean;
  search_on_add: boolean;
  configured: boolean;
  locked?: boolean;
}
export interface OverseerrSettings {
  url: string;
  api_key: string;
  prefer: boolean;
  configured: boolean;
  locked?: boolean;
}
export interface NtfySettings {
  url: string;
  topic: string;
  token: string;
  priority: number;
  configured: boolean;
  locked?: boolean;
}
export interface TelegramSettings {
  bot_token: string;
  configured: boolean;
  locked?: boolean;
}
export interface GeneralSettings {
  reconcile_interval: string;
  stale_days: number;
  public_url: string;
  image_base: string;
}

export interface Settings {
  tmdb: TmdbSettings;
  library: LibrarySettings;
  radarr: ArrSettings;
  sonarr: ArrSettings;
  overseerr: OverseerrSettings;
  ntfy: NtfySettings;
  telegram: TelegramSettings;
  general: GeneralSettings;
}

export type SettingsPatch = {
  [K in keyof Settings]?: Partial<Settings[K]>;
};

export interface TestResult {
  ok: boolean;
  message: string;
}

export interface ServiceOptions {
  quality_profiles?: { id: number; name: string }[];
  root_folders?: { path: string; free_space: number }[];
  sections?: { id: string; title: string; type: string }[];
}

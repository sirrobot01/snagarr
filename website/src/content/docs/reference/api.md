---
title: HTTP API
description: Every /api/v1 endpoint, the item, service and settings types, the error codes and the role each route needs.
---

Base path `/api/v1`. All bodies are JSON. All timestamps are RFC 3339 UTC.

## Endpoints

| Method | Path | Role |
|--------|------|------|
| `GET` | `/health` | none |
| `POST` | `/webhooks/{service}` | secret in query |
| `GET` | `/me` | any token |
| `GET` | `/status` | any token |
| `POST` | `/capture` | any token |
| `GET` | `/search` | any token |
| `GET` | `/items` | any token |
| `GET` | `/items/{id}` | any token |
| `POST` | `/items/{id}/resolve` | own item, or admin |
| `POST` | `/items/{id}/archive` | own item, or admin |
| `DELETE` | `/items/{id}` | own item, or admin |
| `POST` | `/items/{id}/send` | any token, own services |
| `POST` | `/plex/pin` | any token |
| `GET` | `/plex/pin/{id}` | any token |
| `GET` | `/plex/servers` | any token |
| `GET` `POST` | `/services` | any token |
| `PATCH` `DELETE` | `/services/{id}` | owner, or admin |
| `POST` | `/services/{id}/test` | owner, or admin |
| `GET` | `/services/{id}/options` | owner, or admin |
| `GET` `POST` | `/users` | admin |
| `PATCH` `DELETE` | `/users/{id}` | admin |
| `GET` `POST` | `/users/{id}/tokens` | admin |
| `GET` | `/users/{id}/services` | admin |
| `DELETE` | `/tokens/{id}` | admin |
| `GET` `PUT` | `/settings` | admin |
| `POST` | `/settings/test` | admin |
| `POST` | `/admin/sync` | admin |

## Authentication

```
Authorization: Bearer sngr_xxxxxxxxxxxxxxxxxxxxxxxx
```

A token belongs to one user. Snagarr stores only its SHA-256 digest. First run prints one admin token and a setup URL that carries it in the fragment: `http://localhost:8080/#token=sngr_…`.

`/api/v1/webhooks/*` takes no bearer token. It authenticates with `?secret=`.

A `member` may capture, read, build their own services, send to their own services, and resolve, archive or delete the items they captured. Everything else is admin-only and answers `403 forbidden`.

## Errors

```json
{ "error": { "code": "not_found", "message": "item 42 does not exist" } }
```

| Status | Code |
|--------|------|
| 400 | `bad_request` |
| 401 | `unauthorized` |
| 403 | `forbidden` |
| 404 | `not_found` |
| 409, 410 | `conflict` |
| 422 | `unresolvable` |
| 500 | `internal_error` |
| 502 | `upstream_error` |
| 503 | `not_configured` |

An unknown field in a request body is `400 bad_request`.

## Types

```ts
type MediaType = "movie" | "tv";
type Status = "needs_review" | "new" | "monitored" | "requested" | "available" | "watched";
type Role = "admin" | "member";
type Source = "web" | "shortcut" | "telegram" | "bookmarklet" | "api" | "cli";
type ServiceKind = "plex" | "emby" | "jellyfin" | "radarr" | "sonarr" | "overseerr" | "ntfy";

interface UserRef { id: number; display_name: string; role: Role }

interface Item {
  id: number;
  tmdb_id: number | null;      // null while needs_review
  media_type: MediaType | "";
  title: string;               // the raw input until the item resolves
  year: number | null;
  poster_path: string | null;  // TMDB path, e.g. "/abc.jpg"
  status: Status;
  archived: boolean;
  raw_input: string;           // never discarded
  source: Source;
  source_url: string | null;
  note: string | null;
  captured_by: UserRef | null;
  captured_at: string;
  resolved_at: string | null;
  available_at: string | null;
  overview: string | null;     // from the entity cache
  runtime: number | null;
  genres: string[] | null;
  candidates: Candidate[] | null; // on GET /items/{id} when needs_review
}

interface Candidate {
  tmdb_id: number; media_type: MediaType; title: string;
  year: number | null; poster_path: string | null;
  overview: string | null; score: number; // 0..1
}

interface SearchResult {
  tmdb_id: number; media_type: MediaType; title: string;
  year: number | null; poster_path: string | null; overview: string | null;
  state: Status;               // composite state, answered from local indexes
  item_id: number | null;      // set when this title is already snagged
  from: "library" | "tmdb";    // which tier produced the row
}

interface Service {
  id: number;
  user_id: number;             // the owner. It never changes
  kind: ServiceKind;
  name: string;
  config: ServiceConfig;       // the kind's own document. Secrets masked
  enabled: boolean;
  locked: boolean;             // an environment variable rewrites it on every start
  created_at: string;
  updated_at: string;
}

// One shape per kind. A kind uses a subset of these keys.
interface ServiceConfig {
  url?: string;                // every kind except a bare ntfy topic
  token?: string;              // plex, emby, jellyfin, ntfy. Masked
  api_key?: string;            // radarr, sonarr, overseerr. Masked
  section_ids?: string[] | null;  // media servers
  collection_name?: string;       // media servers
  quality_profile_id?: number;    // radarr, sonarr
  root_folder?: string;           // radarr, sonarr
  search_on_add?: boolean;        // radarr, sonarr
  season_folder?: boolean;        // sonarr
  topic?: string;                 // ntfy
  priority?: number;              // ntfy, 1..5
}
```

`telegram` survives in `Source` for items captured before the bot was dropped. Snagarr runs no Telegram integration.

`state` drives the single badge on a row:

| `state` | Badge | Class |
|---------|-------|-------|
| `available`, `watched` | `IN LIBRARY` | `.sg-lib` |
| `monitored` | `MONITORED` | `.sg-mon` |
| `requested` | `REQUESTED` | `.sg-req` |
| `needs_review` | `NEEDS REVIEW` | `.sg-rev` |
| `new` | `NEW` | `.sg-new` |

Every state is the household union. A title reads `available` when any member's media server holds it. See [Services](/snagarr/configure/services/#union-state-personal-action).

## Capture

### `POST /capture`

`source` defaults to `api`.

Exact identity — send `tmdb_id` and `media_type`:

```json
{ "tmdb_id": 1233413, "media_type": "movie", "source": "web", "query": "sinn" }
```

Resolves inline. `201 Created` for a new item, `200 OK` with the existing one when that title is already snagged. Capture is idempotent per TMDB ID. Send the typed text as `query`; it is kept as `raw_input`.

Free text or a link — send `query` or `url`:

```json
{ "query": "that vampire one w/ dafoe", "source": "shortcut" }
```

`202 Accepted` with a `needs_review` item whose title is the raw input. Resolution runs in the background: known links (TMDB, IMDB) resolve by ID, other pages are scraped for hints, free text goes to TMDB search with confidence scoring. Poll `GET /items/{id}`.

A capture is never rejected for being unidentifiable. An unresolvable input stays `needs_review` with its candidates and its raw input.

### `GET /search?q=&limit=`

Library matches rank first, then TMDB by relevance. Each row carries its composite state. Cached 24 h.

```json
{ "results": [ /* SearchResult[] */ ] }
```

## Items

### `GET /items`

| Parameter | Values |
|-----------|--------|
| `status` | any `Status` |
| `type` | `movie`, `tv` |
| `q` | free text |
| `captured_by` | user ID |
| `archived` | `true`, `false` (default `false`) |
| `limit` | default 100, max 500 |
| `offset` | integer |

```json
{ "items": [ /* Item[] */ ], "total": 24 }
```

### `GET /items/{id}`

One item, with `candidates` when it needs review.

### `POST /items/{id}/resolve`

```json
{ "tmdb_id": 1233413, "media_type": "movie" }
```

Resolves a `needs_review` item to a candidate, or re-points a resolved one. Recomputes state. Returns the item.

### `POST /items/{id}/send`

```json
{ "target": "radarr" }
```

`target` is `radarr`, `sonarr` or `overseerr`. Adds the title with that service's quality profile and root folder, then sets the item to `monitored`, or `requested` for Overseerr. Returns the item.

The send is personal. A member spends their own service and nobody else's; with none of that kind, the answer is `503 not_configured`. An admin falls through to their own service, then the capturer's, then another admin's.

A title the target already tracks counts as success.

### `POST /items/{id}/archive`

```json
{ "archived": true }
```

Hides the item from the default list and keeps its history.

### `DELETE /items/{id}`

Removes the item and its candidates. `204 No Content`.

## Services

Every integration except TMDB is a service owned by one member. See [Services](/snagarr/configure/services/).

### `GET /services`

The caller's own services, disabled ones included.

```json
{ "services": [
  { "id": 3, "user_id": 2, "kind": "radarr", "name": "Default",
    "config": { "url": "http://radarr.lan:7878", "api_key": "••••4e2a",
                "quality_profile_id": 4, "root_folder": "/movies", "search_on_add": true },
    "enabled": true, "locked": false,
    "created_at": "2026-07-01T09:12:00Z", "updated_at": "2026-07-01T09:12:00Z" }
] }
```

### `POST /services`

```json
{ "kind": "radarr", "name": "Mine",
  "config": { "url": "http://radarr.lan:7878", "api_key": "your_key" } }
```

`201 Created` with the service. The caller owns it. `name` defaults to `Default`. Snagarr merges your `config` over the kind's defaults, so a new media server already reads `"collection_name": "Snagged"`.

| Error | Cause |
|-------|-------|
| `400 bad_request` | `kind` is not one Snagarr knows |
| `409 conflict` | You already own a service of that kind with that name |

### `PATCH /services/{id}` · `DELETE /services/{id}`

```json
{ "name": "Mine", "enabled": false, "config": { "root_folder": "/data/movies" } }
```

`PATCH` merges `config` over the stored document; an omitted field keeps its value, and an echoed mask keeps the stored secret. It returns the service. `DELETE` answers `204 No Content` and takes that service's index rows with it.

The owner reaches both. An admin reaches anybody's. Any other caller gets `403 forbidden`.

### `POST /services/{id}/test`

```json
{ "ok": true, "message": "OK · 611 monitored" }
```

`ok: false` carries the upstream failure in `message`, for example `401 — check token`. The test reads the **stored** record, so save an edit before you test it.

### `GET /services/{id}/options`

```json
{ "quality_profiles": [ { "id": 4, "name": "HD-1080p" } ],
  "root_folders":     [ { "path": "/movies", "free_space": 812340000000 } ] }
```

A `radarr` or `sonarr` service answers with the pair above. A `plex`, `emby` or `jellyfin` service answers `{ "sections": [ { "id": "1", "title": "Movies", "type": "movie" } ] }`. Any other kind answers `400 bad_request`.

### `GET /users/{id}/services` — admin

One member's whole stack, disabled services included. Same shape as `GET /services`.

## Plex sign-in

The three routes turn a plex.tv sign-in into a URL and a token, so nobody has to find an `X-Plex-Token` by hand.

### `POST /plex/pin`

```json
{ "id": 4471223, "code": "H7QK", "auth_url": "https://app.plex.tv/auth#…",
  "expires_at": "2026-07-01T09:27:00Z" }
```

### `GET /plex/pin/{id}`

Poll it while the user signs in.

| Status | Body |
|--------|------|
| `202` | `{ "status": "pending" }` |
| `200` | `{ "status": "linked", "token": "…" }` |
| `410` | `conflict` — the sign-in expired |

### `GET /plex/servers?token=`

```json
{ "servers": [ { "name": "Home", "client_identifier": "abc…",
                 "connections": [ { "uri": "http://plex.lan:32400", "local": true, "relay": false } ] } ] }
```

Connections come back fastest first. Store the head with the token in a `plex` service.

## Users and tokens — admin

### `GET /me`

Not admin-gated. Every client calls it once at start-up to learn its role.

```json
{ "id": 1, "display_name": "Mukhtar", "role": "admin" }
```

### `GET /users` · `POST /users`

```json
{ "users": [ { "id": 1, "display_name": "Mukhtar", "role": "admin",
               "telegram_user_id": null, "token_count": 3,
               "created_at": "2026-07-01T09:12:00Z" } ] }
```

```json
{ "display_name": "Amina", "role": "member" }
```

`telegram_user_id` is accepted and stored. Nothing reads it.

### `PATCH /users/{id}` · `DELETE /users/{id}`

`PATCH` accepts the same fields as `POST`. Deleting a user keeps their items and nulls the attribution. It deletes their services and every index row those services own. The last admin cannot be deleted or demoted: `409 conflict`.

### `GET /users/{id}/tokens` · `POST /users/{id}/tokens`

```json
{ "tokens": [ { "id": 7, "name": "iPhone Shortcut", "prefix": "sngr_a1b2",
                "created_at": "…", "last_used_at": "…", "revoked": false } ] }
```

```json
{ "name": "iPhone Shortcut" }
```

The response is the only time the raw token is readable:

```json
{ "id": 7, "name": "iPhone Shortcut", "token": "sngr_a1b2c3…", "created_at": "…" }
```

### `DELETE /tokens/{id}`

Revokes one token. `204 No Content`.

## Settings — admin

Settings carry the TMDB key and the install-wide knobs. Nothing else. See [Settings](/snagarr/configure/settings/).

### `GET /settings`

```json
{
  "tmdb":    { "api_key": "••••4e2a", "configured": true, "locked": false },
  "general": { "reconcile_interval": "15m0s",
               "public_url": "http://localhost:8080",
               "shortcut_url": "https://www.icloud.com/shortcuts/c4b4dabe0b55481c9fe35fac0a4a266b",
               "webhook_secret": "3f8c1e…", "configured": true, "locked": false }
}
```

Each section carries `configured` (the section holds enough to work) and `locked` (an environment variable pins a value in that section, so the UI renders the card read-only). Locking is reported per section, not per field. `general.configured` is always `true`.

`tmdb.api_key` comes back as `••••` plus the last four characters. `general.webhook_secret` returns in clear text, because you must paste it into Radarr and Tautulli.

Durations round-trip in Go format: `"15m"` is accepted and returns as `"15m0s"`.

### `PUT /settings`

Accepts a partial object of the same shape and merges it over the current values. Returns the full settings. Send a masked secret back unchanged to keep the stored value. `configured` and `locked` are ignored on input, so a client may echo a whole `GET /settings` response.

### `POST /settings/test`

```json
{ "service": "tmdb" }
```

```json
{ "ok": true, "message": "OK" }
```

`tmdb` is the only accepted name; anything else is `400 bad_request`. Test every other integration with `POST /services/{id}/test`.

## Operations

### `GET /status`

```json
{
  "version": "0.1.0",
  "counts": { "total": 24, "ready": 6, "pending": 15, "needs_review": 3, "archived": 2 },
  "sync": { "library_at": "…", "arr_at": "…", "collection_at": "…", "running": false },
  "shortcut_url": "https://www.icloud.com/shortcuts/c4b4dabe0b55481c9fe35fac0a4a266b",
  "services": { "tmdb": true, "library": true, "radarr": true, "sonarr": true,
                "overseerr": false, "ntfy": true }
}
```

`services` is household-wide: each flag is true when **anybody** has a working service of that kind. It never says whose.

`shortcut_url` repeats `general.shortcut_url`, because `/settings` is admin-only and a member still needs the link.

`sync.library_at` is the most recent sweep across every media server. A sync that has never run reports `null`.

### `POST /admin/sync` — admin

Forces an index and collection reconcile. `202 Accepted`. Watch `sync.running` in `GET /status`.

### `GET /health`

No auth. `{ "status": "ok", "version": "0.1.0" }`.

## Webhooks

`?secret=` must match `general.webhook_secret`.

| Endpoint | Sender | Effect |
|----------|--------|--------|
| `POST /webhooks/radarr` | Radarr | Marks the item available, syncs the collections, notifies |
| `POST /webhooks/sonarr` | Sonarr | The same, for episodes |
| `POST /webhooks/tautulli` | Tautulli | Marks the item watched |
| `POST /webhooks/emby` | Emby | Marks the item watched |
| `POST /webhooks/jellyfin` | Jellyfin | Marks the item watched |

Import webhooks act on `Download`, `MovieFileImported`, `EpisodeFileImported` and `Import`. Playback webhooks act on any payload that carries a TMDB ID, so trigger them on playback stop, not playback start. All return `204 No Content`, including for a payload that matches nothing. Full field list: [Webhooks](/snagarr/use/webhooks/#fields-snagarr-reads).

## CORS

Snagarr answers preflight `OPTIONS` with `204` on `/api/v1`, allows any origin and allows the `Authorization` and `Content-Type` headers. It never authenticates with a cookie. Add no CORS headers at a reverse proxy; a duplicate `Access-Control-Allow-Origin` makes the browser reject the response.

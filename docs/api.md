# Snagarr HTTP API (v1)

The API is the product. The web app is one client; the Shortcut, the bookmarklet
and the Telegram bot are others. All of them use the same endpoints.

Base path: `/api/v1`. All bodies are JSON. All timestamps are RFC 3339 UTC.

## Authentication

Send a bearer token on every request:

```
Authorization: Bearer sngr_xxxxxxxxxxxxxxxxxxxxxxxx
```

A token belongs to one user. The first run creates the admin user, prints one
token, and prints a setup URL that carries the token in the fragment:

```
http://localhost:8080/#token=sngr_...
```

The web app reads that fragment, stores the token, and removes it from the URL.

Webhook endpoints under `/api/v1/webhooks/*` do not use bearer auth. Each one
takes a per-endpoint secret in the query string instead, because Radarr,
Tautulli and Emby cannot all set headers.

## Roles

`admin` may do everything. `member` may capture, read, and resolve items they
captured. These actions are admin-only:

- send an item to Radarr, Sonarr or Overseerr
- archive or delete an item another user captured
- read or write settings, users and tokens
- force a reconcile

A `member` calling an admin route gets `403 forbidden`.

## Errors

```json
{ "error": { "code": "not_found", "message": "item 42 does not exist" } }
```

| Status | Code |
|--------|------|
| 400 | `bad_request`, `invalid_query` |
| 401 | `unauthorized` |
| 403 | `forbidden` |
| 404 | `not_found` |
| 409 | `conflict` |
| 422 | `unresolvable` |
| 502 | `upstream_error` |
| 503 | `not_configured` |

## Types

```ts
type MediaType = "movie" | "tv";
type Status = "needs_review" | "new" | "monitored" | "requested" | "available" | "watched";
type Role = "admin" | "member";
type Source = "web" | "shortcut" | "telegram" | "bookmarklet" | "api" | "cli";

interface UserRef { id: number; display_name: string; role: Role }

interface Item {
  id: number;
  tmdb_id: number | null;      // null while needs_review
  media_type: MediaType | "";
  title: string;               // the raw input until the item resolves
  year: number | null;
  poster_path: string | null;  // TMDB path, e.g. "/abc.jpg". Prefix with image_base.
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
  candidates: Candidate[] | null; // present on GET /items/{id} when needs_review
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
```

`state` drives the single badge on every row:

| `state` | Badge | Class |
|---------|-------|-------|
| `available`, `watched` | `IN LIBRARY` | `.sg-lib` |
| `monitored` | `MONITORED` | `.sg-mon` |
| `requested` | `REQUESTED` | `.sg-req` |
| `needs_review` | `NEEDS REVIEW` | `.sg-rev` |
| `new` | `NEW` | `.sg-new` |

## Capture

### `POST /capture`

```json
{ "query": "sinners", "source": "web", "note": null, "source_url": null }
```

Send either `query` or `url`, not both. `source` defaults to `api`.

Responds `202 Accepted` with the created item. Resolution runs asynchronously,
so the item usually comes back with `status: "needs_review"` and the raw input
as its title. Poll `GET /items/{id}` or refetch the list.

When the query resolves to a title that is already snagged, the existing item is
returned with `200 OK` instead — capture is idempotent per TMDB ID.

### `GET /search?q=&limit=`

Merged search. Library matches rank first, then TMDB by relevance. Results carry
their composite state so the UI can badge before the user adds. Cached 24 h.

```json
{ "results": [ /* SearchResult[] */ ] }
```

## Items

### `GET /items`

Query parameters, all optional: `status`, `type`, `q`, `captured_by`,
`archived` (`true`/`false`, default `false`), `limit` (default 100, max 500),
`offset`.

```json
{ "items": [ /* Item[] */ ], "total": 24 }
```

### `GET /items/{id}`

One item, including `candidates` when it needs review.

### `POST /items/{id}/resolve`

```json
{ "tmdb_id": 1233413, "media_type": "movie" }
```

Resolves a `needs_review` item to a chosen candidate, or re-points a resolved
one. Recomputes state. Returns the item.

### `POST /items/{id}/send` — admin

```json
{ "target": "radarr" }
```

`target` is `radarr`, `sonarr` or `overseerr`. Adds the title to that service
using the configured quality profile and root folder, then sets the item to
`monitored` (or `requested` for Overseerr). Returns the item.

### `POST /items/{id}/archive`

```json
{ "archived": true }
```

Archiving hides an item from the default list but keeps its history. Members may
only archive items they captured.

### `DELETE /items/{id}` — admin

Removes the item and its candidates. `204 No Content`.

## Users and tokens — admin

### `GET /me`

The user behind the current token. Every client calls this once at start-up to
learn its role. Not admin-gated.

```json
{ "id": 1, "display_name": "Mukhtar", "role": "admin" }
```

### `GET /users`

```json
{ "users": [ { "id": 1, "display_name": "Mukhtar", "role": "admin",
               "telegram_user_id": 44190231, "token_count": 3,
               "created_at": "2026-07-01T09:12:00Z" } ] }
```

### `POST /users`

```json
{ "display_name": "Amina", "role": "member", "telegram_user_id": 8802441 }
```

### `PATCH /users/{id}` · `DELETE /users/{id}`

Patch accepts the same fields. Deleting a user keeps their items and nulls the
attribution. The last admin cannot be deleted or demoted — `409 conflict`.

### `GET /users/{id}/tokens`

```json
{ "tokens": [ { "id": 7, "name": "iPhone Shortcut", "prefix": "sngr_a1b2",
                "created_at": "…", "last_used_at": "…", "revoked": false } ] }
```

### `POST /users/{id}/tokens`

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

### `GET /settings`

Secrets come back masked (`"••••••••4e2a"`) and never in clear text. A masked
value sent back unchanged in a `PUT` leaves the stored secret alone.

```json
{
  "tmdb":    { "api_key": "••••4e2a", "configured": true },
  "library": { "provider": "plex", "url": "http://plex.lan:32400",
               "token": "••••1f7c", "section_ids": ["1","2"],
               "collection_name": "Snagged", "configured": true },
  "radarr":  { "url": "…", "api_key": "••••", "quality_profile_id": 4,
               "root_folder": "/movies", "search_on_add": true, "configured": true },
  "sonarr":  { "url": "…", "api_key": "••••", "quality_profile_id": 3,
               "root_folder": "/tv", "season_folder": true,
               "search_on_add": true, "configured": true },
  "overseerr": { "url": "", "api_key": "", "prefer": false, "configured": false },
  "ntfy":    { "url": "https://ntfy.sh", "topic": "snagarr-home",
               "token": "", "priority": 3, "configured": true },
  "telegram":{ "bot_token": "••••", "configured": true },
  "general": { "reconcile_interval": "15m", "stale_days": 90,
               "public_url": "http://localhost:8080", "image_base": "https://image.tmdb.org/t/p" }
}
```

Fields set through environment variables are additionally marked
`"locked": true` and the UI renders them read-only.

### `PUT /settings`

Accepts a partial object of the same shape. Returns the full settings.

### `POST /settings/test`

```json
{ "service": "radarr" }
```

```json
{ "ok": true, "message": "OK · 611 monitored" }
```

`ok: false` carries the upstream failure in `message` — the settings card shows
it verbatim (`401 — check token`).

### `GET /settings/options?service=radarr`

Live lookup so the UI can offer real choices instead of free-text IDs.

```json
{ "quality_profiles": [ { "id": 4, "name": "HD-1080p" } ],
  "root_folders":     [ { "path": "/movies", "free_space": 812340000000 } ] }
```

For `service=library` it returns `{ "sections": [ { "id": "1", "title": "Movies", "type": "movie" } ] }`.

## Operations

### `GET /status`

Drives the footer strip and the settings page.

```json
{
  "version": "0.1.0",
  "counts": { "total": 24, "ready": 6, "pending": 15, "needs_review": 3, "archived": 2 },
  "sync": { "library_at": "…", "arr_at": "…", "collection_at": "…", "running": false },
  "services": { "tmdb": true, "library": true, "radarr": true,
                "sonarr": true, "overseerr": false, "ntfy": true }
}
```

### `POST /admin/sync` — admin

Forces an index and collection reconcile. Returns `202 Accepted`; watch
`GET /status` for `sync.running`.

### `GET /health`

No auth. `{ "status": "ok", "version": "0.1.0" }`.

## Webhooks

Each endpoint takes `?secret=` matching the value shown in settings.

| Endpoint | Sender | Effect |
|----------|--------|--------|
| `POST /webhooks/radarr` | Radarr | `Download`/`MovieFileImported` marks the item available, syncs the collection and notifies |
| `POST /webhooks/sonarr` | Sonarr | same for episodes |
| `POST /webhooks/tautulli` | Tautulli | playback stop past the watched threshold marks the item watched |
| `POST /webhooks/emby` | Emby or Jellyfin | playback stop marks the item watched |

All return `204 No Content`, including for payloads that match nothing — a
webhook must never make the sender retry.

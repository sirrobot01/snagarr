---
title: Introduction
description: Snagarr captures a film or show, resolves it against TMDB, sends it to Radarr or Sonarr, and puts it in a collection on your media server when the file lands.
---

Snagarr is one Go binary with the React client embedded. All state lives in one SQLite file. It needs no external database, no cache server and no reverse proxy.

| | |
|---|---|
| Listen address | `:8080` |
| Data directory | `data`, or `/data` in the container image |
| API base path | `/api/v1` |
| Authentication | Bearer token, prefix `sngr_` |
| Image | `ghcr.io/sirrobot01/snagarr` |
| Required integration | TMDB API key (v3) |

## Pipeline

1. **Capture.** A client posts free text, a link or a TMDB ID to `POST /api/v1/capture`.
2. **Identify.** The resolver matches the input against TMDB. An input with no clear winner stays `needs_review` with scored candidates attached.
3. **Send.** A member sends the item to their own Radarr, Sonarr or Overseerr, with that service's quality profile and root folder.
4. **Reconcile.** Every 15 minutes Snagarr mirrors every member's media server, Radarr, Sonarr and Overseerr into local indexes, then recomputes each item status.
5. **Notify.** An item that becomes available triggers one ntfy push to the capturer, and enters the `Snagged` collection on every media server that holds it.
6. **Retire.** A playback webhook marks the item watched and removes the title from the collections.

## Per-member services

Every integration except TMDB belongs to one household member. Radarr, Sonarr, Overseerr, Plex, Emby, Jellyfin and ntfy are rows in a `services` table with an owner.

| | |
|---|---|
| **State is the household union** | A title reads `IN LIBRARY` when any member's server holds it. |
| **Actions are personal** | A member sends to their own service, or gets `503`. |
| **Collections are personal** | Each media server gets its own `Snagged` collection, holding only titles that server has. |
| **Pushes are personal** | The availability push goes to the capturer's ntfy. |

Settings hold what stays global: the TMDB key and the install-wide knobs. See [Services](/snagarr/configure/services/).

## Item status

| Status | Meaning |
|--------|---------|
| `needs_review` | The resolver found no confident TMDB match. Candidates are attached. |
| `new` | Resolved. Sent to no service. |
| `monitored` | A Radarr or Sonarr in the household holds the title. |
| `requested` | An Overseerr in the household holds a request for the title. |
| `available` | The file is in somebody's library. |
| `watched` | A playback event reported a watch. |

Each collection holds `(snagged ∩ available) − watched`, limited to the titles that server has. It is live state, not history.

## Local indexes

Snagarr answers every state question from SQLite. No request to Snagarr waits on Plex, Radarr or Sonarr. A failed upstream call is logged and skipped; the stale index keeps answering and the next reconcile pass retries.

Every index row belongs to a service. Disable a service and it stops answering. Delete it and its rows go with it.

## Roles

One shared list with attribution. Each household member has an account, while
roles decide who may change install-wide or destructive settings.

| Action | `admin` | `member` |
|--------|---------|----------|
| Capture, search, read items | yes | yes |
| Resolve, archive, delete **own** items | yes | yes |
| Resolve, archive, delete **another user's** items | yes | no |
| Build and test **own** services | yes | yes |
| Read, edit or delete **another user's** services | yes | no |
| Send to Radarr, Sonarr or Overseerr | own, then the capturer's, then another admin's | own only |
| Read or write settings, users and tokens | yes | no |
| Force a reconcile | yes | no |

A `member` that calls an admin route gets `403 forbidden`.

## Not implemented

| Feature | State |
|---------|-------|
| Telegram | Dropped. No setting, no service kind, no bot. `telegram_user_id` on a user and `telegram` in `Source` are stored and read by nothing. |
| Command line capture | The binary has `serve` and `version` only. |
| Generic webhook ingest | `/api/v1/webhooks/*` accepts import and playback events only. It never creates an item. |

## Next

- [Install](/snagarr/start/install/) — Docker, binary, systemd, reverse proxy.
- [First run](/snagarr/start/first-run/) — account registration and the setup wizard.
- [Services](/snagarr/configure/services/) — who owns what.
- [HTTP API](/snagarr/reference/api/) — every endpoint.

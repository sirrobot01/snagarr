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
3. **Send.** An admin sends the item to Radarr, Sonarr or Overseerr with the configured quality profile and root folder.
4. **Reconcile.** Every 15 minutes Snagarr mirrors the media server, Radarr, Sonarr and Overseerr into local indexes, then recomputes each item status.
5. **Notify.** An item that becomes available triggers one ntfy push and enters the `Snagged` collection.
6. **Retire.** A playback webhook marks the item watched and removes the title from the collection.

## Item status

| Status | Meaning |
|--------|---------|
| `needs_review` | The resolver found no confident TMDB match. Candidates are attached. |
| `new` | Resolved. Sent to no service. |
| `monitored` | Radarr or Sonarr holds the title. |
| `requested` | Overseerr holds a request for the title. |
| `available` | The file is in the library. |
| `watched` | A playback event reported a watch. |

The collection holds `(snagged ∩ available) − watched`. It is live state, not history.

## Local indexes

Snagarr answers every state question from SQLite. No request to Snagarr waits on Plex, Radarr or Sonarr. A failed upstream call is logged and skipped; the stale index keeps answering and the next reconcile pass retries.

## Roles

One shared list with attribution. No accounts, no passwords.

| Action | `admin` | `member` |
|--------|---------|----------|
| Capture, search, read items | yes | yes |
| Resolve, archive, delete **own** items | yes | yes |
| Resolve, archive, delete **another user's** items | yes | no |
| Send to Radarr, Sonarr or Overseerr | yes | no |
| Read or write settings, users and tokens | yes | no |
| Force a reconcile | yes | no |

A `member` that calls an admin route gets `403 forbidden`.

## Not implemented

| Feature | State |
|---------|-------|
| Telegram bot | The settings card and `telegram.bot_token` exist. No code reads them. |
| Command line capture | The binary has `serve` and `version` only. |
| Generic webhook ingest | `/api/v1/webhooks/*` accepts import and playback events only. It never creates an item. |

## Next

- [Install](/snagarr/start/install/) — Docker, binary, systemd, reverse proxy.
- [First run](/snagarr/start/first-run/) — the admin token and the setup wizard.
- [HTTP API](/snagarr/reference/api/) — every endpoint.

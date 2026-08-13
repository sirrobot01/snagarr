---
title: Environment variables
description: Every SNAGARR_ variable — the start-up options, the settings it overrides and the first admin's services it seeds.
---

Snagarr reads three kinds of variable.

| Kind | Applied | Change needs |
|------|---------|--------------|
| Start-up option | Once, before the database opens | A restart |
| Setting override | At start-up and after every save | A restart |
| Service seed | At start-up | A restart |

No `SNAGARR_*` variable outside these tables exists.

## Start-up options

| Variable | Default | Effect | Flag |
|----------|---------|--------|------|
| `SNAGARR_ADDR` | `:8080` | HTTP listen address | `--addr` |
| `SNAGARR_DATA_DIR` | `data` | Directory for `snagarr.db` and `secret.key` | `--data` |
| `SNAGARR_LOG_LEVEL` | `info` | One of `debug`, `info`, `warn`, `error` | *(none)* |

An unknown log level falls back to `info`. An empty variable falls back to the default. The flags win over the variables:

```sh
snagarr serve --addr :9090 --data /var/lib/snagarr
```

The container image sets `SNAGARR_DATA_DIR=/data`.

## Setting overrides

| Variable | Overrides | Type |
|----------|-----------|------|
| `SNAGARR_TMDB_API_KEY` | `tmdb.api_key` | string |
| `SNAGARR_PUBLIC_URL` | `general.public_url` | string |
| `SNAGARR_SHORTCUT_URL` | `general.shortcut_url` | string |
| `SNAGARR_RECONCILE_INTERVAL` | `general.reconcile_interval` | Go duration |

These four are the whole list. Settings hold nothing else now: every other integration is a [service](/snagarr/configure/services/).

`general.webhook_secret` has no variable.

## Service seeding

The variables below do not override a setting. They write **the first admin's** services, so a Docker-first operator never has to open the UI.

| Variable | Kind | Config field | Type |
|----------|------|--------------|------|
| `SNAGARR_LIBRARY_PROVIDER` | — | Picks the kind: `plex`, `emby` or `jellyfin` | string |
| `SNAGARR_LIBRARY_URL` | media server | `url` | string |
| `SNAGARR_LIBRARY_TOKEN` | media server | `token` | string |
| `SNAGARR_LIBRARY_COLLECTION` | media server | `collection_name` | string |
| `SNAGARR_RADARR_URL` | `radarr` | `url` | string |
| `SNAGARR_RADARR_API_KEY` | `radarr` | `api_key` | string |
| `SNAGARR_RADARR_QUALITY_PROFILE_ID` | `radarr` | `quality_profile_id` | integer |
| `SNAGARR_RADARR_ROOT_FOLDER` | `radarr` | `root_folder` | string |
| `SNAGARR_SONARR_URL` | `sonarr` | `url` | string |
| `SNAGARR_SONARR_API_KEY` | `sonarr` | `api_key` | string |
| `SNAGARR_SONARR_QUALITY_PROFILE_ID` | `sonarr` | `quality_profile_id` | integer |
| `SNAGARR_SONARR_ROOT_FOLDER` | `sonarr` | `root_folder` | string |
| `SNAGARR_OVERSEERR_URL` | `overseerr` | `url` | string |
| `SNAGARR_OVERSEERR_API_KEY` | `overseerr` | `api_key` | string |
| `SNAGARR_NTFY_URL` | `ntfy` | `url` | string |
| `SNAGARR_NTFY_TOPIC` | `ntfy` | `topic` | string |
| `SNAGARR_NTFY_TOKEN` | `ntfy` | `token` | string |

### Rules

- Each seeded service is owned by the first admin and named `Default`.
- Seeding runs on every start. It rewrites the config of the service it owns.
- A seeded service comes back with `"locked": true`. The UI renders its fields read-only.
- Snagarr skips a kind the environment says nothing about. A service you made in the UI is left alone.
- `SNAGARR_LIBRARY_PROVIDER` must be `plex`, `emby` or `jellyfin`. Any other value seeds no media server.
- Seeding needs an admin to own the services. Until one exists, seeding does nothing.
- Another member's service is never touched, whatever the variables say.

`section_ids`, `search_on_add`, `season_folder` and `priority` have no variable. Set them in the UI or with `PATCH /api/v1/services/{id}`.

## Rules for every variable

- An empty variable is ignored. So is an integer or a duration that does not parse.
- A `PUT /api/v1/settings` still writes your value to the database. The environment value goes back on top until you remove the variable.
- A settings override locks the whole settings card in the UI, not the single field. A service seed locks that one service record.

## Example

```yaml
services:
  snagarr:
    image: ghcr.io/sirrobot01/snagarr:latest
    ports: ["8080:8080"]
    volumes:
      - snagarr:/data
    environment:
      SNAGARR_LOG_LEVEL: debug
      SNAGARR_PUBLIC_URL: https://snagarr.example.com
      SNAGARR_TMDB_API_KEY: your_tmdb_key
      SNAGARR_LIBRARY_PROVIDER: plex
      SNAGARR_LIBRARY_URL: http://plex.lan:32400
      SNAGARR_LIBRARY_TOKEN: your_plex_token
      SNAGARR_RECONCILE_INTERVAL: 30m
    restart: unless-stopped
volumes:
  snagarr:
```

That file gives the first admin a Plex service called `Default`. Every other member connects their own.

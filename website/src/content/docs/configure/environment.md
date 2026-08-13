---
title: Environment variables
description: Every SNAGARR_ variable, the start-up options it covers and the setting it overrides.
---

Snagarr reads two kinds of variable. Start-up options are read once, before the database opens; a change needs a restart. Every other variable overrides a stored setting at start-up and after every save.

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
| `SNAGARR_LIBRARY_PROVIDER` | `library.provider` | string |
| `SNAGARR_LIBRARY_URL` | `library.url` | string |
| `SNAGARR_LIBRARY_TOKEN` | `library.token` | string |
| `SNAGARR_LIBRARY_COLLECTION` | `library.collection_name` | string |
| `SNAGARR_RADARR_URL` | `radarr.url` | string |
| `SNAGARR_RADARR_API_KEY` | `radarr.api_key` | string |
| `SNAGARR_RADARR_QUALITY_PROFILE_ID` | `radarr.quality_profile_id` | integer |
| `SNAGARR_RADARR_ROOT_FOLDER` | `radarr.root_folder` | string |
| `SNAGARR_SONARR_URL` | `sonarr.url` | string |
| `SNAGARR_SONARR_API_KEY` | `sonarr.api_key` | string |
| `SNAGARR_SONARR_QUALITY_PROFILE_ID` | `sonarr.quality_profile_id` | integer |
| `SNAGARR_SONARR_ROOT_FOLDER` | `sonarr.root_folder` | string |
| `SNAGARR_OVERSEERR_URL` | `overseerr.url` | string |
| `SNAGARR_OVERSEERR_API_KEY` | `overseerr.api_key` | string |
| `SNAGARR_NTFY_URL` | `ntfy.url` | string |
| `SNAGARR_NTFY_TOPIC` | `ntfy.topic` | string |
| `SNAGARR_NTFY_TOKEN` | `ntfy.token` | string |
| `SNAGARR_TELEGRAM_BOT_TOKEN` | `telegram.bot_token` | string |
| `SNAGARR_PUBLIC_URL` | `general.public_url` | string |
| `SNAGARR_RECONCILE_INTERVAL` | `general.reconcile_interval` | Go duration |

## Rules

- An empty variable is ignored. So is a number or a duration that does not parse.
- A `PUT /api/v1/settings` still writes your value to the database. The environment value goes back on top until you remove the variable.
- An override locks the whole settings card in the UI, not the single field. `SNAGARR_RADARR_URL` alone makes the Radarr card read-only.
- These settings have no variable: `library.section_ids`, `radarr.search_on_add`, `sonarr.search_on_add`, `sonarr.season_folder`, `radarr.season_folder`, `ntfy.priority`, `general.webhook_secret`.

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

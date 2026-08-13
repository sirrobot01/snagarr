---
title: Settings
description: Where Snagarr stores settings, how it encrypts them, how environment overrides lock a card, and what the reconcile loop does.
---

Settings live in one JSON row of the `settings` table in `<data dir>/snagarr.db`, encrypted with `<data dir>/secret.key`. Read and write them at `GET`/`PUT /api/v1/settings` (admin only). `PUT` accepts a partial document; an omitted field keeps its stored value.

Start-up options — `SNAGARR_ADDR`, `SNAGARR_DATA_DIR`, `SNAGARR_LOG_LEVEL` — are not settings. They are read once, before the database opens. See [Environment variables](/snagarr/configure/environment/).

## Sections

| Section | Counts as configured when | Page |
|---------|---------------------------|------|
| `tmdb` | `api_key` is set | [below](#tmdb) |
| `library` | `provider`, `url` and `token` are all set | [Media servers](/snagarr/configure/media-servers/) |
| `radarr` | `url` and `api_key` are set | [Radarr and Sonarr](/snagarr/configure/radarr-sonarr/) |
| `sonarr` | `url` and `api_key` are set | [Radarr and Sonarr](/snagarr/configure/radarr-sonarr/) |
| `overseerr` | `url` and `api_key` are set | [Radarr and Sonarr](/snagarr/configure/radarr-sonarr/) |
| `ntfy` | `topic` is set | [Notifications](/snagarr/configure/notifications/) |
| `telegram` | `bot_token` is set | [Notifications](/snagarr/configure/notifications/) |
| `general` | always | [below](#general) |

## Secrets at rest

Snagarr encrypts every settings value with AES-256-GCM before it writes to the database. The key is 32 random bytes in `<data dir>/secret.key`, mode `0600`, generated on first start.

:::danger[Back up `secret.key` with the database]
Snagarr cannot read any stored setting without it. A restored database beside a new key loses every credential.
:::

Seven fields are masked in every API response:

`tmdb.api_key` · `library.token` · `radarr.api_key` · `sonarr.api_key` · `overseerr.api_key` · `ntfy.token` · `telegram.bot_token`

`GET /api/v1/settings` returns `••••` plus the last four characters. Send a masked value back unchanged to keep the stored secret.

`general.webhook_secret` is encrypted at rest like the rest but returns in clear text, because you must paste it into Radarr and Tautulli.

## TMDB

TMDB is required. Without it, Snagarr saves captures but never resolves them.

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `tmdb.api_key` | string | *(empty)* | `SNAGARR_TMDB_API_KEY` |

Use a TMDB API key (v3).

## General

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `general.reconcile_interval` | duration | `15m0s` | `SNAGARR_RECONCILE_INTERVAL` |
| `general.public_url` | string | *(empty)* | `SNAGARR_PUBLIC_URL` |
| `general.shortcut_url` | string | *(empty)* | `SNAGARR_SHORTCUT_URL` |
| `general.webhook_secret` | string | *(32 hex characters, generated)* | *(none)* |

`reconcile_interval` is a Go duration string (`15m`, `1h`, `90s`). Zero or less falls back to 15 minutes. The value round-trips in long form: send `15m`, read `15m0s`. The loop reads the interval before each wait, so a change applies after the current wait ends without a restart.

`public_url` is the URL an outside client uses, for example `https://snagarr.example.com`. It builds the ntfy click link, the setup URL and the bookmarklet.

`shortcut_url` is the iCloud link you publish from the Shortcuts app, for example `https://www.icloud.com/shortcuts/abc123`. The Snag screen shows an **Install the iOS Shortcut** button while it is set, and hides the button while it is empty. Build the shortcut first. See [Capture clients](/snagarr/use/clients/#apple-shortcut).

`webhook_secret` authenticates every inbound webhook. It has no environment override. See [Webhooks](/snagarr/use/webhooks/).

## Environment overrides

An environment variable wins over the stored value. Snagarr applies the overrides at start-up and again after every save. A save still writes your value to the database, and the environment value goes back on top; the stored value takes effect only when you remove the variable.

Snagarr ignores an empty variable, an unparsable number and an unparsable duration.

An override locks the whole settings card, not one field: the UI renders the card read-only, because an edit there is overwritten on the next restart. `SNAGARR_RADARR_URL` locks the URL, the API key, the quality profile and the root folder on the Radarr card.

`GET /api/v1/settings` reports `locked` per section.

## Fields the UI does not render

| Field | Set it with |
|-------|-------------|
| `library.collection_name` | API, or `SNAGARR_LIBRARY_COLLECTION` |
| `radarr.search_on_add` | API |
| `sonarr.search_on_add` | API |
| `sonarr.season_folder` | API |
| `ntfy.token` | API, or `SNAGARR_NTFY_TOKEN` |
| `ntfy.priority` | API |

The General card renders `public_url`, `reconcile_interval`, `shortcut_url` and `webhook_secret`.

```sh
curl -X PUT http://localhost:8080/api/v1/settings \
  -H "Authorization: Bearer sngr_your_admin_token" \
  -H "Content-Type: application/json" \
  -d '{"general":{"reconcile_interval":"30m"},"library":{"collection_name":"Watch Later"}}'
```

## The reconcile loop

Snagarr runs one pass at start-up, then one every `general.reconcile_interval`. Each pass does this work in order:

1. Replace the Radarr index and the Sonarr index. One list call per service.
2. Replace the Overseerr request index, if Overseerr is configured.
3. Sync the media server index.
4. Recompute the status of every snagged item from the local indexes.
5. Send one ntfy push for each item that became available.
6. Make the collection hold `(snagged ∩ available) − watched`.
7. Refresh cached TMDB metadata older than 7 days.
8. Delete expired rows from the HTTP response cache.

The media server sync is incremental on most passes and full once every 24 hours. Only a full sweep removes rows, because only a full sweep can see a deleted title.

A failed sync is logged and skipped. The pass continues, the stale index keeps answering and the next pass retries.

Force a pass with `POST /api/v1/admin/sync`, or with **Force reconcile now** in the settings UI.

Inbound webhooks accelerate the loop. They never replace it. See [Webhooks](/snagarr/use/webhooks/).

## Test a connection

The **Test** button on each card calls `POST /api/v1/settings/test`. The card shows the upstream message unchanged, for example `OK · 611 items` or `401 — check token`. There is no test for Telegram.

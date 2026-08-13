---
title: Settings
description: The two install-wide settings sections, how Snagarr encrypts them, how environment overrides lock a card, and what the reconcile loop does.
---

Settings are what stays global once every integration belongs to a member: the TMDB key and the install-wide knobs. Everything else is a [service](/snagarr/configure/services/).

Settings live in one JSON row of the `settings` table in `<data dir>/snagarr.db`, encrypted with `<data dir>/secret.key`. Read and write them at `GET`/`PUT /api/v1/settings` (admin only). `PUT` accepts a partial document; an omitted field keeps its stored value.

Start-up options — `SNAGARR_ADDR`, `SNAGARR_DATA_DIR`, `SNAGARR_LOG_LEVEL` — are not settings. They are read once, before the database opens. See [Environment variables](/snagarr/configure/environment/).

## Sections

| Section | Counts as configured when |
|---------|---------------------------|
| `tmdb` | `api_key` is set |
| `general` | always |

Radarr, Sonarr, Overseerr, Plex, Emby, Jellyfin and ntfy are gone from this endpoint. Each member connects their own at `/api/v1/services`.

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
| `general.shortcut_url` | string | Snagarr's published Shortcut | `SNAGARR_SHORTCUT_URL` |
| `general.webhook_secret` | string | *(32 hex characters, generated)* | *(none)* |

`reconcile_interval` is a Go duration string (`15m`, `1h`, `90s`). Zero or less falls back to 15 minutes. The value round-trips in long form: send `15m`, read `15m0s`. The loop reads the interval before each wait, so a change applies after the current wait ends without a restart.

`public_url` is the URL an outside client uses, for example `https://snagarr.example.com`. It builds the ntfy click link and the bookmarklet.

`shortcut_url` defaults to `https://www.icloud.com/shortcuts/c4b4dabe0b55481c9fe35fac0a4a266b`, the Shortcut Snagarr publishes. The Snag screen shows an **Install the iOS Shortcut** button while the value is set, and hides the button while it is empty. Replace it with your own iCloud link to publish a different Shortcut. See [Capture clients](/snagarr/use/clients/#apple-shortcut).

`webhook_secret` authenticates every inbound webhook. It has no environment override. See [Webhooks](/snagarr/use/webhooks/).

## Secrets at rest

Snagarr encrypts the settings blob and every service config with AES-256-GCM before it writes to the database. The key is 32 random bytes in `<data dir>/secret.key`, mode `0600`, generated on first start.

:::danger[Back up `secret.key` with the database]
Snagarr cannot read any stored setting or service without it. A restored database beside a new key loses every credential.
:::

| Value | Masked in the API |
|-------|-------------------|
| `tmdb.api_key` | Yes |
| A service config `api_key` or `token` | Yes |
| `general.webhook_secret` | No |

A masked value comes back as `••••` plus the last four characters. Send it back unchanged to keep the stored secret.

`general.webhook_secret` is encrypted at rest like the rest but returns in clear text, because you must paste it into Radarr and Tautulli.

## Environment overrides

An environment variable wins over the stored value. Snagarr applies the overrides at start-up and again after every save. A save still writes your value to the database, and the environment value goes back on top; the stored value takes effect only when you remove the variable.

Snagarr ignores an empty variable and an unparsable duration.

An override locks the whole settings card, not one field: the UI renders the card read-only, because an edit there is overwritten on the next restart. `SNAGARR_PUBLIC_URL` locks the public URL, the reconcile interval and the Shortcut link on the General card.

`GET /api/v1/settings` reports `locked` per section. Services carry their own `locked` flag per record.

```sh
curl -X PUT http://localhost:8080/api/v1/settings \
  -H "Authorization: Bearer sngr_your_admin_token" \
  -H "Content-Type: application/json" \
  -d '{"general":{"reconcile_interval":"30m"}}'
```

## The reconcile loop

Snagarr runs one pass at start-up, then one every `general.reconcile_interval`. Each pass walks every enabled service in the household and does this work in order:

1. Replace the Radarr index and the Sonarr index. One list call per service.
2. Replace the Overseerr request index. One list call per service.
3. Sync each media server index.
4. Recompute the status of every snagged item from the local indexes.
5. Send one ntfy push for each item that became available.
6. Make each media server's collection hold `(snagged ∩ available) − watched`.
7. Refresh cached TMDB metadata older than 7 days.
8. Delete expired rows from the HTTP response cache.

The media server sync is incremental on most passes and full once every 24 hours, per service. Only a full sweep removes rows, because only a full sweep can see a deleted title.

A failed sync is logged and skipped. The pass continues, the stale index keeps answering and the next pass retries.

Force a pass with `POST /api/v1/admin/sync`, or with **Force reconcile now** in the settings UI. Creating, changing or deleting a service also starts one.

Inbound webhooks accelerate the loop. They never replace it. See [Webhooks](/snagarr/use/webhooks/).

## Test a connection

`POST /api/v1/settings/test` accepts one service name:

```json
{ "service": "tmdb" }
```

Any other name is `400 bad_request`. Test every other integration with `POST /api/v1/services/{id}/test`.
